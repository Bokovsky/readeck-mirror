// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestBookmarkAPIShare(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := app.Client(WithToken("user"))
	bookmarkID := app.Users["user"].Bookmarks[0].UID

	client.RT(t,
		WithMethod("GET"),
		WithTarget("/api/bookmarks/"+bookmarkID+"/share/link"),
		AssertStatus(201),
	)

	publicPath := client.History[0].Response.Redirect
	require.NotEmpty(t, publicPath, "public path is set")

	client.RT(t,
		WithTarget(publicPath),
		AssertStatus(200),
		AssertContains(`Shared by user`),
	)
}
