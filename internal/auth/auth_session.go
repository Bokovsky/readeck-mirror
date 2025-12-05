// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"net/http"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/sessions"
)

// SessionAuthProvider is the last authentication provider.
// It's alway enabled in case of every previous provider failing.
type SessionAuthProvider struct {
	// A function that returns the request's session
	GetSession func(*http.Request) *sessions.Session

	// A function that sets a Location header when
	// authentication fails.
	UnauthorizedHandler func(http.ResponseWriter, *http.Request)
}

// Handler always set this provider to the request and it must come as the last one.
// If authentication fails, it will redirect to the login page.
func (p *SessionAuthProvider) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := withProvider(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Authenticate checks if the request's session cookie is valid and
// the user exists.
func (p *SessionAuthProvider) Authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	sess := p.GetSession(r)
	u, err := p.checkSession(sess)
	if u == nil || err != nil {
		p.clearSession(sess, w, r)
		return r, err
	}

	// At this point, the user is granted access.
	// We renew its session for another max age duration.
	sess.Save(w, r)
	ctx := withAuthInfo(r.Context(), &Info{
		Provider: &ProviderInfo{
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
	p.UnauthorizedHandler(w, r)
}
