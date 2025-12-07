// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/acls"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/sessions"
	"codeberg.org/readeck/readeck/pkg/ctxr"
	"codeberg.org/readeck/readeck/pkg/http/request"
)

type ctxTokenKey struct{}

var withToken, getToken = ctxr.WithGetter[string](ctxTokenKey{})

// Interface guards.
var (
	_ auth.Provider                  = (*TokenAuthProvider)(nil)
	_ auth.LoggerProvider            = (*TokenAuthProvider)(nil)
	_ auth.FeatureCsrfProvider       = (*TokenAuthProvider)(nil)
	_ auth.FeaturePermissionProvider = (*TokenAuthProvider)(nil)

	_ auth.Provider       = (*SessionAuthProvider)(nil)
	_ auth.LoggerProvider = (*SessionAuthProvider)(nil)
)

// TokenAuthProvider handles authentication using a bearer token
// passed in the request "Authorization" header with the scheme
// "Bearer".
type TokenAuthProvider struct{}

// Log implements [auth.LoggerProvider].
func (p *TokenAuthProvider) Log(r *http.Request) *slog.Logger {
	return slog.With(slog.String("@id", request.GetReqID(r.Context())))
}

// Handler sets [tokenAuthProvider] when the client submit an Authorization header
// that contains a token.
func (p *TokenAuthProvider) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if token, ok := p.getToken(r); ok {
			ctx = auth.WithProvider(ctx, p)
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

	ctx := auth.WithAuthInfo(r.Context(), &auth.Info{
		Provider: &auth.ProviderInfo{
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
	if len(auth.GetRequestAuthInfo(r).Provider.Roles) == 0 {
		return true
	}

	for _, scope := range auth.GetRequestAuthInfo(r).Provider.Roles {
		if acls.Enforce(scope, obj, act) {
			return true
		}
	}

	return false
}

// GetPermissions returns all the permissions attached to the current authentication provider
// role list. If no role is defined, it will fallback to the user permission list.
func (p *TokenAuthProvider) GetPermissions(r *http.Request) []string {
	if len(auth.GetRequestAuthInfo(r).Provider.Roles) == 0 {
		return nil
	}

	return acls.GetPermissions(auth.GetRequestAuthInfo(r).Provider.Roles...)
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

// SessionAuthProvider is the last authentication provider.
// It's alway enabled in case of every previous provider failing.
type SessionAuthProvider struct{}

// Log implements [auth.LoggerProvider].
func (p *SessionAuthProvider) Log(r *http.Request) *slog.Logger {
	return slog.With(slog.String("@id", request.GetReqID(r.Context())))
}

// Handler always set this provider to the request and it must come as the last one.
// If authentication fails, it will redirect to the login page.
func (p *SessionAuthProvider) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.WithProvider(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Authenticate checks if the request's session cookie is valid and
// the user exists.
func (p *SessionAuthProvider) Authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	sess := GetSession(r)
	u, err := p.checkSession(sess)
	if u == nil || err != nil {
		p.clearSession(sess, w, r)
		return r, err
	}

	// At this point, the user is granted access.
	// We renew its session for another max age duration.
	sess.Save(w, r)
	ctx := auth.WithAuthInfo(r.Context(), &auth.Info{
		Provider: &auth.ProviderInfo{
			Name: "http session",
		},
		User: u,
	})
	return r.WithContext(ctx), nil
}

func (p *SessionAuthProvider) checkSession(sess *sessions.Session) (u *users.User, err error) {
	if sess.IsNew {
		return
	}

	if sess.Payload.User == 0 {
		return
	}

	if u, err = users.Users.GetOne(goqu.C("id").Eq(sess.Payload.User)); err != nil {
		return nil, err
	}

	if u.Seed != sess.Payload.Seed {
		return nil, nil
	}

	return
}

func (p *SessionAuthProvider) clearSession(sess *sessions.Session, w http.ResponseWriter, r *http.Request) {
	sess.Clear(w, r)
	unauthorizedHandler(w, r)
}
