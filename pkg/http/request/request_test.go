// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package request_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/http/request"
)

func networkList(networks ...string) []*net.IPNet {
	res := []*net.IPNet{}
	for _, s := range networks {
		_, cidr, err := net.ParseCIDR(s)
		if err != nil {
			panic(err)
		}
		res = append(res, cidr)
	}

	return res
}

func TestXForwarded(t *testing.T) {
	tests := []struct {
		RemoteAddr       string
		XForwardedFor    string
		XForwardedHost   string
		XForwardedProto  string
		ExpectedRemoteIP string
		ExpectedRealIP   string
		ExpectedURL      string
	}{
		{
			"127.0.0.1:1234",
			"203.0.113.1, 192.168.2.1, ::1",
			"example.net",
			"https",
			"127.0.0.1",
			"203.0.113.1",
			"https://example.net/abc?test=1",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1, 192.168.2.1, ::1",
			"example.net",
			"http",
			"127.0.0.1",
			"203.0.113.1",
			"http://example.net/abc?test=1",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1, 192.168.2.1, ::1",
			"example.net",
			"",
			"127.0.0.1",
			"203.0.113.1",
			"https://example.net/abc?test=1",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1",
			"example.net:8443",
			"https",
			"127.0.0.1",
			"203.0.113.1",
			"https://example.net:8443/abc?test=1",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"127.0.0.1",
			"203.0.113.1",
			"https://example.net/abc?test=1",
		},
		{
			"[::1]:1234",
			"2001:db8:fa::2",
			"example.net",
			"https",
			"::1",
			"2001:db8:fa::2",
			"https://example.net/abc?test=1",
		},
		{
			"[fd00::ff01]:1234",
			"2001:db8:fa::2",
			"example.net",
			"https",
			"fd00::ff01",
			"2001:db8:fa::2",
			"https://example.net/abc?test=1",
		},
		{
			"[2001:db8:ff::1]:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"2001:db8:ff::1",
			"2001:db8:ff::1",
			"https://localhost/abc?test=1",
		},
		{
			"128.66.1.1:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"128.66.1.1",
			"128.66.1.1",
			"https://localhost/abc?test=1",
		},
		{
			"128.66.1.1:1234",
			"203.0.113.1",
			"",
			"",
			"128.66.1.1",
			"128.66.1.1",
			"https://localhost/abc?test=1",
		},
		{
			"128.66.1.1:1234",
			"",
			"",
			"",
			"128.66.1.1",
			"128.66.1.1",
			"http://localhost/abc?test=1",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			r := httptest.NewRequest("GET", "/abc?test=1", nil)
			r.Host = "localhost"
			r.RemoteAddr = test.RemoteAddr
			r.Header.Set("X-Forwarded-For", test.XForwardedFor)
			r.Header.Set("X-Forwarded-Host", test.XForwardedHost)
			r.Header.Set("X-Forwarded-Proto", test.XForwardedProto)

			var ctx context.Context
			w := httptest.NewRecorder()
			h := request.InitRequest(networkList(
				"127.0.0.0/8",
				"10.0.0.0/8",
				"172.16.0.0/12",
				"192.168.0.0/16",
				"fd00::/8",
				"::1/128",
			)...)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				ctx = r.Context()
			}))

			h.ServeHTTP(w, r)

			assert := require.New(t)
			assert.Equal(test.ExpectedURL, request.GetURL(ctx).String())
			assert.Equal(test.ExpectedRemoteIP, request.GetRemoteIP(ctx).String())
			assert.Equal(test.ExpectedRealIP, request.GetRealIP(ctx).String())
		})
	}
}

func TestBaseURL(t *testing.T) {
	tests := []struct {
		RemoteAddr       string
		XForwardedFor    string
		XForwardedHost   string
		XForwardedProto  string
		ExpectedRemoteIP string
		ExpectedRealIP   string
		ExpectedURL      string
	}{
		{
			"127.0.0.1:1234",
			"203.0.113.1, 192.168.2.1, ::1",
			"example.net",
			"https",
			"127.0.0.1",
			"203.0.113.1",
			"https://example.com/abc?test=1",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1, 192.168.2.1, ::1",
			"example.net",
			"",
			"127.0.0.1",
			"203.0.113.1",
			"https://example.com/abc?test=1",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1",
			"example.net:8443",
			"https",
			"127.0.0.1",
			"203.0.113.1",
			"https://example.com/abc?test=1",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"127.0.0.1",
			"203.0.113.1",
			"https://example.com/abc?test=1",
		},
		{
			"[::1]:1234",
			"2001:db8:fa::2",
			"example.net",
			"https",
			"::1",
			"2001:db8:fa::2",
			"https://example.com/abc?test=1",
		},
		{
			"[fd00::ff01]:1234",
			"2001:db8:fa::2",
			"example.net",
			"https",
			"fd00::ff01",
			"2001:db8:fa::2",
			"https://example.com/abc?test=1",
		},
		{
			"[2001:db8:ff::1]:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"2001:db8:ff::1",
			"2001:db8:ff::1",
			"https://example.com/abc?test=1",
		},
		{
			"128.66.1.1:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"128.66.1.1",
			"128.66.1.1",
			"https://example.com/abc?test=1",
		},
		{
			"128.66.1.1:1234",
			"203.0.113.1",
			"",
			"",
			"128.66.1.1",
			"128.66.1.1",
			"https://example.com/abc?test=1",
		},
		{
			"128.66.1.1:1234",
			"",
			"",
			"",
			"128.66.1.1",
			"128.66.1.1",
			"https://example.com/abc?test=1",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			r := httptest.NewRequest("GET", "/abc?test=1", nil)
			r.Host = "localhost"
			r.RemoteAddr = test.RemoteAddr
			r.Header.Set("X-Forwarded-For", test.XForwardedFor)
			r.Header.Set("X-Forwarded-Host", test.XForwardedHost)
			r.Header.Set("X-Forwarded-Proto", test.XForwardedProto)

			u, _ := url.Parse("https://example.com/")
			var ctx context.Context
			w := httptest.NewRecorder()

			chi.Chain(
				request.InitBaseURL(u),
				request.InitRequest(networkList(
					"127.0.0.0/8",
					"10.0.0.0/8",
					"172.16.0.0/12",
					"192.168.0.0/16",
					"fd00::/8",
					"::1/128",
				)...),
			).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				ctx = r.Context()
			})).ServeHTTP(w, r)

			assert := require.New(t)
			assert.Exactly(r.URL, request.GetURL(ctx))
			assert.Equal(test.ExpectedURL, request.GetURL(ctx).String())
			assert.Equal(test.ExpectedRemoteIP, request.GetRemoteIP(ctx).String())
			assert.Equal(test.ExpectedRealIP, request.GetRealIP(ctx).String())
		})
	}

	t.Run("nil_base_url", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/abc?test=1", nil)
		r.Host = "localhost"
		r.RemoteAddr = "[::1]:1234"
		r.Header.Set("X-Forwarded-For", "[::1]")
		r.Header.Set("X-Forwarded-Host", "example.net")
		r.Header.Set("X-Forwarded-Proto", "https")

		var ctx context.Context
		w := httptest.NewRecorder()

		chi.Chain(
			request.InitBaseURL(nil),
			request.InitRequest(networkList(
				"::1/128",
			)...),
		).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			ctx = r.Context()
		})).ServeHTTP(w, r)

		assert := require.New(t)
		assert.Exactly(r.URL, request.GetURL(ctx))
		assert.Equal("https://example.net/abc?test=1", request.GetURL(ctx).String())
	})
}
