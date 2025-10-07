// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package signin_test

import (
	"testing"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestAPI(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := NewClient(t, app)

	RunRequestSequence(t, client, "",
		RequestTest{
			Method:       "POST",
			Target:       "/api/auth",
			JSON:         map[string]string{},
			ExpectStatus: 400,
		},
		RequestTest{
			Method: "POST",
			Target: "/api/auth",
			JSON: map[string]string{
				"application": "test",
				"username":    "admin",
				"password":    "nope",
			},
			ExpectStatus: 403,
			ExpectJSON: `{
				"status":403,
				"message":"Invalid user and/or password"
			}`,
		},
		RequestTest{
			Method: "POST",
			Target: "/api/auth",
			JSON: map[string]string{
				"application": "test",
				"username":    "admin@localhost",
				"password":    "nope",
			},
			ExpectStatus: 403,
			ExpectJSON: `{
				"status":403,
				"message":"Invalid user and/or password"
			}`,
		},
		RequestTest{
			Method: "POST",
			Target: "/api/auth",
			JSON: map[string]string{
				"application": "test",
				"username":    "unknown",
				"password":    "whatever",
			},
			ExpectStatus: 403,
			ExpectJSON: `{
				"status":403,
				"message":"Invalid user and/or password"
			}`,
		},
		RequestTest{
			Method: "POST",
			Target: "/api/auth",
			JSON: map[string]string{
				"application": "test",
				"username":    "admin",
				"password":    "admin",
			},
			ExpectStatus: 201,
			ExpectJSON: `{
					"id": "<<PRESENCE>>",
					"token": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			Method: "GET",
			Target: "/api/profile",
			Headers: map[string]string{
				"Authorization": "Bearer {{ (index .History 0).JSON.token }}",
			},
			ExpectStatus: 200,
		},
		RequestTest{
			Method: "POST",
			Target: "/api/auth",
			JSON: map[string]string{
				"application": "test",
				"username":    "admin@localhost",
				"password":    "admin",
			},
			ExpectStatus: 201,
			ExpectJSON: `{
					"id": "<<PRESENCE>>",
					"token": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			Method: "POST",
			Target: "/api/auth",
			JSON: map[string]any{
				"application": "test",
				"username":    "admin@localhost",
				"password":    "admin",
				"roles":       []string{"bookmarks:read"},
			},
			ExpectStatus: 201,
			ExpectJSON: `{
					"id": "<<PRESENCE>>",
					"token": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			Method: "GET",
			Target: "/api/profile",
			Headers: map[string]string{
				"Authorization": "Bearer {{ (index .History 0).JSON.token }}",
			},
			ExpectStatus: 200,
			Assert: func(t *testing.T, r *Response) {
				r.AssertJQ(t, ".provider.roles", []any{"bookmarks:read"})
			},
		},
		RequestTest{
			Method: "POST",
			Target: "/api/auth",
			JSON: map[string]any{
				"application": "test",
				"username":    "admin@localhost",
				"password":    "admin",
				"roles":       []string{"scoped_admin_r", "scoped_bookmarks_r"},
			},
			ExpectStatus: 201,
			ExpectJSON: `{
					"id": "<<PRESENCE>>",
					"token": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			Method: "GET",
			Target: "/api/profile",
			Headers: map[string]string{
				"Authorization": "Bearer {{ (index .History 0).JSON.token }}",
			},
			ExpectStatus: 200,
			Assert: func(t *testing.T, r *Response) {
				r.AssertJQ(t, ".provider.roles", []any{"admin:read", "bookmarks:read"})
			},
		},
	)
}
