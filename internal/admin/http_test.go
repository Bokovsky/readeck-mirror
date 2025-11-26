// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package admin_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestPermissions(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	u1, err := NewTestUser("test1", "test1@localhost", "test1", "user")
	require.NoError(t, err)

	u2, err := NewTestUser("test2", "test2@localhost", "test2", "user")
	require.NoError(t, err)

	users := []string{"admin", "staff", "user", "disabled", ""}
	for _, user := range users {
		t.Run(user, func(t *testing.T) {
			// API
			app.Client(WithToken(user)).Sequence(
				RT(
					WithTarget("/api/admin/users"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 200)
						case "":
							rsp.AssertStatus(t, 401)
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/api/admin/users"),
					WithBody(map[string]any{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 422)
						case "":
							rsp.AssertStatus(t, 401)
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
				RT(
					WithTarget("/api/admin/users/"+u1.User.UID),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 200)
						case "":
							rsp.AssertStatus(t, 401)
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPatch),
					WithTarget("/api/admin/users/"+u1.User.UID),
					WithBody(map[string]any{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 200)
						case "":
							rsp.AssertStatus(t, 401)
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
				RT(
					WithMethod(http.MethodDelete),
					WithTarget("/api/admin/users/"+u1.User.UID),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 204)
						case "":
							rsp.AssertStatus(t, 401)
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
			)(t)

			// Views
			app.Client(WithSession(user)).Sequence(
				RT(
					WithTarget("/admin"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/admin/users")
						case "":
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
				RT(
					WithTarget("/admin/users"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 200)
						case "":
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
				RT(
					WithTarget("/admin/users/add"),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 200)
						case "":
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/admin/users/add"),
					WithBody(map[string]any{}),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 422)
						case "":
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
				RT(
					WithTarget("/admin/users/"+u2.User.UID),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 200)
						case "":
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget(fmt.Sprintf("/admin/users/%s/delete", u2.User.UID)),
					WithAssert(func(t *testing.T, rsp *Response) {
						switch user {
						case "admin":
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/admin/users")
						case "":
							rsp.AssertStatus(t, 303)
							rsp.AssertRedirect(t, "/login")
						default:
							rsp.AssertStatus(t, 403)
						}
					}),
				),
			)(t)
		})
	}
}
