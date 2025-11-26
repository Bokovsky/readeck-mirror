// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestViews(t *testing.T) {
	app := NewTestApp(t)
	defer func() {
		app.Close(t)
	}()

	client := app.Client(WithSession("admin"))
	u1, err := NewTestUser("test1", "test1@localhost", "test1", "user")
	require.NoError(t, err)
	_ = u1

	t.Run("users", func(t *testing.T) {
		client.RT(t,
			WithTarget("/admin"),
			AssertStatus(303),
			AssertRedirect("/admin/users"),
		)

		client.RT(t,
			WithTarget("/admin/users"),
			AssertStatus(200),
			AssertContains("Users</h1>"),
		)

		client.RT(t,
			WithTarget("/admin/users/add"),
			AssertStatus(200),
			AssertContains("New User</h1>"),
		)

		// Create user
		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/admin/users/add"),
			WithBody(url.Values{}),
			AssertStatus(422),
			AssertContains("Please check your form for errors."),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/admin/users/add"),
			WithBody(url.Values{
				"username": {"test3@localhost"},
				"password": {"1234"},
				"email":    {"test3"},
				"group":    {"user"},
			}),
			AssertStatus(422),
			AssertContains("must contain English letters"),
			AssertContains("not a valid email address"),
		)

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/admin/users/add"),
			WithBody(url.Values{
				"username": {"test3"},
				"password": {"1234"},
				"email":    {"test3@localhost"},
				"group":    {"user"},
			}),
			AssertStatus(303),
			AssertRedirect(`^/admin/users/\w+$`),
		)

		client.RT(t,
			WithTarget(client.History[0].Response.Redirect),
			AssertStatus(200),
			AssertContains("test3</h1>"),
		)

		// Update user
		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/admin/users/"+u1.User.UID),
			WithBody(url.Values{}),
			AssertStatus(303),
			AssertRedirect("/admin/users/"+u1.User.UID),
		)

		client.RT(t,
			WithTarget(client.History[0].Response.Redirect),
			AssertStatus(200),
			AssertContains("test1</h1>"),
			AssertContains("<strong>User updated.</strong>"),
		)

		// Udpate current user
		client.RT(t,
			WithMethod("POST"),
			WithTarget("/admin/users/"+app.Users["admin"].User.UID),
			WithBody(url.Values{
				"username": {"test3@localhost"},
				"password": {"1234"},
				"email":    {"test3"},
				"group":    {"user"},
			}),
			AssertStatus(422),
			AssertContains("must contain English letter"),
			AssertContains("not a valid email address"),
		)

		client.RT(t,
			WithMethod("POST"),
			WithTarget("/admin/users/"+app.Users["admin"].User.UID),
			WithBody(url.Values{}),
			AssertStatus(303),
			AssertRedirect("/admin/users/"+app.Users["admin"].User.UID),
		)

		client.RT(t,
			WithTarget(client.History[0].Response.Redirect),
			AssertStatus(200),
			AssertContains("admin</h1>"),
			AssertContains("<strong>User updated.</strong>"),
		)

		// Delete user
		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/admin/users/"+u1.User.UID+"/delete"),
			AssertStatus(303),
			AssertRedirect("/admin/users"),
		)

		client.RT(t,
			WithTarget("/admin/users/"+u1.User.UID),
			AssertStatus(200),
			AssertContains("User will be removed in a few seconds"),
			WithAssert(func(t *testing.T, _ *Response) {
				assert := require.New(t)
				evt := map[string]interface{}{}

				// An event was sent
				assert.Len(Events().Records("task"), 1)
				assert.NoError(json.Unmarshal(Events().Records("task")[0], &evt))
				assert.Equal("user.delete", evt["name"])
				assert.InEpsilon(float64(u1.User.ID), evt["id"], 0)

				// There's a task in the store
				task := fmt.Sprintf("tasks:user.delete:%d", u1.User.ID)
				m := Store().Get(task)
				assert.NotEmpty(m)
			}),
		)

		// Cancel deletion
		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/admin/users/"+u1.User.UID+"/delete"),
			WithBody(url.Values{"cancel": {"1"}}),
			AssertStatus(303),
			AssertRedirect("/admin/users"),
			WithAssert(func(t *testing.T, _ *Response) {
				// The task is not in the store anymore
				task := fmt.Sprintf("tasks:user.delete:%d", u1.User.ID)
				m := Store().Get(task)
				require.Empty(t, m)
			}),
		)
	})
}
