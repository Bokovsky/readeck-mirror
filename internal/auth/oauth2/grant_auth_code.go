// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oauth2

// This implements the Authorization Code Grant
// https://datatracker.ietf.org/doc/html/rfc6749#section-1.3.1
// https://datatracker.ietf.org/doc/html/rfc7636

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/chacha20poly1305"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/forms"
)

const (
	nonceSize = chacha20poly1305.NonceSizeX
	overhead  = chacha20poly1305.Overhead
)

var (
	errInvalidChallenge = errors.New("challenge is not valid")
	errRequestExpired   = errors.New("request has expired")
)

// authCodeRequest is the authorization request.
// Once encrypted using XChaCha20-Poly1305, it becomes a
// stateless authorization code that can be carried around
// until its expiration.
type authCodeRequest struct {
	ClientID  string    `json:"a"`
	TokenID   string    `json:"t"`
	Scopes    []string  `json:"s"`
	Challenge string    `json:"ch"`
	UserID    int       `json:"u"`
	Expires   time.Time `json:"e"`
}

// encode turns an [authCodeRequest] into JSON and encodes it.
func (r authCodeRequest) encode() ([]byte, error) {
	var data []byte
	data, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, nonceSize, len(data)+overhead+nonceSize)
	rand.Read(nonce)

	aead, err := chacha20poly1305.NewX(configs.Keys.OauthRequestKey())
	if err != nil {
		return nil, err
	}

	return aead.Seal(nonce, nonce, data, nil), nil
}

// loadAuthCodeRequest returns a new [authCodeRequest] from
// encoded bytes.
func loadAuthCodeRequest(data []byte) (*authCodeRequest, error) {
	if len(data) < nonceSize+overhead {
		return nil, errors.New("payload is too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	aead, err := chacha20poly1305.NewX(configs.Keys.OauthRequestKey())
	if err != nil {
		return nil, err
	}

	content, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	req := new(authCodeRequest)
	if err := json.Unmarshal(content, req); err != nil {
		return nil, err
	}

	return req, nil
}

type authorizeViewRouter struct {
	chi.Router
}

func newAuthorizeViewRouter() *authorizeViewRouter {
	router := &authorizeViewRouter{chi.NewRouter()}

	router.With(
		server.Csrf,
		server.WithSession(),
		server.WithRedirectLogin,
		auth.Required,
		router.withClient,
	).Route("/", func(r chi.Router) {
		r.Get("/", router.authorizeHandler)
		r.Post("/", router.authorizeHandler)
	})

	return router
}

func (h *authorizeViewRouter) withClient(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			server.Err(w, r, errInvalidClient)
			return
		}

		client, err := Clients.GetOne(goqu.C("uid").Eq(clientID))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				server.Err(w, r, errInvalidClient)
			} else {
				server.Err(w, r, errServerError.withError(err))
			}
			return
		}

		ctx := withClient(r.Context(), client)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authorizeHandler is the authorization page returned to a user.
// It shows a form with an accept or deny action. Once the form is
// submitted, it returns a 302 response with the redirection
// containing the code and state, or the error if the user denied
// the request.
func (h *authorizeViewRouter) authorizeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "nostore")
	w.Header().Set("Pragma", "no-cache")

	f := newAuthorizationForm(server.Locale(r), auth.GetRequestUser(r))

	switch r.Method {
	case http.MethodGet:
		forms.BindURL(f, r)
	case http.MethodPost:
		forms.Bind(f, r)
	}

	client := getClient(r.Context())

	// Validate redirect URI first
	redir, _ := url.Parse(f.Get("redirect_uri").String())
	if !slices.Contains(client.RedirectURIs, redir.String()) {
		// This error can't obviouvsly be sent through a redirection.
		server.TextMsg(w, r, http.StatusBadRequest, "invalid redirect URI")
		return
	}

	if !f.IsValid() {
		// Errors other than an invalid redirect URI are added to the redirect URL
		// https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1
		f.getError().redirect(w, r, redir, url.Values{"state": []string{f.Get("state").String()}})
		return
	}

	tc := server.TC{
		"Client": client,
		"Redir":  fmt.Sprintf("%s://%s", redir.Scheme, redir.Hostname()),
	}

	availableScopes := f.Get("scope").(interface {
		Choices() forms.ValueChoices[string]
	}).Choices()
	providedScopes := f.Get("scope").Value().([]string)
	scopes := []string{}
	for _, x := range availableScopes {
		if x.In(providedScopes) {
			scopes = append(scopes, x.Name)
		}
	}
	tc["Scopes"] = scopes

	// Remove form-action CSP directive
	policy := server.GetCSPHeader(r)
	policy.Del("form-action")
	policy.Write(w.Header())

	if r.Method == http.MethodPost {
		params := redir.Query()
		params.Set("code", "")
		if f.Get("state").String() != "" {
			params.Set("state", f.Get("state").String())
		}

		if !f.Get("granted").(*forms.BooleanField).V() {
			errAccessDenied.withDescription("access denied").redirect(w, r, redir, params)
			return
		}

		// Add encrypted code to the redirect_uri parameter.
		code, err := f.getCode(auth.GetRequestUser(r))
		if err != nil {
			errServerError.withError(err).redirect(w, r, redir, params)
			return
		}

		params.Set("code", code)
		if state := f.Get("state").String(); state != "" {
			params.Set("state", state)
		}
		redir.RawQuery = params.Encode()
		w.Header().Set("Location", redir.String())
		w.WriteHeader(http.StatusFound)
		return
	}

	server.RenderTemplate(w, r, http.StatusOK, "auth/oauth/auth-code", tc)
}

// authorizationCodeHandler is the api route that receives the authorization code
// and returns an access token when all checks are successful.
func (api *oauthAPI) authorizationCodeHandler(w http.ResponseWriter, r *http.Request) {
	f := getTokenForm(r.Context())
	req, err := f.loadRequest()
	if err != nil {
		server.Err(w, r, errInvalidGrant.withDescription("code is not valid").withError(err))
		return
	}

	user, err := users.Users.GetOne(goqu.C("id").Eq(req.UserID))
	if err != nil {
		server.Err(w, r, errInvalidGrant.withDescription("user not found").withError(err))
		return
	}

	client, err := Clients.GetOne(goqu.C("uid").Eq(req.ClientID))
	if err != nil {
		server.Err(w, r, errInvalidClient.withDescription("client not found").withError(err))
		return
	}

	t := &tokens.Token{
		UID:         req.TokenID,
		UserID:      &user.ID,
		ClientID:    &client.ID,
		IsEnabled:   true,
		Application: client.Name,
		Roles:       req.Scopes,
	}
	if err = tokens.Tokens.Create(t); err != nil {
		server.Err(w, r,
			errServerError.withDescription("can't create token").withError(err),
		)
		return
	}

	token, err := configs.Keys.TokenKey().Encode(t.UID)
	if err != nil {
		server.Err(w, r,
			errServerError.withDescription("can't encode token").withError(err),
		)
		return
	}

	server.Render(w, r, http.StatusCreated, tokenResponse{
		UID:         t.UID,
		AccessToken: token,
		TokenType:   "Bearer",
		Scope:       strings.Join(t.Roles, " "),
	})
}
