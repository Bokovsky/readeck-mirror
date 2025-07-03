// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes_test

import (
	"net/url"
	"testing"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

//nolint:gocyclo,gocognit
func TestPermissions(t *testing.T) {
	app := NewTestApp(t)
	defer func() {
		app.Close(t)
	}()

	client := NewClient(t, app)

	users := []string{"admin", "staff", "user", "disabled", ""}
	for _, user := range users {
		RunRequestSequence(t, client, user,
			// API
			RequestTest{
				Target: "/api/bookmarks",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/sync",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/api/bookmarks/sync",
				JSON:   map[string]any{},
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/feed",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/api/bookmarks",
				JSON:   map[string]string{},
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 422)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/{{(index .User.Bookmarks 0).UID}}",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Method: "PATCH",
				Target: "/api/bookmarks/{{(index .User.Bookmarks 0).UID}}",
				JSON:   map[string]any{},
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/{{(index .User.Bookmarks 0).UID}}/article",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/{{(index .User.Bookmarks 0).UID}}/x/props.json",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/{{(index .User.Bookmarks 0).UID}}/article.epub",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/{{(index .User.Bookmarks 0).UID}}/article.md",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/annotations",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/collections",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/api/bookmarks/collections",
				JSON:   true,
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 422)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/collections/RuXBpzio59ktWTEHDodLPU",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 404)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Method: "PATCH",
				Target: "/api/bookmarks/collections/RuXBpzio59ktWTEHDodLPU",
				JSON:   true,
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 404)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Method: "DELETE",
				Target: "/api/bookmarks/collections/RuXBpzio59ktWTEHDodLPU",
				JSON:   true,
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 404)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/labels",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Target: "/api/bookmarks/labels/foo",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 404)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/api/bookmarks/import/text",
				JSON:   false,
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 422)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 401)
					}
				},
			},
			// Views
			RequestTest{
				Target: "/bookmarks",
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
				Target: "/bookmarks/unread",
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
				Target: "/bookmarks",
				Form:   make(url.Values),
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
				Target: "/bookmarks/{{(index .User.Bookmarks 0).UID}}/diagnosis",
				Headers: map[string]string{
					"x-turbo": "1",
				},
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
				Target: "/bookmarks/{{(index .User.Bookmarks 0).UID}}",
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
				Target: "/bookmarks/{{(index .User.Bookmarks 0).UID}}",
				Form:   url.Values{},
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
				Target: "/bookmarks/{{(index .User.Bookmarks 0).UID}}",
			},
			RequestTest{
				Method: "POST",
				Target: "/bookmarks/{{(index .User.Bookmarks 0).UID}}/delete",
				Form:   url.Values{},
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
				Target: "/bookmarks/collections",
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
				Target: "/bookmarks/collections/add",
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
				Target: "/bookmarks/collections/add",
				Form:   url.Values{},
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
				Target: "/bookmarks/collections/RuXBpzio59ktWTEHDodLPU",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 404)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Target: "/bookmarks/highlights",
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
				Target: "/bookmarks/labels",
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
				Target: "/bookmarks/labels/foo",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 404)
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
				Target: "/bookmarks/labels/foo/delete",
				Form:   url.Values{},
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 404)
					case "disabled":
						r.AssertStatus(t, 403)
					default:
						r.AssertStatus(t, 303)
						r.AssertRedirect(t, "/login")
					}
				},
			},
			RequestTest{
				Target: "/bookmarks/import",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 303)
					}
				},
			},
			RequestTest{
				Target: "/bookmarks/import/NJXoidA6hYSoWyJ6cyuCo4",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 303)
					}
				},
			},
			RequestTest{
				Target: "/bookmarks/import/text",
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 200)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 303)
					}
				},
			},
			RequestTest{
				Method: "POST",
				Target: "/bookmarks/import/wallabag",
				Form:   url.Values{},
				Assert: func(t *testing.T, r *Response) {
					switch user {
					case "admin", "staff", "user":
						r.AssertStatus(t, 422)
					case "disabled":
						r.AssertStatus(t, 403)
					case "":
						r.AssertStatus(t, 303)
					}
				},
			},

			// Public bookmark's assets
			RequestTest{
				Target:       "/bm/{{(slice (index .User.Bookmarks 0).UID 0 2)}}/{{(index .User.Bookmarks 0).UID}}/img/icon.png",
				ExpectStatus: 200,
			},
			RequestTest{
				Target:       "/bm/{{(slice (index .User.Bookmarks 0).UID 0 2)}}/{{(index .User.Bookmarks 0).UID}}/_resources/KUhyzHK6GqcKLf4e4557qP.png",
				ExpectStatus: 200,
			},
		)
	}
}
