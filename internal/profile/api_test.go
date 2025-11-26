// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package profile_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestAPI(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := app.Client(WithToken("user"))

	client.RT(
		WithTarget("/api/profile"),
		AssertStatus(200),
		AssertJSON(`{
			"provider":{
				"name":"bearer token",
				"application":"tests",
				"id":"<<PRESENCE>>",
				"roles":["user"],
				"permissions":"<<PRESENCE>>"
			},
			"user":{
				"username":"user",
				"email":"user@localhost",
				"created":"<<PRESENCE>>",
				"updated":"<<PRESENCE>>",
				"settings": "<<PRESENCE>>"
			}
		}`),
	)(t)

	client.RT(
		WithMethod(http.MethodPatch),
		WithTarget("/api/profile"),
		WithBody(map[string]any{}),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>"
		}`),
	)(t)

	client.RT(
		WithMethod(http.MethodPatch),
		WithTarget("/api/profile"),
		WithBody(map[string]any{
			"username": " newuser ",
			"email":    " newuser@localhost ",
		}),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"email": "newuser@localhost",
			"updated": "<<PRESENCE>>",
			"username":"newuser"
		}`),
	)(t)

	client.RT(
		WithMethod(http.MethodPatch),
		WithTarget("/api/profile"),
		WithBody(map[string]any{
			"username": " ",
		}),
		AssertStatus(422),
		AssertJSON(`{
			"is_valid":false,
			"errors":null,
			"fields":{
				"email":{
					"is_null": false,
					"is_bound": false,
					"value": "<<PRESENCE>>",
					"errors":null
				},
				"username":{
					"is_null": false,
					"is_bound": true,
					"value":"",
					"errors":[
						"field is required"
					]
				},
				"settings_lang": "<<PRESENCE>>",
				"settings_addon_reminder": "<<PRESENCE>>",
				"settings_reader_width": "<<PRESENCE>>",
				"settings_reader_font": "<<PRESENCE>>",
				"settings_reader_font_size": "<<PRESENCE>>",
				"settings_reader_line_height": "<<PRESENCE>>",
				"settings_reader_justify": "<<PRESENCE>>",
				"settings_reader_hyphenation": "<<PRESENCE>>",
				"settings_email_epub_to": "<<PRESENCE>>",
				"settings_email_reply_to": "<<PRESENCE>>"
			}
		}`),
	)(t)

	client.RT(
		WithMethod(http.MethodPatch),
		WithTarget("/api/profile"),
		WithBody(map[string]any{
			"username": "user@localhost",
			"email":    "user",
		}),
		AssertStatus(422),
		AssertJSON(`{
			"is_valid":false,
			"errors":null,
			"fields":{
				"email":{
					"is_null": false,
					"is_bound": true,
					"value": "user",
					"errors":[
						"not a valid email address"
					]
				},
				"username":{
					"is_null": false,
					"is_bound": true,
					"value":"user@localhost",
					"errors":[
						"must contain English letters, digits, \"_\" and \"-\" only"
					]
				},
				"settings_lang": "<<PRESENCE>>",
				"settings_addon_reminder": "<<PRESENCE>>",
				"settings_reader_width": "<<PRESENCE>>",
				"settings_reader_font": "<<PRESENCE>>",
				"settings_reader_font_size": "<<PRESENCE>>",
				"settings_reader_line_height": "<<PRESENCE>>",
				"settings_reader_justify": "<<PRESENCE>>",
				"settings_reader_hyphenation": "<<PRESENCE>>",
				"settings_email_epub_to": "<<PRESENCE>>",
				"settings_email_reply_to": "<<PRESENCE>>"
			}
		}`),
	)(t)

	client.RT(
		WithMethod(http.MethodPut),
		WithTarget("/api/profile/password"),
		WithBody(map[string]any{
			"password": "newpassword",
		}),
		AssertStatus(200),
	)(t)

	client.RT(
		WithMethod(http.MethodPut),
		WithTarget("/api/profile/password"),
		WithBody(map[string]any{
			"password": "  ",
		}),
		AssertStatus(422),
		AssertJSON(`{
			"is_valid":false,
			"errors":null,
			"fields":{
				"current":{
					"is_null": true,
					"is_bound": false,
					"value": "",
					"errors":null
				},
				"password":{
					"is_null": false,
					"is_bound": true,
					"value":"  ",
					"errors":["password must be at least 8 character long"]
				}
			}
		}`),
	)(t)
}

func TestAPIDeleteToken(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	u1, err := NewTestUser("test1", "test1@localhost", "test1", "user")
	require.NoError(t, err)

	app.Users[u1.User.Username] = u1

	t.Run("delete foreign token", func(t *testing.T) {
		client := app.Client(WithToken("user"))

		client.RT(
			WithMethod(http.MethodDelete),
			WithTarget("/api/profile/tokens/"+u1.Token.UID),
			AssertStatus(404),
		)(t)
	})

	t.Run("delete own token", func(t *testing.T) {
		client := app.Client(WithToken("test1"))

		client.RT(WithTarget("/api/profile"), AssertStatus(200))(t)

		client.RT(
			WithMethod(http.MethodDelete),
			WithTarget("/api/profile/tokens/"+u1.Token.UID),
			AssertStatus(204),
		)(t)

		client.RT(WithTarget("/api/profile"), AssertStatus(401))(t)
	})
}
