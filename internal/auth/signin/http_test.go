// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package signin_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestSignin(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := NewClient(t, app)

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
			// Since we perform login, we run the tests with no user
			RunRequestSequence(t, client, "",
				RequestTest{
					Target:         "/",
					ExpectStatus:   303,
					ExpectRedirect: "/login",
				},
				RequestTest{
					Target:       "/login",
					ExpectStatus: 200,
				},
				RequestTest{
					Method: "POST",
					Target: "/login",
					Form: url.Values{
						"username": {test.user},
						"password": {test.password},
					},
					ExpectStatus: test.loginStatus,
					ExpectRedirect: func() string {
						if test.loginStatus == 303 {
							return "/"
						}
						return ""
					}(),
				},
				RequestTest{
					Target:       "/profile",
					ExpectStatus: test.profileStatus,
				},
			)
		}
	})

	t.Run("logout view", func(t *testing.T) {
		RunRequestSequence(t, client, "",
			RequestTest{
				Method:       "POST",
				Target:       "/logout",
				Form:         url.Values{},
				ExpectStatus: 303,
			},
		)
		RunRequestSequence(t, client, "user",
			RequestTest{
				Target:       "/profile",
				ExpectStatus: 200,
				Assert: func(t *testing.T, _ *Response) {
					require.Len(t, client.Cookies(), 1)
				},
			},
			RequestTest{
				Method:         "POST",
				Target:         "/logout",
				Form:           url.Values{},
				ExpectStatus:   303,
				ExpectRedirect: "/",
				Assert: func(t *testing.T, _ *Response) {
					require.Empty(t, client.Cookies())
				},
			},
			RequestTest{
				Target:         "/",
				ExpectStatus:   303,
				ExpectRedirect: "/login",
			},
		)
	})
}
