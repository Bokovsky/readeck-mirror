// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package acls_test

import (
	"maps"
	"testing"

	"codeberg.org/readeck/readeck/internal/acls"
	"github.com/stretchr/testify/require"
)

var defaultResolved = map[string][]string{
	"": {
		"email:send",
	},
	"admin": {
		"admin:users:read",
		"admin:users:write",
		"api:admin:users:read",
		"api:admin:users:write",
		"api:bookmarks:collections:read",
		"api:bookmarks:collections:write",
		"api:bookmarks:export",
		"api:bookmarks:import:write",
		"api:bookmarks:read",
		"api:bookmarks:write",
		"api:cookbook:read",
		"api:opds:read",
		"api:profile:info",
		"api:profile:read",
		"api:profile:tokens:delete",
		"api:profile:tokens:read",
		"api:profile:write",
		"bookmarks:collections:read",
		"bookmarks:collections:write",
		"bookmarks:export",
		"bookmarks:import:write",
		"bookmarks:read",
		"bookmarks:write",
		"cookbook:read",
		"docs:read",
		"email:send",
		"profile:read",
		"profile:tokens:read",
		"profile:tokens:write",
		"profile:write",
		"system:read",
	},
	"admin:read": {
		"api:admin:users:read",
		"api:profile:info",
		"api:profile:tokens:delete",
		"system:read",
	},
	"admin:write": {
		"api:admin:users:write",
		"api:profile:info",
		"api:profile:tokens:delete",
	},
	"api_common": {
		"api:profile:info",
		"api:profile:tokens:delete",
	},
	"bookmarks:read": {
		"api:bookmarks:collections:read",
		"api:bookmarks:export",
		"api:bookmarks:read",
		"api:opds:read",
		"api:profile:info",
		"api:profile:tokens:delete",
		"bookmarks:read",
		"email:send",
	},
	"bookmarks:write": {
		"api:bookmarks:collections:write",
		"api:bookmarks:write",
		"api:profile:info",
		"api:profile:tokens:delete",
	},
	"profile:read": {
		"api:profile:info",
		"api:profile:read",
		"api:profile:tokens:delete",
	},
	"staff": {
		"api:bookmarks:collections:read",
		"api:bookmarks:collections:write",
		"api:bookmarks:export",
		"api:bookmarks:import:write",
		"api:bookmarks:read",
		"api:bookmarks:write",
		"api:opds:read",
		"api:profile:info",
		"api:profile:read",
		"api:profile:tokens:delete",
		"api:profile:tokens:read",
		"api:profile:write",
		"bookmarks:collections:read",
		"bookmarks:collections:write",
		"bookmarks:export",
		"bookmarks:import:write",
		"bookmarks:read",
		"bookmarks:write",
		"docs:read",
		"email:send",
		"profile:read",
		"profile:tokens:read",
		"profile:tokens:write",
		"profile:write",
		"system:read",
	},
	"user": {
		"api:bookmarks:collections:read",
		"api:bookmarks:collections:write",
		"api:bookmarks:export",
		"api:bookmarks:import:write",
		"api:bookmarks:read",
		"api:bookmarks:write",
		"api:opds:read",
		"api:profile:info",
		"api:profile:read",
		"api:profile:tokens:delete",
		"api:profile:tokens:read",
		"api:profile:write",
		"bookmarks:collections:read",
		"bookmarks:collections:write",
		"bookmarks:export",
		"bookmarks:import:write",
		"bookmarks:read",
		"bookmarks:write",
		"docs:read",
		"email:send",
		"profile:read",
		"profile:tokens:read",
		"profile:tokens:write",
		"profile:write",
	},
}

func TestDefaultPolicy(t *testing.T) {
	acls.Load()
	defer acls.Clear()

	permissions := map[string][]string{}
	for r := range acls.Roles() {
		permissions[r] = acls.GetPermissions(r)
	}

	require.Equal(t, defaultResolved, permissions)
	require.Equal(t, defaultResolved["user"], acls.GetPermissions("user"))

	require.True(t, acls.Enforce("user", "bookmarks", "read"))
	require.False(t, acls.Enforce("user", "admin:users", "read"))

	require.True(t, acls.InGroup("user", "admin"))

	require.True(t, acls.Enforce("user", "email", "send"))
	acls.DeletePermission("email", "send")
	require.False(t, acls.Enforce("user", "email", "send"))
}

func TestDefaultGroups(t *testing.T) {
	acls.Load()
	defer acls.Clear()

	require.Equal(t, []string{"admin", "staff", "user"}, acls.ListGroups("__group__"))
	require.Equal(t, []string{
		"admin:read",
		"admin:write",
		"bookmarks:read",
		"bookmarks:write",
		"profile:read",
	}, acls.ListGroups("__token_scope__"))

	require.Equal(t, []string{
		"bookmarks:read",
		"bookmarks:write",
		"profile:read",
	}, acls.ListGroups("__oauth_scope__"))
}

func TestExtraPolicy(t *testing.T) {
	acls.Load(acls.Group{
		Name: "test",
		Parents: []string{
			"__group__",
			"user",
		},
		Grants: []string{
			"!/api/opds/*",
		},
	})
	defer acls.Clear()

	permissions := map[string][]string{}
	for r := range acls.Roles() {
		permissions[r] = acls.GetPermissions(r)
	}

	expected := map[string][]string{}
	maps.Copy(expected, defaultResolved)
	expected["test"] = []string{
		"api:bookmarks:collections:read",
		"api:bookmarks:collections:write",
		"api:bookmarks:export",
		"api:bookmarks:import:write",
		"api:bookmarks:read",
		"api:bookmarks:write",
		"api:profile:info",
		"api:profile:read",
		"api:profile:tokens:delete",
		"api:profile:tokens:read",
		"api:profile:write",
		"bookmarks:collections:read",
		"bookmarks:collections:write",
		"bookmarks:export",
		"bookmarks:import:write",
		"bookmarks:read",
		"bookmarks:write",
		"docs:read",
		"email:send",
		"profile:read",
		"profile:tokens:read",
		"profile:tokens:write",
		"profile:write",
	}

	require.Equal(t, expected, permissions)

	require.Equal(t, []string{"admin", "staff", "test", "user"}, acls.ListGroups("__group__"))

	require.True(t, acls.Enforce("user", "api:opds", "read"))
	require.False(t, acls.Enforce("test", "api:opds", "read"))
}
