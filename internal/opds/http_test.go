// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package opds_test

import (
	"testing"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
	"github.com/stretchr/testify/require"
)

func TestPermissions(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	users := []string{"admin", "staff", "user", "disabled", ""}
	for _, user := range users {
		app.Client(WithToken(user)).Sequence(
			RT(
				WithTarget("/opds"),
				WithAssert(func(t *testing.T, rsp *Response) {
					switch user {
					case "admin", "staff", "user":
						rsp.AssertStatus(t, 200)
						require.Equal(t,
							"application/atom+xml; profile=opds-catalog; kind=navigation",
							rsp.Header.Get("content-type"))
					case "disabled":
						rsp.AssertStatus(t, 403)
					case "":
						rsp.AssertStatus(t, 401)
					}
				}),
			),
			RT(
				WithTarget("/opds/bookmarks/all"),
				WithAssert(func(t *testing.T, rsp *Response) {
					switch user {
					case "admin", "staff", "user":
						rsp.AssertStatus(t, 200)
						require.Equal(t,
							"application/atom+xml; profile=opds-catalog; kind=acquisition",
							rsp.Header.Get("content-type"))
					case "disabled":
						rsp.AssertStatus(t, 403)
					case "":
						rsp.AssertStatus(t, 401)
					}
				}),
			),
			RT(
				WithTarget("/opds/bookmarks/unread"),
				WithAssert(func(t *testing.T, rsp *Response) {
					switch user {
					case "admin", "staff", "user":
						rsp.AssertStatus(t, 200)
						require.Equal(t,
							"application/atom+xml; profile=opds-catalog; kind=acquisition",
							rsp.Header.Get("content-type"))
					case "disabled":
						rsp.AssertStatus(t, 403)
					case "":
						rsp.AssertStatus(t, 401)
					}
				}),
			),
		)(t)
	}
}
