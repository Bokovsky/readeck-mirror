// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package signin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestAPI(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := app.Client()

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/auth"),
		WithBody(map[string]string{}),
		AssertStatus(400),
	)

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/auth"),
		WithBody(map[string]string{
			"application": "test",
			"username":    "admin",
			"password":    "nope",
		}),
		AssertStatus(403),
		AssertJSON(`{
			"status":403,
			"message":"Invalid user and/or password"
		}`),
	)

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/auth"),
		WithBody(map[string]string{
			"application": "test",
			"username":    "admin@localhost",
			"password":    "nope",
		}),
		AssertStatus(403),
		AssertJSON(`{
			"status":403,
			"message":"Invalid user and/or password"
		}`),
	)

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/auth"),
		WithBody(map[string]string{
			"application": "test",
			"username":    "unknown",
			"password":    "whatever",
		}),
		AssertStatus(403),
		AssertJSON(`{
			"status":403,
			"message":"Invalid user and/or password"
		}`),
	)

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/auth"),
		WithBody(map[string]string{
			"application": "test",
			"username":    "admin",
			"password":    "admin",
		}),
		AssertStatus(201),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"token": "<<PRESENCE>>"
		}`),
	)

	token := client.History[0].Response.JSON.(map[string]any)["token"].(string)
	client.RT(t,
		WithTarget("/api/profile"),
		WithHeader("Authorization", "Bearer "+token),
		AssertStatus(200),
	)

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/auth"),
		WithBody(map[string]string{
			"application": "test",
			"username":    "admin@localhost",
			"password":    "admin",
		}),
		AssertStatus(201),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"token": "<<PRESENCE>>"
		}`),
	)

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/auth"),
		WithBody(map[string]any{
			"application": "test",
			"username":    "admin@localhost",
			"password":    "admin",
			"roles":       []string{"bookmarks:read"},
		}),
		AssertStatus(201),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"token": "<<PRESENCE>>"
		}`),
	)

	token = client.History[0].Response.JSON.(map[string]any)["token"].(string)
	client.RT(t,
		WithTarget("/api/profile"),
		WithHeader("Authorization", "Bearer "+token),
		AssertStatus(200),
		WithAssert(func(t *testing.T, rsp *Response) {
			require.Equal(t,
				[]any{"bookmarks:read"},
				rsp.JSON.(map[string]any)["provider"].(map[string]any)["roles"],
			)
		}),
	)

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/auth"),
		WithBody(map[string]any{
			"application": "test",
			"username":    "admin@localhost",
			"password":    "admin",
			"roles":       []string{"scoped_admin_r", "scoped_bookmarks_r"},
		}),
		AssertStatus(201),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"token": "<<PRESENCE>>"
		}`),
	)

	token = client.History[0].Response.JSON.(map[string]any)["token"].(string)
	client.RT(t,
		WithTarget("/api/profile"),
		WithHeader("Authorization", "Bearer "+token),
		AssertStatus(200),
		WithAssert(func(t *testing.T, rsp *Response) {
			require.Equal(t,
				[]any{"admin:read", "bookmarks:read"},
				rsp.JSON.(map[string]any)["provider"].(map[string]any)["roles"],
			)
		}),
	)
}
