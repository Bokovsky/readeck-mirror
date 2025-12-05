// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package auth defines Readeck's authentication providers.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/pkg/ctxr"
	"codeberg.org/readeck/readeck/pkg/http/request"
)

type (
	ctxAuthInfoKey struct{}
	ctxProviderKey struct{}
)

var (
	withAuthInfo, checkAuthInfo = ctxr.WithChecker[*Info](ctxAuthInfoKey{})
	withProvider, checkProvider = ctxr.WithChecker[Provider](ctxProviderKey{})
)

// Info is the payload with the currently authenticated user
// and some information about the provider.
type Info struct {
	Provider *ProviderInfo
	User     *users.User
}

// ProviderInfo contains information about the provider.
type ProviderInfo struct {
	Name        string
	Application string
	Roles       []string
	ID          string
}

// Provider describes an authencitation provider.
// This is the minimal interface a provider must implements.
type Provider interface {
	Handler(next http.Handler) http.Handler
	Authenticate(http.ResponseWriter, *http.Request) (*http.Request, error)
}

// FeatureCsrfProvider allows a provider to implement a method
// to bypass all CSRF protection.
type FeatureCsrfProvider interface {
	// Must return true to disable CSRF protection for the request.
	CsrfExempt(*http.Request) bool
}

// FeaturePermissionProvider allows a provider to implement a permission
// check of its own. Usually providing scoped permissions.
type FeaturePermissionProvider interface {
	HasPermission(*http.Request, string, string) bool
	GetPermissions(*http.Request) []string
}

// NullProvider is the provider returned when no other provider
// could be activated.
type NullProvider struct{}

// Handler implements [Provider].
func (p *NullProvider) Handler(next http.Handler) http.Handler {
	return next
}

// Authenticate is a noop.
func (p *NullProvider) Authenticate(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
	return r, nil
}

// Init returns an [http.Handler] that iterates over the given providers.
// Once a provider adds itself to the request's context with [withProvider],
// the other providers are skipped and it becomes the default provider.
// This lets us do a few interesting things like:
// - stop when a provider meets some conditions (ie. an HTTP header)
// - prepare information for the next provider to pick up (for a forwarded auth)
// - return an HTTP response and terminate everything in case of error.
func Init(providers ...Provider) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Wrap the final handler so the request will alway have a [Provider] and [Info].
		next = defaultProviderHandler(next)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var h http.Handler

			if len(providers) == 0 {
				h = next
			} else {
				// Build the final handler with each provider wrapped in a handler
				// that allows to skip them when necessary.
				h = skipNextProviderHandler(providers[len(providers)-1].Handler(next), next)
				for i := len(providers) - 2; i >= 0; i-- {
					h = skipNextProviderHandler(providers[i].Handler(h), next)
				}
			}

			h.ServeHTTP(w, r)
		})
	}
}

// Required returns an [http.Handler] that will enforce authentication
// on the request. It uses the request authentication provider to perform
// the authentication.
//
// A provider performing a successful authentication must store
// its authentication information using [withAuthInfo].
//
// When the request has this attribute it will carry on.
// Otherwise it stops the response with a 403 error.
//
// The logged in user can be retrieved with [GetRequestUser].
func Required(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provider, ok := checkProvider(r.Context())
		if !ok {
			slog.Error("authentication",
				slog.String("@id", request.GetReqID(r.Context())),
				slog.Any("err", errors.New("no authentication provider")),
			)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		req, err := provider.Authenticate(w, r)
		if req != nil {
			r = req
		}

		if err != nil {
			slog.Error("authentication error",
				slog.String("@id", request.GetReqID(r.Context())),
				slog.Any("err", err),
			)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if _, ok := checkAuthInfo(r.Context()); !ok || GetRequestUser(r).IsAnonymous() {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
			authInfo := GetRequestAuthInfo(r)
			slog.Debug("authenticated user",
				slog.String("provider", authInfo.Provider.Name),
				slog.String("id", authInfo.Provider.ID),
				slog.String("user", authInfo.User.Username),
			)
		}

		next.ServeHTTP(w, r)
	})
}

// HasPermission returns true if the user that's connected can perform
// the action "act" on object "obj". It will check the user permissions
// and any scope given by the authentication provider.
func HasPermission(r *http.Request, obj, act string) bool {
	info := GetRequestAuthInfo(r)

	// Checked the scoped permissions if any
	// Note that the provider permission must be in the user's scope
	// to succeed.
	if p, ok := GetRequestProvider(r).(FeaturePermissionProvider); ok {
		return info.User.HasPermission(obj, act) && p.HasPermission(r, obj, act)
	}

	// Fallback to user permissions
	return info.User.HasPermission(obj, act)
}

// GetPermissions returns all the permissions available for the request.
// If the authentication provider implements it, a subset of permissions
// is sent, otherwise, the user own permissions is returned.
func GetPermissions(r *http.Request) []string {
	info := GetRequestAuthInfo(r)
	if info.User.IsAnonymous() {
		return []string{}
	}

	if p, ok := GetRequestProvider(r).(FeaturePermissionProvider); ok {
		if res := p.GetPermissions(r); res != nil {
			return res
		}
	}

	return info.User.Permissions()
}

// defaultProviderHandler is a final handler that sets [NullProvider]
// as the request's authentication provider when no other provider
// was set.
func defaultProviderHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Set a default provider
		if _, ok := checkProvider(ctx); !ok {
			ctx = withProvider(ctx, &NullProvider{})
		}

		// Always set a anonymous user and empty provider.
		// It's overridden later by the authentication when
		// entering the [Required] handler.
		ctx = withAuthInfo(ctx, &Info{
			Provider: &ProviderInfo{},
			User:     &users.User{},
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// skipNextProviderHandler is a [http.Handler] that jumps to the last handler
// when a [Provider] is already present in the request's context.
func skipNextProviderHandler(next http.Handler, last http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := checkProvider(r.Context()); ok {
			// we have a provider already, jump to the last handler.
			last.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetRequestProvider returns the current request's authentication
// provider.
func GetRequestProvider(r *http.Request) Provider {
	if p, ok := checkProvider(r.Context()); ok {
		return p
	}
	return nil
}

// GetRequestAuthInfo returns the current request's auth info.
func GetRequestAuthInfo(r *http.Request) *Info {
	info, _ := checkAuthInfo(r.Context())
	return info
}

// GetRequestUser returns the current request's user.
func GetRequestUser(r *http.Request) *users.User {
	return GetRequestAuthInfo(r).User
}
