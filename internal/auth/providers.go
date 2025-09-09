// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package auth defines Readeck's authentication providers.
package auth

import (
	"context"
	"log/slog"
	"net/http"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/pkg/http/request"
)

type (
	ctxProviderKey struct{}
	ctxAuthKey     struct{}
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

// Provider is the interface that must implement any authentication
// provider.
type Provider interface {
	// Must return true to enable the provider for the current request.
	IsActive(*http.Request) bool

	// Must return a request with the Info provided when successful.
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

// Info return information about the provider.
func (p *NullProvider) Info(_ *http.Request) *ProviderInfo {
	return &ProviderInfo{
		Name: "null",
	}
}

// IsActive is always false.
func (p *NullProvider) IsActive(_ *http.Request) bool {
	return false
}

// Authenticate doesn't do anything.
func (p *NullProvider) Authenticate(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
	return r, nil
}

// Init returns an http.Handler that will try to find a suitable
// authentication provider on each request. The first to return
// true with its IsActive() method becomes the request authentication
// provider.
//
// If no provider could be found, the NullProvider will then be used.
//
// The provider is then stored in the request's context and can be
// retrieved using GetRequestProvider().
func Init(providers ...Provider) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var provider Provider
			for _, p := range providers {
				if p.IsActive(r) {
					provider = p
					break
				}
			}

			// Set a default provider
			if provider == nil {
				provider = &NullProvider{}
			}
			r = setRequestProvider(r, provider)

			// Always set a anonymous user and empty provider.
			// It's overridden later by the authentication when
			// entering the [Required] handler.
			r = SetRequestAuthInfo(r, &Info{
				User:     &users.User{},
				Provider: &ProviderInfo{},
			})

			next.ServeHTTP(w, r)
		})
	}
}

// Required returns an http.Handler that will enforce authentication
// on the request. It uses the request authentication provider to perform
// the authentication.
//
// A provider performing a successful authentication must store
// its authentication information using SetRequestAuthInfo.
//
// When the request has this attribute it will carry on.
// Otherwise it stops the response with a 403 error.
//
// The logged in user can be retrieved with GetRequestUser().
func Required(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		provider := GetRequestProvider(r)
		r, err := provider.Authenticate(w, r)
		if err != nil {
			slog.Error("authentication error",
				slog.String("@id", request.GetReqID(r.Context())),
				slog.Any("err", err),
			)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if GetRequestUser(r).IsAnonymous() {
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
	}

	return http.HandlerFunc(fn)
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

// setRequestProvider stores the current provider for the request.
func setRequestProvider(r *http.Request, provider Provider) *http.Request {
	ctx := context.WithValue(r.Context(), ctxProviderKey{}, provider)
	return r.WithContext(ctx)
}

// GetRequestProvider returns the current request's authentication
// provider.
func GetRequestProvider(r *http.Request) Provider {
	return r.Context().Value(ctxProviderKey{}).(Provider)
}

// SetRequestAuthInfo stores the request's user.
func SetRequestAuthInfo(r *http.Request, info *Info) *http.Request {
	ctx := context.WithValue(r.Context(), ctxAuthKey{}, info)
	return r.WithContext(ctx)
}

// GetRequestAuthInfo returns the current request's auth info.
func GetRequestAuthInfo(r *http.Request) *Info {
	return r.Context().Value(ctxAuthKey{}).(*Info)
}

// GetRequestUser returns the current request's user.
func GetRequestUser(r *http.Request) *users.User {
	return GetRequestAuthInfo(r).User
}
