// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package signin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestSignin(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	t.Run("login view", func(t *testing.T) {
		type loginTest struct {
			user          string
			password      string
			loginStatus   int
			profileStatus int
		}
		tests := []loginTest{
			{"", "", 422, 303},
			{"admin", "admin", 303, 200},
			{"admin@localhost", "admin", 303, 200},
			{"user", "user", 303, 200},
			{"user@localhost", "user", 303, 200},
			{"disabled", "disabled", 303, 403},
			{"admin", "nope", 401, 303},
		}

		for _, test := range tests {
			t.Run(test.user, func(t *testing.T) {
				client := app.Client()

				client.RT(
					WithTarget("/"),
					AssertStatus(303),
					AssertRedirect("/login"),
				)(t)

				client.RT(
					WithTarget("/login"),
					AssertStatus(200),
				)(t)

				client.RT(
					WithMethod(http.MethodPost),
					WithTarget("/login"),
					WithBody(url.Values{
						"username": {test.user},
						"password": {test.password},
					}),
					AssertStatus(test.loginStatus),
					AssertRedirect(func() string {
						if test.loginStatus == 303 {
							return "/"
						}
						return ""
					}()),
				)(t)

				client.RT(
					WithTarget("/profile"),
					AssertStatus(test.profileStatus),
				)(t)
			})
		}
	})

	t.Run("logout view", func(t *testing.T) {
		client := app.Client()
		client.RT(
			WithMethod(http.MethodPost),
			WithTarget("/logout"),
			WithBody(url.Values{}),
			AssertStatus(303),
			AssertRedirect("/login"),
		)(t)

		WithSession("user")(client)
		client.RT(
			WithTarget("/profile"),
			AssertStatus(200),
			WithAssert(func(t *testing.T, rsp *Response) {
				require.Len(t, client.Jar.Cookies(rsp.URL), 1)
			}),
		)(t)

		client.RT(
			WithMethod(http.MethodPost),
			WithTarget("/logout"),
			WithBody(url.Values{}),
			AssertStatus(303),
			AssertRedirect("/"),
			WithAssert(func(t *testing.T, rsp *Response) {
				require.Empty(t, client.Jar.Cookies(rsp.URL))
			}),
		)(t)

		client.RT(
			WithTarget("/"),
			AssertStatus(303),
			AssertRedirect("/login"),
		)(t)
	})
}
