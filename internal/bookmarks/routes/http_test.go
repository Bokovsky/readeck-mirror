// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

//nolint:gocyclo,gocognit
func TestPermissions(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	users := []string{"admin", "staff", "user", "disabled", ""}
	for _, user := range users {
		t.Run(user, func(t *testing.T) {
			u := app.Users[user]

			app.Client(WithToken(user)).Sequence(t,
				// API
				RT(
					WithTarget("/api/bookmarks"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/sync"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/api/bookmarks/sync"),
					WithBody(map[string]any{}),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/feed"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/api/bookmarks"),
					WithBody(map[string]any{}),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 422)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/"+u.Bookmarks[0].UID),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPatch),
					WithTarget("/api/bookmarks/"+u.Bookmarks[0].UID),
					WithBody(map[string]any{}),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/"+u.Bookmarks[0].UID+"/article"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/"+u.Bookmarks[0].UID+"/x/props.json"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/"+u.Bookmarks[0].UID+"/article.epub"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/"+u.Bookmarks[0].UID+"/article.md"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/annotations"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/collections"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/api/bookmarks/collections"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 422)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/collections/RuXBpzio59ktWTEHDodLPU"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 404)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPatch),
					WithTarget("/api/bookmarks/collections/RuXBpzio59ktWTEHDodLPU"),

					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 404)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodDelete),
					WithTarget("/api/bookmarks/collections/RuXBpzio59ktWTEHDodLPU"),

					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 404)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/labels"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithTarget("/api/bookmarks/labels?name=foo"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 404)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/api/bookmarks/import/text"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 422)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 401)
						}
					}),
				),
			)

			// Views
			app.Client(WithSession(user)).Sequence(t,
				RT(
					WithTarget("/bookmarks"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/unread"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/bookmarks"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 422)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/"+u.Bookmarks[0].UID+"/diagnosis"),
					WithHeader("x-turbo", "1"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/"+u.Bookmarks[0].UID),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/bookmarks/"+u.Bookmarks[0].UID),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 303)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/"+u.Bookmarks[0].UID),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/bookmarks/"+u.Bookmarks[0].UID+"/delete"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 303)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/collections"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/collections/add"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/bookmarks/collections/add"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 422)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/collections/RuXBpzio59ktWTEHDodLPU"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 404)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/highlights"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/labels"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/labels?name=foo"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 404)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/bookmarks/labels/delete?name=foo"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 404)
						case "disabled":
							r.AssertStatus(t, 403)
						default:
							r.AssertStatus(t, 303)
							r.AssertRedirect(t, "/login")
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/import"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 303)
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/import/NJXoidA6hYSoWyJ6cyuCo4"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 303)
						}
					}),
				),
				RT(
					WithTarget("/bookmarks/import/text"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 303)
						}
					}),
				),
				RT(
					WithMethod(http.MethodPost),
					WithTarget("/bookmarks/import/wallabag"),
					WithBody(url.Values{}),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 422)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 303)
						}
					}),
				),

				// Public bookmark's assets
				RT(
					WithTarget(fmt.Sprintf("/bm/%s/%s/img/icon.png",
						u.Bookmarks[0].UID[0:2],
						u.Bookmarks[0].UID,
					)),
					AssertStatus(200),
				),
				RT(
					WithTarget(fmt.Sprintf("/bm/%s/%s/_resources/KUhyzHK6GqcKLf4e4557qP.png",
						u.Bookmarks[0].UID[0:2],
						u.Bookmarks[0].UID,
					)),
					AssertStatus(200),
				),
			)
		})
	}
}
