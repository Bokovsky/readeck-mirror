// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server"
	"github.com/stretchr/testify/require"
)

func TestInitRequest(t *testing.T) {
	s := server.New()
	configs.InitConfiguration()

	tests := []struct {
		RemoteAddr          string
		XForwardedFor       string
		XForwardedHost      string
		XForwardedProto     string
		ExpectedRemoteAddr  string
		ExpectedRemoteHost  string
		ExpectedRemoteProto string
		HostInfo            *server.RemoteInfo
	}{
		{
			"127.0.0.1:1234",
			"203.0.113.1, 192.168.2.1, ::1",
			"example.net",
			"https",
			"203.0.113.1",
			"example.net",
			"https",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   true,
				IsForwarded: true,
				ProxyAddr:   net.ParseIP("127.0.0.1"),
				Host:        "example.net",
				Scheme:      "https",
			},
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1, 192.168.2.1, ::1",
			"example.net",
			"",
			"203.0.113.1",
			"example.net",
			"https",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   true,
				IsForwarded: true,
				ProxyAddr:   net.ParseIP("127.0.0.1"),
				Host:        "example.net",
				Scheme:      "https",
			},
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1",
			"example.net:8443",
			"https",
			"203.0.113.1",
			"example.net:8443",
			"https",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   true,
				IsForwarded: true,
				ProxyAddr:   net.ParseIP("127.0.0.1"),
				Host:        "example.net:8443",
				Scheme:      "https",
			},
		},
		{
			"127.0.0.1:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"203.0.113.1",
			"example.net",
			"https",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   true,
				IsForwarded: true,
				ProxyAddr:   net.ParseIP("127.0.0.1"),
				Host:        "example.net",
				Scheme:      "https",
			},
		},
		{
			"[::1]:1234",
			"2001:db8:fa::2",
			"example.net",
			"https",
			"2001:db8:fa::2",
			"example.net",
			"https",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   true,
				IsForwarded: true,
				ProxyAddr:   net.ParseIP("::1"),
				Host:        "example.net",
				Scheme:      "https",
			},
		},
		{
			"[fd00::ff01]:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"203.0.113.1",
			"example.net",
			"https",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   true,
				IsForwarded: true,
				ProxyAddr:   net.ParseIP("fd00::ff01"),
				Host:        "example.net",
				Scheme:      "https",
			},
		},
		{
			"[2001:db8:ff::1]:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"2001:db8:ff::1",
			"test.local",
			"https",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   false,
				IsForwarded: true,
				ProxyAddr:   net.ParseIP("2001:db8:ff::1"),
				Host:        "example.net",
				Scheme:      "https",
			},
		},
		{
			"128.66.1.1:1234",
			"203.0.113.1",
			"example.net",
			"https",
			"128.66.1.1",
			"test.local",
			"https",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   false,
				IsForwarded: true,
				ProxyAddr:   net.ParseIP("128.66.1.1"),
				Host:        "example.net",
				Scheme:      "https",
			},
		},
		{
			"128.66.1.1:1234",
			"203.0.113.1",
			"",
			"",
			"128.66.1.1",
			"test.local",
			"https",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   false,
				IsForwarded: true,
				ProxyAddr:   net.ParseIP("128.66.1.1"),
				Host:        "",
				Scheme:      "https",
			},
		},
		{
			"128.66.1.1:1234",
			"",
			"",
			"",
			"128.66.1.1",
			"test.local",
			"http",
			&server.RemoteInfo{
				IsForced:    false,
				IsTrusted:   false,
				IsForwarded: false,
				ProxyAddr:   nil,
				Host:        "",
				Scheme:      "",
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			h := s.InitRequest(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

			r, _ := http.NewRequest("GET", "/", nil)
			r.Host = "test.local"
			r.RemoteAddr = test.RemoteAddr
			r.Header.Set("X-Forwarded-For", test.XForwardedFor)
			r.Header.Set("X-Forwarded-Host", test.XForwardedHost)
			r.Header.Set("X-Forwarded-Proto", test.XForwardedProto)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert := require.New(t)
			assert.Equal(test.ExpectedRemoteAddr, r.RemoteAddr)
			assert.Equal(test.ExpectedRemoteHost, r.Host)
			assert.Equal(test.ExpectedRemoteProto, r.URL.Scheme)
			assert.Equal(test.ExpectedRemoteProto+"://"+test.ExpectedRemoteHost+"/", r.URL.String())
			assert.Equal(test.HostInfo, server.GetRemoteInfo(r))
		})
	}
}
