// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/auth/users"
)

type emptyProvider struct{}

func (p *emptyProvider) Handler(next http.Handler) http.Handler {
	return next
}

func (p *emptyProvider) Authenticate(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
	return r, nil
}

type defaultProvider struct{}

func (p *defaultProvider) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = WithProvider(ctx, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (p *defaultProvider) Authenticate(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
	return r, nil
}

type otherProvider struct{}

func (p *otherProvider) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = WithProvider(ctx, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (p *otherProvider) Authenticate(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
	return r, nil
}

type stopProvider struct{}

func (p *stopProvider) Handler(_ http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Test", "abcd")
		w.WriteHeader(204)
	})
}

func (p *stopProvider) Authenticate(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
	return r, nil
}

func TestHandler(t *testing.T) {
	tests := []struct {
		providers []Provider
		assert    func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request)
	}{
		{
			[]Provider{},
			func(t *testing.T, _ *httptest.ResponseRecorder, r *http.Request) {
				require.IsType(t, (*NullProvider)(nil), GetRequestProvider(r))
				require.Equal(t, &Info{
					Provider: &ProviderInfo{},
					User:     &users.User{},
				}, GetRequestAuthInfo(r))
				require.Equal(t, &users.User{}, GetRequestUser(r))
				require.True(t, GetRequestUser(r).IsAnonymous())
			},
		},
		{
			[]Provider{
				&emptyProvider{},
			},
			func(t *testing.T, _ *httptest.ResponseRecorder, r *http.Request) {
				require.IsType(t, (*NullProvider)(nil), GetRequestProvider(r))
				require.Equal(t, &Info{
					Provider: &ProviderInfo{},
					User:     &users.User{},
				}, GetRequestAuthInfo(r))
				require.Equal(t, &users.User{}, GetRequestUser(r))
				require.True(t, GetRequestUser(r).IsAnonymous())
			},
		},
		{
			[]Provider{
				&emptyProvider{},
				&defaultProvider{},
			},
			func(t *testing.T, _ *httptest.ResponseRecorder, r *http.Request) {
				require.IsType(t, (*defaultProvider)(nil), GetRequestProvider(r))
			},
		},
		{
			[]Provider{
				&emptyProvider{},
				&defaultProvider{},
				&otherProvider{},
			},
			func(t *testing.T, _ *httptest.ResponseRecorder, r *http.Request) {
				require.IsType(t, (*defaultProvider)(nil), GetRequestProvider(r))
			},
		},
		{
			[]Provider{
				&emptyProvider{},
				&otherProvider{},
			},
			func(t *testing.T, _ *httptest.ResponseRecorder, r *http.Request) {
				require.IsType(t, (*otherProvider)(nil), GetRequestProvider(r))
			},
		},
		{
			[]Provider{
				&emptyProvider{},
				&stopProvider{},
			},
			func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request) {
				require.Equal(t, 204, w.Code)
				require.Equal(t, "abcd", w.Header().Get("Test"))
				require.Nil(t, GetRequestProvider(r))
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			h := Init(test.providers...)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			h(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				req = req.WithContext(r.Context())
			})).ServeHTTP(w, req)

			test.assert(t, w, req)
		})
	}
}

type authHeaderProvider struct{}

func (p *authHeaderProvider) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.Header.Get("user") != "" {
			ctx = WithProvider(ctx, p)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (p *authHeaderProvider) Authenticate(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
	user := r.Header.Get("user")
	switch user {
	case "anonymous":
		ctx := WithAuthInfo(r.Context(), &Info{
			Provider: &ProviderInfo{
				Name: "header",
			},
			User: &users.User{},
		})
		return r.WithContext(ctx), nil
	case "error":
		return nil, errors.New("invalid user")
	default:
		ctx := WithAuthInfo(r.Context(), &Info{
			Provider: &ProviderInfo{
				Name: "header",
			},
			User: &users.User{ID: 1, Username: user},
		})
		return r.WithContext(ctx), nil
	}
}

type authHeaderProvider2 struct{}

func (p *authHeaderProvider2) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.Header.Get("x-user") != "" {
			ctx = WithProvider(ctx, p)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (p *authHeaderProvider2) Authenticate(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
	ctx := WithAuthInfo(r.Context(), &Info{
		Provider: &ProviderInfo{
			Name: "x-header",
		},
		User: &users.User{ID: 1, Username: r.Header.Get("x-user")},
	})
	return r.WithContext(ctx), nil
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		providers []Provider
		header    map[string]string
		assert    func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request)
	}{
		{
			[]Provider{},
			map[string]string{},
			func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request) {
				require.Equal(t, 403, w.Code)
				require.Nil(t, GetRequestAuthInfo(r))
			},
		},
		{
			[]Provider{&authHeaderProvider{}, &authHeaderProvider2{}},
			map[string]string{},
			func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request) {
				require.Equal(t, 403, w.Code)
				require.Nil(t, GetRequestAuthInfo(r))
			},
		},
		{
			[]Provider{&authHeaderProvider{}, &authHeaderProvider2{}},
			map[string]string{
				"user": "alice",
			},
			func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request) {
				require.Equal(t, 200, w.Code)
				require.Equal(t, &Info{
					Provider: &ProviderInfo{
						Name: "header",
					},
					User: &users.User{ID: 1, Username: "alice"},
				}, GetRequestAuthInfo(r))
			},
		},
		{
			[]Provider{&authHeaderProvider{}, &authHeaderProvider2{}},
			map[string]string{
				"user": "anonymous",
			},
			func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request) {
				require.Equal(t, 403, w.Code)
				require.Nil(t, GetRequestAuthInfo(r))
			},
		},
		{
			[]Provider{&authHeaderProvider{}, &authHeaderProvider2{}},
			map[string]string{
				"user": "error",
			},
			func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request) {
				require.Equal(t, 403, w.Code)
				require.Nil(t, GetRequestAuthInfo(r))
			},
		},
		{
			[]Provider{&authHeaderProvider{}, &authHeaderProvider2{}},
			map[string]string{
				"x-user": "test",
			},
			func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request) {
				require.Equal(t, 200, w.Code)
				require.Equal(t, &Info{
					Provider: &ProviderInfo{
						Name: "x-header",
					},
					User: &users.User{ID: 1, Username: "test"},
				}, GetRequestAuthInfo(r))
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range test.header {
				req.Header.Set(k, v)
			}

			h := Init(test.providers...)
			h(Required(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				req = req.WithContext(r.Context())
			}))).ServeHTTP(w, req)

			test.assert(t, w, req)
		})
	}
}
