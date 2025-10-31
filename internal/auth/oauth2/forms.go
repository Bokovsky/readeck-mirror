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
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"golang.org/x/net/idna"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/forms"
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
			forms.NewTextField("client_name", forms.Trim, forms.Required, forms.StrLen(0, 128)),
			forms.NewTextField("client_uri", forms.Trim, forms.Required, forms.StrLen(0, 256), isValidClientURI),
			forms.NewTextField("logo_uri", forms.Trim, forms.StrLen(0, 8<<10), isValidLogoURI),
			forms.NewTextField("software_id", forms.Trim, forms.Required, forms.StrLen(0, 128)),
			forms.NewTextField("software_version", forms.Trim, forms.Required, forms.StrLen(0, 64)),
			forms.NewTextListField("redirect_uris",
				forms.Trim,
				forms.FieldValidatorFunc(func(f forms.Field) error {
					if !slices.Contains(
						forms.GetForm(f).Get("grant_types").(*forms.TextListField).V(),
						grantTypeAuthCode,
					) {
						return nil
					}

					if len(f.(*forms.ListField[string]).V()) == 0 {
						return forms.ErrRequired
					}
					return nil
				}),
				isValidRedirectURI,
			),
			forms.NewTextListField("grant_types",
				forms.ChoicesPairs([][2]string{
					{grantTypeAuthCode, grantTypeAuthCode},
					{grantTypeDeviceCode, grantTypeDeviceCode},
				}),
				forms.Default([]string{grantTypeAuthCode, grantTypeDeviceCode}),
			),

			// Ignored fields but we want to coerce their values
			forms.NewTextField("token_endpoint_auth_method",
				forms.ChoicesPairs([][2]string{{"none", "none"}}),
				forms.Default("none"),
			),
			forms.NewTextListField("response_types",
				forms.ChoicesPairs([][2]string{
					{"code", "code"},
				}),
				forms.Default([]string{"code"}),
			),
		),
	}
}

func (f *clientForm) getError() oauthError {
	switch {
	case len(f.Get("redirect_uris").Errors()) > 0:
		return errInvalidRedirectURI.withDescription(f.Get("redirect_uris").Errors().Error())
	default:
		return errInvalidClientMetadata.withDescription(newFormError(f).description)
	}
}

func (f *clientForm) createClient() (*oauthClient, error) {
	client := &oauthClient{
		ID:                      uuid.New().URN(),
		Name:                    f.Get("client_name").String(),
		URI:                     f.Get("client_uri").String(),
		Logo:                    f.Get("logo_uri").String(),
		RedirectURIs:            f.Get("redirect_uris").(*forms.TextListField).V(),
		GrantTypes:              f.Get("grant_types").(*forms.TextListField).V(),
		TokenEndpointAuthMethod: f.Get("token_endpoint_auth_method").String(),
		ResponseTypes:           f.Get("response_types").(*forms.TextListField).V(),
		SoftwareID:              f.Get("software_id").String(),
		SoftwareVersion:         f.Get("software_version").String(),
	}

	if err := client.store(); err != nil {
		return nil, err
	}

	return client, nil
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

func (f *revokeTokenForm) revoke(r *http.Request) error {
	tokenID, err := configs.Keys.TokenKey().Decode(f.Get("token").String())
	if err != nil {
		return err
	}

	// must be authenticated with the same token
	if tokenID != auth.GetRequestAuthInfo(r).Provider.ID {
		return errAccessDenied
	}

	token, err := tokens.Tokens.GetOne(goqu.C("uid").Eq(tokenID))
	if err != nil {
		if errors.Is(err, tokens.ErrNotFound) {
			return nil
		}
		return err
	}

	return token.Delete()
}

// isValidClientURI checks the given client URL.
// It must be https only and resolve to an ip that is not
// private or a loopback address.
var isValidClientURI = forms.TypedValidator(func(v string) bool {
	u, err := url.Parse(v)
	if err != nil {
		return false
	}

	if u.Scheme != "https" {
		return false
	}
	if u.Hostname() == "" {
		return false
	}
	host, err := idna.ToASCII(u.Hostname())
	if err != nil {
		return false
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return false
	}

	// Private and loopback is not allowed
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() {
			return false
		}
	}

	return true
}, errors.New("invalid client URI"))

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
