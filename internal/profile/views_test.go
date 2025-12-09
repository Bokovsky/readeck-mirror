// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package profile_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-shiori/dom"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/pkg/totp"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestViews(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	t.Run("profile", func(t *testing.T) {
		defer func() {
			require.NoError(t, app.Users["user"].Reset())
		}()

		client := app.Client(WithSession("user"))

		client.RT(t, WithTarget("/profile"), AssertStatus(200))

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget(client.History.PrevURL()),
			WithBody(url.Values{
				"username": {"user@localhost"},
				"email":    {"user"},
			}),
			AssertStatus(422),
			AssertContains("must contain English letters"),
			AssertContains("not a valid email address"),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget(client.History.PrevURL()),
			WithBody(url.Values{
				"username": {"user"},
			}),
			AssertStatus(303),
			AssertRedirect("/profile"),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget(client.History.PrevURL()),
			WithBody(url.Values{
				"username": {"    "},
			}),
			AssertStatus(422),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget(client.History.PrevURL()),
			WithBody(url.Values{
				"username": {"user"},
				"email":    {"invalid"},
			}),
			AssertStatus(422),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/profile/session"),
			WithBody(url.Values{
				"bookmark_list_display": {"grid"},
			}),
			AssertStatus(200),
			AssertJSON(`{
				"bookmark_list_display":"grid"
			}`),
		)
	})

	t.Run("password", func(t *testing.T) {
		defer func() {
			require.NoError(t, app.Users["user"].Reset())
		}()

		client := app.Client(WithSession("user"))

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/profile/password"),
			WithBody(url.Values{
				"action":   {"change"},
				"current":  {"user"},
				"password": {"user1234"},
			}),
			AssertStatus(303),
			AssertRedirect("/profile/password"),
		)

		// The session has been updated, we can still use the website
		client.RT(t, WithTarget("/profile"), AssertStatus(200))
	})

	t.Run("tokens", func(t *testing.T) {
		client := app.Client(WithSession("staff"))

		client.RT(t, WithTarget("/profile/tokens"), AssertStatus(200))

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/profile/tokens"),
			AssertStatus(303),
			AssertRedirect("/profile/tokens/.+"),
		)

		client.RT(t,
			WithTarget(client.History[0].Response.Redirect),
			AssertStatus(200),
			AssertContains("New token created"),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget(client.History.PrevURL()),
			WithBody(url.Values{"application": {"test"}}),
			AssertStatus(303),
			WithAssert(func(t *testing.T, rsp *Response) {
				rsp.AssertRedirect(t, rsp.Request.URL.Path)
			}),
		)

		// Delete token
		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget(client.History.PrevURL()+"/delete"),
			AssertStatus(303),
			AssertRedirect("/profile/tokens"),
		)

		client.RT(t,
			WithTarget(client.History[1].URL.String()),
			AssertStatus(200),
			WithAssert(func(t *testing.T, rsp *Response) {
				assert := require.New(t)

				_, tokenID := path.Split(rsp.URL.Path)
				token, err := tokens.Tokens.GetOne(goqu.C("uid").Eq(tokenID))
				if err != nil {
					t.Error(err)
				}

				// An event was sent
				assert.Len(Events().Records("task"), 1)
				evt := map[string]interface{}{}
				assert.NoError(json.Unmarshal(Events().Records("task")[0], &evt))
				assert.Equal("token.delete", evt["name"])
				assert.InDelta(float64(token.ID), evt["id"], 0)

				// There's a task in the store
				task := fmt.Sprintf("tasks:token.delete:%d", token.ID)
				m := Store().Get(task)
				assert.NotEmpty(m)

				payload := map[string]interface{}{}
				assert.NoError(json.Unmarshal([]byte(m), &payload))
				assert.InDelta(float64(20), payload["delay"], 0)
			}),
		)

		// Cancel deletion
		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget(client.History.PrevURL()+"/delete"),
			WithBody(url.Values{"cancel": {"1"}}),
			AssertStatus(303),
			AssertRedirect("/profile/tokens"),
		)

		client.RT(t,
			WithTarget(client.History[1].URL.String()),
			AssertStatus(200),
			WithAssert(func(t *testing.T, rsp *Response) {
				_, tokenID := path.Split(rsp.URL.Path)
				token, err := tokens.Tokens.GetOne(goqu.C("uid").Eq(tokenID))
				if err != nil {
					t.Error(err)
				}

				// The task is not in the store anymore
				task := fmt.Sprintf("tasks:token.delete:%d", token.ID)
				m := Store().Get(task)
				require.Empty(t, m)
			}),
		)
	})

	t.Run("enable totp", func(t *testing.T) {
		defer func() {
			app.Users["user"].User.TOTPSecret = nil
			require.NoError(t, app.Users["user"].Reset())
		}()

		client := app.Client(WithSession("user"))

		client.RT(t,
			WithTarget("/profile/password"),
			AssertStatus(200),
			AssertContains("Setup Verification Code</a>"),
		)

		client.RT(t,
			WithTarget("/profile/otp"),
			AssertStatus(200),
		)

		u, err := url.Parse(dom.GetAttribute(
			dom.QuerySelector(client.History[0].Response.HTML, "[data-totp-uri]"),
			"data-totp-uri",
		))
		require.NoError(t, err)
		secret := u.Query().Get("secret")
		require.NotEmpty(t, secret)

		code := totp.NewCode(secret)
		otp, err := code.OTP(time.Now().UTC())
		require.NoError(t, err)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/profile/otp"),
			WithBody(url.Values{
				"secret": {secret},
				"otp":    {otp},
			}),
			AssertStatus(303),
			AssertRedirect("/profile/password"),
		)

		client.RT(t,
			WithTarget(client.History[0].Response.Redirect),
			AssertStatus(200),
			AssertContains("Verification Code is now enabled"),
			AssertContains("Remove</button>"),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/profile/password"),
			WithBody(url.Values{"action": {"remove-totp"}}),
			AssertStatus(303),
			AssertRedirect("/profile/password"),
		)

		client.RT(t,
			WithTarget(client.History.PrevURL()),
			AssertStatus(200),
			AssertContains("Setup Verification Code</a>"),
		)
	})
	t.Run("enable totp ko", func(t *testing.T) {
		defer func() {
			app.Users["user"].User.TOTPSecret = nil
			require.NoError(t, app.Users["user"].Reset())
		}()

		client := app.Client(WithSession("user"))

		client.Assert(t, RT(
			WithTarget("/profile/otp"),
			AssertStatus(200),
		))

		u, err := url.Parse(dom.GetAttribute(
			dom.QuerySelector(client.History[0].Response.HTML, "[data-totp-uri]"),
			"data-totp-uri",
		))
		require.NoError(t, err)
		secret := u.Query().Get("secret")
		require.NotEmpty(t, secret)

		code := totp.NewCode(secret)

		tests := []struct {
			name   string
			code   func() (string, error)
			assert []TestOption
		}{
			{
				"too late",
				func() (string, error) { return code.OTP(time.Now().UTC().Add(2 * time.Minute)) },
				[]TestOption{
					AssertStatus(422),
					AssertContains("Invalid code"),
				},
			},
			{
				"wrong code",
				func() (string, error) { return totp.Generate().OTP(time.Now().UTC()) },
				[]TestOption{
					AssertStatus(422),
					AssertContains("Invalid code"),
				},
			},
			{
				"no code",
				func() (string, error) { return "", nil },
				[]TestOption{
					AssertStatus(422),
					AssertContains("field is required"),
				},
			},
			{
				"code too short",
				func() (string, error) { return "1111", nil },
				[]TestOption{
					AssertStatus(422),
					AssertContains("text must contain 6 characters"),
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				otp, err := test.code()
				require.NoError(t, err)

				client.RT(t,
					append([]TestOption{
						WithMethod(http.MethodPost),
						WithTarget("/profile/otp"),
						WithBody(url.Values{
							"secret": {secret},
							"otp":    {otp},
						}),
					}, test.assert...)...,
				)
			})
		}
	})
}
