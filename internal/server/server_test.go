// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/http/request"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestInitRequest(t *testing.T) {
	configs.InitConfiguration()

	tests := []struct {
		RemoteAddr          string
		XForwardedFor       string
		XForwardedHost      string
		XForwardedProto     string
		ExpectedRemoteAddr  string
		ExpectedRemoteHost  string
		ExpectedRemoteProto string
	}{
		{
			"127.0.0.1:1234",
			"203.0.113.1, 192.168.2.1, ::1",
			"example.net",
			"https",
			"203.0.113.1",
			"example.net",
			"https",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1, 192.168.2.1, ::1",
			"example.net",
			"",
			"203.0.113.1",
			"example.net",
			"https",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1",
			"example.net:8443",
			"https",
			"203.0.113.1",
			"example.net:8443",
			"https",
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"203.0.113.1",
			"example.net",
			"https",
		},
		{
			"[::1]:1234",
			"2001:db8:fa::2",
			"example.net",
			"https",
			"2001:db8:fa::2",
			"example.net",
			"https",
		},
		{
			"[fd00::ff01]:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"203.0.113.1",
			"example.net",
			"https",
		},
		{
			"[2001:db8:ff::1]:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"2001:db8:ff::1",
			"test.local",
			"https",
		},
		{
			"128.66.1.1:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"128.66.1.1",
			"test.local",
			"https",
		},
		{
			"128.66.1.1:1234",
			"203.0.113.1",
			"",
			"",
			"128.66.1.1",
			"test.local",
			"https",
		},
		{
			"128.66.1.1:1234",
			"",
			"",
			"",
			"128.66.1.1",
			"test.local",
			"http",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			r, _ := http.NewRequest("GET", "/", nil)
			r.Host = "test.local"
			r.RemoteAddr = test.RemoteAddr
			r.Header.Set("X-Forwarded-For", test.XForwardedFor)
			r.Header.Set("X-Forwarded-Host", test.XForwardedHost)
			r.Header.Set("X-Forwarded-Proto", test.XForwardedProto)
			w := httptest.NewRecorder()

			var ru *url.URL
			var cu *url.URL
			var ip net.IP

			server.InitRequest()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				ru = r.URL
				cu = request.GetURL(r.Context())
				ip = request.GetRealIP(r.Context())
			})).ServeHTTP(w, r)

			assert := require.New(t)
			assert.Exactly(cu, ru)
			assert.Equal(test.ExpectedRemoteAddr, ip.String())
			assert.Equal(test.ExpectedRemoteHost, ru.Host)
			assert.Equal(test.ExpectedRemoteProto, ru.Scheme)
			assert.Equal(test.ExpectedRemoteProto+"://"+test.ExpectedRemoteHost+"/", ru.String())
		})
	}
}

func TestCsrfProtect(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := NewClient(t, app)
	app.Users["user"].Login(client)
	defer client.Logout()

	tests := []struct {
		name         string
		secFetchDest string
		secFetchMode string
		secFetchSite string
		origin       string
		expected     map[string]int
	}{
		{
			name:         "same-origin allowd",
			secFetchSite: "same-origin",
			expected:     map[string]int{"GET": http.StatusOK, "POST": http.StatusSeeOther},
		},
		{
			name:         "none allowed",
			secFetchSite: "none",
			expected:     map[string]int{"GET": http.StatusOK, "POST": http.StatusSeeOther},
		},
		{
			name:         "cross-site blocked",
			secFetchSite: "cross-site",
			expected:     map[string]int{"GET": http.StatusOK, "POST": http.StatusForbidden, "PUT": http.StatusForbidden},
		},
		{
			name:         "same-site blocked",
			secFetchSite: "same-site",
			expected:     map[string]int{"GET": http.StatusOK, "POST": http.StatusForbidden},
		},
		{
			name:         "navigate document from inside",
			secFetchDest: "document",
			secFetchMode: "navigate",
			secFetchSite: "same-origin",
			expected:     map[string]int{"GET": http.StatusOK, "POST": http.StatusSeeOther},
		},
		{
			name:         "navigate document user initiated",
			secFetchDest: "document",
			secFetchMode: "navigate",
			secFetchSite: "none",
			expected:     map[string]int{"GET": http.StatusOK, "POST": http.StatusSeeOther},
		},
		{
			name:         "navigate document from outside",
			secFetchDest: "document",
			secFetchMode: "navigate",
			secFetchSite: "cross-site",
			expected:     map[string]int{"GET": http.StatusOK, "POST": http.StatusForbidden},
		},
		{
			name:         "navigate embed inside",
			secFetchDest: "embed",
			secFetchMode: "navigate",
			secFetchSite: "same-origin",
			expected:     map[string]int{"GET": http.StatusOK},
		},
		{
			name:         "navigate frame inside",
			secFetchDest: "frame",
			secFetchMode: "navigate",
			secFetchSite: "same-origin",
			expected:     map[string]int{"GET": http.StatusOK},
		},
		{
			name:         "navigate iframe inside",
			secFetchDest: "iframe",
			secFetchMode: "navigate",
			secFetchSite: "same-origin",
			expected:     map[string]int{"GET": http.StatusOK},
		},
		{
			name:         "navigate object inside",
			secFetchDest: "object",
			secFetchMode: "navigate",
			secFetchSite: "same-origin",
			expected:     map[string]int{"GET": http.StatusOK},
		},
		{
			name:         "navigate embed from outside",
			secFetchDest: "embed",
			secFetchMode: "navigate",
			secFetchSite: "cross-site",
			expected:     map[string]int{"GET": http.StatusOK},
		},
		{
			name:         "navigate frame from outside",
			secFetchDest: "frame",
			secFetchMode: "navigate",
			secFetchSite: "cross-site",
			expected:     map[string]int{"GET": http.StatusOK},
		},
		{
			name:         "navigate iframe from outside",
			secFetchDest: "iframe",
			secFetchMode: "navigate",
			secFetchSite: "cross-site",
			expected:     map[string]int{"GET": http.StatusOK},
		},
		{
			name:         "navigate object from outside",
			secFetchDest: "object",
			secFetchMode: "navigate",
			secFetchSite: "cross-site",
			expected:     map[string]int{"GET": http.StatusOK},
		},

		{
			name:     "no sec-fetch no origin",
			expected: map[string]int{"GET": http.StatusOK, "POST": http.StatusSeeOther},
		},
		{
			name:     "no sec-fetch matching origin",
			origin:   fmt.Sprintf("%s://%s", client.URL.Scheme, client.URL.Host),
			expected: map[string]int{"GET": http.StatusOK, "POST": http.StatusSeeOther},
		},
		{
			name:     "no sec-fetch mismatched origin",
			origin:   "https://attacker.example",
			expected: map[string]int{"GET": http.StatusOK, "POST": http.StatusForbidden},
		},
		{
			name:     "no sec-fetch null origin",
			origin:   "null",
			expected: map[string]int{"GET": http.StatusOK, "POST": http.StatusForbidden},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for method, expected := range test.expected {
				t.Run(method, func(t *testing.T) {
					req := client.NewRequest(method, "/profile", nil)
					if req.Method != http.MethodGet {
						req.Body = io.NopCloser(strings.NewReader("username=user"))
						req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					}

					if test.secFetchMode != "" {
						req.Header.Set("Sec-Fetch-Mode", test.secFetchMode)
					}
					if test.secFetchSite != "" {
						req.Header.Set("Sec-Fetch-Site", test.secFetchSite)
					}
					if test.secFetchDest != "" {
						req.Header.Set("Sec-Fetch-Dest", test.secFetchDest)
					}
					if test.origin != "" {
						req.Header.Set("Origin", test.origin)
					}

					rsp := client.Request(req)

					if c := rsp.Header.Get("content-type"); rsp.StatusCode >= 400 && !strings.HasPrefix(c, "text/html;") {
						t.Errorf(`got content-type "%s", want "text/html"`, rsp.Header.Get("content-type"))
					}
					if rsp.StatusCode != expected {
						t.Errorf("got status %d, want %d", rsp.StatusCode, expected)
					}
				})
			}
		})
	}
}
