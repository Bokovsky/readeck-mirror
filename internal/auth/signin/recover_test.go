// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package signin_test

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestRecover(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	t.Run("recover views", func(t *testing.T) {
		client := app.Client()

		client.RT(t,
			WithTarget("/login/recover"),
			AssertStatus(200),
		)

		token := ""
		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login/recover"),
			WithBody(url.Values{
				"step":  {"0"},
				"email": {"user@localhost"},
			}),
			AssertStatus(200),
			WithAssert(func(t *testing.T, _ *Response) {
				require.Contains(t, app.LastEmail, "login/recover/")
				rx := regexp.MustCompile(
					regexp.QuoteMeta("http://"+client.URL.Host) + "/login/recover/(.+)\r\n",
				)
				m := rx.FindStringSubmatch(app.LastEmail)
				if len(m) < 2 {
					t.Fatal("could not find recovery link in last email")
				}
				token = m[1]
			}),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login/recover/"+token),
			WithBody(url.Values{
				"step":     {"2"},
				"password": {"09876543"},
			}),
			AssertStatus(200),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login"),
			WithBody(url.Values{
				"username": {"user"},
				"password": {"09876543"},
			}),
			AssertStatus(303),
			AssertRedirect("/"),
		)
	})

	t.Run("recover no user", func(t *testing.T) {
		client := app.Client()

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login/recover"),
			WithBody(url.Values{
				"step":  {"0"},
				"email": {"nope@localhost"},
			}),
			AssertStatus(200),
			WithAssert(func(t *testing.T, _ *Response) {
				require.Contains(t,
					app.LastEmail,
					"However, this email address is not associated with any account",
				)
			}),
		)
	})

	t.Run("recover steps", func(t *testing.T) {
		client := app.Client()

		client.RT(t,
			WithTarget("/login/recover/abcdefghijkl"),
			AssertStatus(200),
			AssertContains("Invalid recovery code"),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login/recover/abcdefghijkl"),
			WithBody(url.Values{"password": {"09876543"}}),
			AssertStatus(422),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login/recover/abcdefghijkl"),
			WithBody(url.Values{
				"step":     {"2"},
				"password": {"09876543"},
			}),
			AssertStatus(200),
			AssertContains("Invalid recovery code"),
		)
	})
}
