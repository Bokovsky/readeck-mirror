// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"errors"
	"log/slog"
	"net/http"
	"path"
	"time"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/internal/sessions"
	"codeberg.org/readeck/readeck/pkg/ctxr"
	"codeberg.org/readeck/readeck/pkg/http/securecookie"
)

type (
	ctxSessionKey struct{}
	ctxFlashKey   struct{}
)

var (
	withSession, getSession             = ctxr.WithChecker[*sessions.Session](ctxSessionKey{})
	withFlashMessages, getFlashMessages = ctxr.WithChecker[[]sessions.FlashMessage](ctxFlashKey{})
)

var sessionHandler *securecookie.Handler

// InitSession creates the session handler.
func InitSession() (err error) {
	// Create the session handler
	sessionHandler = securecookie.NewHandler(
		securecookie.Key(configs.Keys.SessionKey()),
		securecookie.WithPath(path.Join(urls.Prefix())),
		securecookie.WithMaxAge(configs.Config.Server.Session.MaxAge),
		securecookie.WithName(configs.Config.Server.Session.CookieName),
	)

	return
}

// WithSession initialize a session handler that will be available
// on the included routes.
func WithSession() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Store session
			session, err := sessions.New(sessionHandler, r)
			if err != nil && !errors.Is(err, http.ErrNoCookie) {
				slog.Warn("session cookie", slog.Any("err", err))
			}

			// Add session to context
			ctx := r.Context()
			ctx = withSession(ctx, session)

			// If auth provider is not [auth.SessionAuthProvider], we're done
			if _, ok := auth.GetRequestProvider(r).(*auth.SessionAuthProvider); !ok {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// New session, we set the [Payload.LastUpdate] to now
			// in order to invalidate the HTTP cache.
			if session.IsNew {
				session.Payload.LastUpdate = time.Now()
			}

			// Pop messages and store then. We must do it before
			// anything is sent to the client.
			flashes := session.Flashes()
			ctx = withFlashMessages(ctx, flashes)
			if len(flashes) > 0 {
				session.Save(w, r)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetSession returns the session currently stored in context.
// It will panic (on purpose) if the route is not using the
// WithSession() middleware.
func GetSession(r *http.Request) *sessions.Session {
	if sess, ok := getSession(r.Context()); ok {
		return sess
	}
	return nil
}

// AddFlash saves a flash message in the session.
func AddFlash(w http.ResponseWriter, r *http.Request, typ, msg string) error {
	session := GetSession(r)
	session.AddFlash(typ, msg)
	return session.Save(w, r)
}

// Flashes returns the flash messages retrieved from the session
// in the session middleware.
func Flashes(r *http.Request) []sessions.FlashMessage {
	if msgs, ok := getFlashMessages(r.Context()); ok && msgs != nil {
		return msgs
	}
	return make([]sessions.FlashMessage, 0)
}
