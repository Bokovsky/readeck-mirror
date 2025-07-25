// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package profile_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

//nolint:gocyclo
func TestPermissions(t *testing.T) {
	app := NewTestApp(t)
	defer func() {
		app.Close(t)
	}()

	client := NewClient(t, app)

	tokens := map[string]string{
		"admin":    app.Users["admin"].Token.UID,
		"user":     app.Users["user"].Token.UID,
		"staff":    app.Users["staff"].Token.UID,
		"disabled": app.Users["disabled"].Token.UID,
		"":         "abcdefgh",
	}

	users := []string{"admin", "staff", "user", "disabled", ""}
	for _, user := range users {
		RunRequestSequence(t, client, user,
			// API
			RequestTest{
				JSON:   true,
				Target: "/api/profile",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				JSON:   true,
				Target: "/api/profile/tokens",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				JSON:   true,
				Method: "DELETE",
				Target: "/api/profile/tokens/notfound",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 404)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Method: "PATCH",
				Target: "/api/profile",
				JSON:   map[string]string{},
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Method: "PUT",
				Target: "/api/profile/password",
				JSON:   map[string]string{},
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						require.Equal(t, 422, r.StatusCode)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 401)
					}
				},
			},

			// Views
			RequestTest{
				Target: "/profile",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/profile",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 303)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Target: "/profile/password",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/profile/password",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 422)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Target: "/profile/tokens",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/profile/tokens",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 303)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Target: "/profile/tokens/" + tokens[user],
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/profile/tokens/" + tokens[user],
				Form:   url.Values{"application": {"test"}},
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 303)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Target: "/profile/tokens/" + tokens[user],
			},
			RequestTest{
				Method: "POST",
				Target: "/profile/tokens/" + tokens[user] + "/delete",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 303)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Target: "/profile/import",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/profile/import",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 422)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Target: "/profile/export",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/profile/export",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
						require.Equal(t, "application/zip", r.Header.Get("content-type"))
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
		)
	}
}
