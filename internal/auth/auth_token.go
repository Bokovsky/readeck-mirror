// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/acls"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/pkg/ctxr"
)

type ctxTokenKey struct{}

var withToken, getToken = ctxr.WithGetter[string](ctxTokenKey{})

// TokenAuthProvider handles authentication using a bearer token
// passed in the request "Authorization" header with the scheme
// "Bearer".
type TokenAuthProvider struct{}

// Handler sets [TokenAuthProvider] when the client submit an Authorization header
// that contains a token.
func (p *TokenAuthProvider) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if token, ok := p.getToken(r); ok {
			ctx = withProvider(ctx, p)
			ctx = withToken(ctx, token)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Authenticate performs the authentication using the "Authorization: Bearer"
// header provided.
func (p *TokenAuthProvider) Authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	token := getToken(r.Context())
	if token == "" {
		p.denyAccess(w)
		return r, errors.New("invalid authentication header")
	}

	uid, err := configs.Keys.TokenKey().Decode(token)
	if err != nil {
		p.denyAccess(w)
		return r, err
	}

	res, err := tokens.Tokens.GetUser(uid)
	if err != nil {
		p.denyAccess(w)
		return r, err
	}

	if err := res.Token.Update(goqu.Record{
		"last_used": time.Now().UTC(),
	}); err != nil {
		return r, err
	}

	if res.Token.IsExpired() {
		p.denyAccess(w)
		return r, errors.New("expired token")
	}

	ctx := withAuthInfo(r.Context(), &Info{
		Provider: &ProviderInfo{
			Name:        "bearer token",
			Application: res.Token.Application,
			Roles:       res.Token.Roles,
			ID:          res.Token.UID,
		},
		User: res.User,
	})
	return r.WithContext(ctx), nil
}

// HasPermission checks the permission on the current authentication provider role
// list. If the role list is empty, the user permissions apply.
func (p *TokenAuthProvider) HasPermission(r *http.Request, obj, act string) bool {
	if len(GetRequestAuthInfo(r).Provider.Roles) == 0 {
		return true
	}

	for _, scope := range GetRequestAuthInfo(r).Provider.Roles {
		if acls.Enforce(scope, obj, act) {
			return true
		}
	}

	return false
}

// GetPermissions returns all the permissions attached to the current authentication provider
// role list. If no role is defined, it will fallback to the user permission list.
func (p *TokenAuthProvider) GetPermissions(r *http.Request) []string {
	if len(GetRequestAuthInfo(r).Provider.Roles) == 0 {
		return nil
	}

	return acls.GetPermissions(GetRequestAuthInfo(r).Provider.Roles...)
}

// CsrfExempt is always true for this provider.
func (p *TokenAuthProvider) CsrfExempt(_ *http.Request) bool {
	return true
}

// getToken reads the token from the "Authorization" header.
func (p *TokenAuthProvider) getToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", false
	}

	// Try basic auth first
	if _, token, ok := r.BasicAuth(); ok {
		return token, ok
	}

	// Bearer token otherwise
	const prefix = "Bearer "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return "", false
	}
	return auth[len(prefix):], true
}

func (p *TokenAuthProvider) denyAccess(w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", `Bearer realm="Bearer token"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
}
