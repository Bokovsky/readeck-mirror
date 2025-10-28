// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oauth2

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type (
	ctxClientFormKey struct{}
)

const (
	grantTypeAuthCode   = "authorization_code"
	grantTypeDeviceCode = "urn:ietf:params:oauth:grant-type:device_code"
)

type clientForm struct {
	*forms.Form
}

func newClientForm(tr forms.Translator) *clientForm {
	return &clientForm{
		forms.Must(
			forms.WithTranslator(context.Background(), tr),
			forms.NewTextField("client_id", forms.Trim, forms.ValueValidatorFunc[string](func(f forms.Field, v string) error {
				c, _ := forms.GetForm(f).Context().Value(ctxClientFormKey{}).(*Client)
				if c == nil {
					return nil
				}

				// Per RFC 7592:
				// The client MUST include its "client_id" field in the request, and it
				// MUST be the same as its currently issued client identifier.
				if c.UID != v {
					return errors.New("client ID doesn't match")
				}
				return nil
			})),
			forms.NewTextField("client_name", forms.Trim, forms.Required, forms.StrLen(0, 128)),
			forms.NewTextField("client_uri", forms.Trim, forms.Required, forms.StrLen(0, 256), forms.IsURL("https")),
			forms.NewTextField("logo_uri", forms.Trim, forms.StrLen(0, 8<<10), isValidLogoURI),
			forms.NewTextField("software_id", forms.Trim, forms.Required, forms.StrLen(0, 128)),
			forms.NewTextField("software_version", forms.Trim, forms.Required, forms.StrLen(0, 64)),
			forms.NewTextListField("redirect_uris", forms.Trim, forms.Required, isValidRedirectURI),

			// Ignored fields but we want to coerce their values
			forms.NewTextField("token_endpoint_auth_method", forms.ChoicesPairs([][2]string{{"none", "none"}})),
			forms.NewTextListField("grant_types", forms.ChoicesPairs([][2]string{
				{grantTypeAuthCode, grantTypeAuthCode},
				{grantTypeDeviceCode, grantTypeDeviceCode},
			})),
			forms.NewTextListField("response_types", forms.ChoicesPairs([][2]string{
				{"code", "code"},
			})),
		),
	}
}

func (f *clientForm) setClient(c *Client) {
	ctx := context.WithValue(f.Context(), ctxClientFormKey{}, c)
	f.SetContext(ctx)
}

func (f *clientForm) getError() oauthError {
	switch {
	case len(f.Get("redirect_uris").Errors()) > 0:
		return errInvalidRedirectURI.withDescription(f.Get("redirect_uris").Errors().Error())
	default:
		return errInvalidClientMetadata.withDescription(newFormError(f).description)
	}
}

func (f *clientForm) createClient() (*Client, error) {
	client := &Client{
		Name:            f.Get("client_name").String(),
		Website:         f.Get("client_uri").String(),
		Logo:            f.Get("logo_uri").String(),
		RedirectURIs:    f.Get("redirect_uris").Value().([]string),
		SoftwareID:      f.Get("software_id").String(),
		SoftwareVersion: f.Get("software_version").String(),
	}

	if err := Clients.Create(client); err != nil {
		return nil, err
	}

	return client, nil
}

func (f *clientForm) updateClient(client *Client) (res map[string]any, err error) {
	if !f.IsBound() {
		err = errors.New("form is not bound")
		return
	}

	res = make(map[string]any)
	for _, field := range f.Fields() {
		if !field.IsBound() || field.IsNil() {
			continue
		}
		switch field.Name() {
		case "client_name":
			res["name"] = field.String()
		case "client_uri":
			res["website"] = field.String()
		case "logo_uri":
			res["logo"] = field.String()
		case "redirect_uris":
			res["redirect_uris"] = types.Strings(field.Value().([]string))
		case "software_version":
			res["software_version"] = field.String()
		}
	}

	if len(res) == 0 {
		return
	}

	if err = client.Update(res); err != nil {
		f.AddErrors("", forms.ErrUnexpected)
		return
	}

	return
}

type authorizationForm struct {
	*forms.Form
}

func newAuthorizationForm(tr forms.Translator, user *users.User) *authorizationForm {
	return &authorizationForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("client_id", forms.Trim, forms.Required),
		forms.NewTextField("redirect_uri", forms.Trim, forms.Required, isValidRedirectURI),
		newScopeField("scope",
			forms.Trim,
			forms.Required,
			forms.ChoicesPairs(users.GroupList(tr, "__oauth_scope__", user)),
		),
		forms.NewTextField("state", forms.Trim),
		forms.NewTextField("code_challenge", forms.Required),
		forms.NewTextField("code_challenge_method",
			forms.Trim,
			forms.Required,
			forms.ChoicesPairs([][2]string{
				{"S256", "S256"},
			}),
		),
		forms.NewBooleanField("granted"),
	)}
}

func (f *authorizationForm) getCode(user *users.User) (string, error) {
	req := authCodeRequest{
		ClientID:  f.Get("client_id").String(),
		TokenID:   base58.NewUUID(),
		Scopes:    f.Get("scope").Value().([]string),
		Challenge: f.Get("code_challenge").String(),
		UserID:    user.ID,
		Expires:   time.Now().UTC().Add(time.Minute * 10),
	}

	slices.Sort(req.Scopes)
	req.Scopes = slices.Compact(req.Scopes)

	code, err := req.encode()
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(code), nil
}

func (f *authorizationForm) getError() oauthError {
	switch {
	case len(f.Get("scope").Errors()) > 0:
		return errInvalidScope.withDescription(f.Get("scope").Errors().Error())
	default:
		return newFormError(f)
	}
}

type deviceForm struct {
	*forms.Form
}

func newDeviceForm(tr forms.Translator) *deviceForm {
	return &deviceForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("client_id", forms.Trim, forms.Required),
		newScopeField("scope",
			forms.Trim,
			forms.Required,
			forms.ChoicesPairs(users.GroupList(tr, "__oauth_scope__", nil)),
		),
	)}
}

func (f *deviceForm) getError() oauthError {
	switch {
	case len(f.Get("scope").Errors()) > 0:
		return errInvalidScope.withDescription(f.Get("scope").Errors().Error())
	default:
		return newFormError(f)
	}
}

type tokenForm struct {
	*forms.Form
}

func newTokenForm(tr forms.Translator) *tokenForm {
	requiredByGrantType := func(t string) forms.FieldValidator {
		return forms.FieldValidatorFunc(func(f forms.Field) error {
			if forms.GetForm(f).Get("grant_type").String() == t {
				return forms.Required(f)
			}
			return nil
		})
	}

	return &tokenForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("grant_type",
			forms.Trim,
			forms.Required,
			forms.ChoicesPairs([][2]string{
				{grantTypeAuthCode, grantTypeAuthCode},
				{grantTypeDeviceCode, grantTypeDeviceCode},
			}),
		),
		forms.NewTextField("code",
			forms.Trim,
			requiredByGrantType(grantTypeAuthCode),
		),
		forms.NewTextField("code_verifier",
			forms.Trim,
			requiredByGrantType(grantTypeAuthCode),
		),

		forms.NewTextField("device_code",
			forms.Trim,
			requiredByGrantType(grantTypeDeviceCode),
		),
		forms.NewTextField("client_id",
			forms.Trim,
			requiredByGrantType(grantTypeDeviceCode),
		),
	)}
}

func (f *tokenForm) loadRequest() (*authCodeRequest, error) {
	data, err := base64.RawURLEncoding.DecodeString(f.Get("code").String())
	if err != nil {
		return nil, err
	}

	req, err := loadAuthCodeRequest(data)
	if err != nil {
		return nil, err
	}

	if !f.verifyChallenge(req.Challenge) {
		return nil, errInvalidChallenge
	}

	if time.Now().UTC().After(req.Expires) {
		return nil, errRequestExpired
	}

	return req, nil
}

func (f *tokenForm) verifyChallenge(challenge string) bool {
	c, err := base64.RawURLEncoding.DecodeString(challenge)
	if err != nil {
		return false
	}

	h := sha256.New()
	h.Write([]byte(f.Get("code_verifier").String()))

	return subtle.ConstantTimeCompare(c, h.Sum(nil)) == 1
}

type revokeTokenForm struct {
	*forms.Form
}

func newRevokeTokenForm(tr forms.Translator) *revokeTokenForm {
	return &revokeTokenForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("token", forms.Trim, forms.Required),
	)}
}

func (f *revokeTokenForm) revoke(client *Client) error {
	tokenID, err := configs.Keys.TokenKey().Decode(f.Get("token").String())
	if err != nil {
		return err
	}

	token, err := tokens.Tokens.GetOne(goqu.C("uid").Eq(tokenID))
	if err != nil && !errors.Is(err, tokens.ErrNotFound) {
		return err
	}
	if token == nil {
		return nil
	}

	// A client can only remove its own tokens
	if *token.ClientID != client.ID {
		return errInvalidRequest
	}

	return token.Delete()
}

var isValidLogoURI = forms.TypedValidator(func(v string) bool {
	if v == "" {
		return true
	}
	u, err := url.Parse(v)
	if err != nil {
		return false
	}

	if u.Scheme != "data" || !strings.HasPrefix(u.Opaque, "image/png;base64,") {
		return false
	}

	text, _ := strings.CutPrefix(u.Opaque, "image/png;base64,")
	_, err = png.DecodeConfig(base64.NewDecoder(base64.StdEncoding, strings.NewReader(text)))
	return err == nil
}, errors.New("invalid logo URI"))

var isValidRedirectURI = forms.TypedValidator(func(v string) bool {
	u, err := url.Parse(v)
	if err != nil {
		return false
	}

	switch u.Scheme {
	case "":
		return false
	case "https":
		// https needs a hostname
		return u.Hostname() != ""
	case "http":
		// only allow http with a loopback IP address
		host := u.Hostname()
		if ip, err := netip.ParseAddr(host); err == nil {
			if ip.IsLoopback() {
				return true
			}
		}
		return false
	default:
		// Allow URIs like net.myapp:auth-callback
		return true
	}
}, forms.ErrInvalidURL)

type scopeField struct {
	*forms.TextListField
}

func newScopeField(name string, options ...any) *scopeField {
	return &scopeField{
		forms.NewTextListField(name, options...),
	}
}

func (f *scopeField) UnmarshalValues(values []string) error {
	newValues := []string{}
	for _, v := range values {
		newValues = append(newValues, strings.Fields(v)...)
	}

	return f.TextListField.UnmarshalValues(newValues)
}

func (f *scopeField) UnmarshalJSON(data []byte) error {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		f.Set(nil)
		return err
	}

	s, ok := decoded.(string)
	if !ok {
		f.Set(nil)
		return errors.New("invalid value")
	}

	return f.UnmarshalValues([]string{s})
}
