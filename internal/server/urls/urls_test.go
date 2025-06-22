// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package urls_test

import (
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"github.com/stretchr/testify/require"
)

func TestPrefix(t *testing.T) {
	t.Run("unconfigured", func(t *testing.T) {
		configs.Config.Server.Prefix = "/app/"
		defer func() {
			configs.Config.Server.Prefix = ""
		}()
		require.Equal(t, "/app/", urls.Prefix())
	})
}

func TestCurrentPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{
			"/app/test/123",
			"/test/123",
		},
		{
			"/app/test/123?a=1",
			"/test/123?a=1",
		},
		{
			"/some/path",
			"",
		},
	}

	configs.Config.Server.Prefix = "/app/"
	defer func() {
		configs.Config.Server.Prefix = ""
	}()

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			r := httptest.NewRequest("GET", test.path, nil)

			require.Equal(t, test.expected, urls.CurrentPath(r))
		})
	}
}

func TestAbsoluteURL(t *testing.T) {
	tests := []struct {
		parts    []string
		expected string
	}{
		{
			[]string{},
			"https://example.net/app/some/path",
		},
		{
			[]string{"/"},
			"https://example.net/app/",
		},
		{
			[]string{"/test"},
			"https://example.net/app/test",
		},
		{
			[]string{"./test"},
			"https://example.net/app/some/path/test",
		},
		{
			[]string{"/test", "../t", "abc"},
			"https://example.net/app/t/abc",
		},
		{
			[]string{"../t", "abc"},
			"https://example.net/app/t/abc",
		},
		{
			[]string{"/test?a=1&b=2"},
			"https://example.net/app/test?a=1&b=2",
		},
	}

	r := httptest.NewRequest("GET", "/app/some/path", nil)
	r.URL.Scheme = "https"
	r.URL.Host = "example.net"

	configs.Config.Server.Prefix = "/app/"
	defer func() {
		configs.Config.Server.Prefix = ""
	}()

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			res := urls.AbsoluteURL(r, test.parts...)
			require.Equal(t, test.expected, res.String())
		})
	}
}

func TestPathOnly(t *testing.T) {
	mustParse := func(s string) *url.URL {
		u, err := url.Parse(s)
		if err != nil {
			panic(err)
		}
		return u
	}

	tests := []struct {
		src      *url.URL
		expected string
	}{
		{
			mustParse("https://example.net/some/path"),
			"/some/path",
		},
		{
			mustParse("https://example.net/some/path?abc=1"),
			"/some/path?abc=1",
		},
		{
			mustParse("https://example.net/some/path?abc=1#test.123"),
			"/some/path?abc=1#test.123",
		},
		{
			mustParse(""),
			"",
		},
		{
			mustParse("some/path"),
			"/some/path",
		},
		{
			mustParse("/some/path?abc=1"),
			"/some/path?abc=1",
		},
		{
			mustParse("some/path?abc=1#test.123"),
			"/some/path?abc=1#test.123",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			require.Equal(t, test.expected, urls.PathOnly(test.src))
		})
	}
}
