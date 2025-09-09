// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/acls"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
)

// TokenAuthProvider handles authentication using a bearer token
// passed in the request "Authorization" header with the scheme
// "Bearer".
type TokenAuthProvider struct{}

// IsActive returns true when the client submits a bearer token.
func (p *TokenAuthProvider) IsActive(r *http.Request) bool {
	_, ok := p.getToken(r)
	return ok
}

// Authenticate performs the authentication using the "Authorization: Bearer"
// header provided.
func (p *TokenAuthProvider) Authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	token, ok := p.getToken(r)
	if !ok {
		p.denyAccess(w)
		return r, errors.New("invalid authentication header")
	}

	uid, err := tokens.DecodeToken(token)
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

	return SetRequestAuthInfo(r, &Info{
		Provider: &ProviderInfo{
			Name:        "bearer token",
			Application: res.Token.Application,
			Roles:       res.Token.Roles,
			ID:          res.Token.UID,
		},
		User: res.User,
	}), nil
}

// HasPermission checks the permission on the current authentication provider role
// list. If the role list is empty, the user permissions apply.
func (p *TokenAuthProvider) HasPermission(r *http.Request, obj, act string) bool {
	if len(GetRequestAuthInfo(r).Provider.Roles) == 0 {
		return true
	}

	for _, scope := range GetRequestAuthInfo(r).Provider.Roles {
		if ok, err := acls.Check(scope, obj, act); err != nil {
			slog.Error("ACL check error", slog.Any("err", err))
		} else if ok {
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

	plist, _ := acls.GetPermissions(GetRequestAuthInfo(r).Provider.Roles...)
	return plist
}

// CsrfExempt is always true for this provider.
func (p *TokenAuthProvider) CsrfExempt(_ *http.Request) bool {
	return true
}

// getToken reads the token from the "Authorization" header.
func (p *TokenAuthProvider) getToken(r *http.Request) (token string, ok bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return
	}

	// Try basic auth first
	if _, token, ok = r.BasicAuth(); ok {
		return
	}

	// Bearer token otherwise
	token = ""
	const prefix = "Bearer "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	token = auth[len(prefix):]
	ok = true
	return
}

func (p *TokenAuthProvider) denyAccess(w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", `Bearer realm="Bearer token"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
}
