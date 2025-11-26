// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package profile_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

//nolint:gocyclo,gocognit
func TestPermissions(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	users := []string{"admin", "user", "staff", "disabled", ""}
	for _, user := range users {
		t.Run(user, func(t *testing.T) {
			token := "abcdefghijkl"
			if u := app.Users[user]; u.Token != nil {
				token = u.Token.UID
			}

			// API
			app.Client(WithToken(user)).Sequence(t,
				RT(
					WithTarget("/api/profile"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/profile/tokens"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodDelete),
					WithTarget("/api/profile/tokens/notfound"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 404)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPatch),
					WithTarget("/api/profile"),
					WithBody(map[string]any{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPut),
					WithTarget("/api/profile/password"),
					WithBody(map[string]any{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 422)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 401)
						}
					}),
				),
			)

			// Views
			app.Client(WithSession(user)).Sequence(t,
				RT(
					WithTarget("/profile"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/profile"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 303)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/profile/password"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/profile/password"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 422)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/profile/tokens"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/profile/tokens"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 303)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/profile/tokens/"+token),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/profile/tokens/"+token),
					WithBody(url.Values{"application": {"test"}}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 303)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/profile/tokens/"+token+"/delete"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 303)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/profile/import"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/profile/import"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 422)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/profile/export"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/profile/export"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin", "staff", "user":
							rsp.AssertStatus(t, 200)
							require.Equal(t, "application/zip", rsp.Header.Get("content-type"))
						case "disabled":
							rsp.AssertStatus(t, 403)
						default:
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						}
					}),
				),
			)
		})
	}
}
