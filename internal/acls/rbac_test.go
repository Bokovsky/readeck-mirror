// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package acls_test

import (
	"fmt"
	"strings"
	"testing"

	"codeberg.org/readeck/readeck/internal/acls"
	"github.com/stretchr/testify/require"
)

func basePolicy() (acls.Permissions, []acls.Group) {
	return acls.Permissions{
			"/system/read":                     {"system", "read"},
			"/email/send":                      {"email", "send"},
			"/api/admin/read":                  {"api:admin:users", "read"},
			"/api/admin/write":                 {"api:admin:users", "write"},
			"/web/admin/read":                  {"admin:users", "read"},
			"/web/admin/write":                 {"admin:users", "write"},
			"/api/cookbook/read":               {"api:cookbook", "read"},
			"/web/cookbook/read":               {"cookbook", "read"},
			"/web/docs/read":                   {"docs", "read"},
			"/api/profile/read":                {"api:profile", "read"},
			"/api/profile/write":               {"api:profile", "write"},
			"/web/profile/read":                {"profile", "read"},
			"/web/profile/write":               {"profile", "write"},
			"/api/profile/tokens/delete":       {"api:profile:tokens", "delete"},
			"/web/profile/tokens/read":         {"profile:tokens", "read"},
			"/web/profile/tokens/write":        {"profile:tokens", "write"},
			"/api/bookmarks/read":              {"api:bookmarks", "read"},
			"/api/bookmarks/write":             {"api:bookmarks", "write"},
			"/api/bookmarks/export":            {"api:bookmarks", "export"},
			"/web/bookmarks/read":              {"bookmarks", "read"},
			"/web/bookmarks/write":             {"bookmarks", "write"},
			"/web/bookmarks/export":            {"bookmarks", "export"},
			"/api/bookmarks/collections/read":  {"api:bookmarks:collections", "read"},
			"/api/bookmarks/collections/write": {"api:bookmarks:collections", "write"},
			"/web/bookmarks/collections/read":  {"bookmarks:collections", "read"},
			"/web/bookmarks/collections/write": {"bookmarks:collections", "write"},
			"/api/bookmarks/import":            {"api:bookmarks", "import"},
			"/web/bookmarks/import":            {"bookmarks", "import"},
			"/api/opds/read":                   {"api:opds", "read"},
		}, []acls.Group{
			{
				Name: "",
				Grants: []string{
					"/email/send",
				},
			},
			{
				Name: "api_common",
				Grants: []string{
					"/api/profile/read",
					"/api/profile/tokens/delete",
				},
			},
			{
				Name: "user",
				Parents: []string{
					"__group__",
					"api_common",
				},
				Grants: []string{
					"/email/send",
					"/*/docs/read",
					"/*/profile/*",
					"/*/profile/tokens/*",
					"/*/bookmarks/read",
					"/*/bookmarks/write",
					"/*/bookmarks/export",
					"/*/bookmarks/collections/read",
					"/*/bookmarks/collections/write",
					"/*/bookmarks/import/write",
					"/api/opds/*",
				},
			},
			{
				Name: "staff",
				Parents: []string{
					"__group__",
					"user",
				},
				Grants: []string{
					"/system/*",
				},
			},
			{
				Name: "admin",
				Parents: []string{
					"__group__",
					"staff",
				},
				Grants: []string{
					"/*/admin/*",
					"/*/cookbook/*",
				},
			},
			{
				Name: "bookmarks:read",
				Parents: []string{
					"__token__",
					"__oauth__",
					"api_common",
				},
				Grants: []string{
					"/api/bookmarks/read",
					"/api/bookmarks/export",
					"/api/bookmarks/collections/read",
					"/api/opds/read",
					"/web/bookmarks/read",
				},
			},
			{
				Name: "bookmarks:write",
				Parents: []string{
					"__token__",
					"__oauth__",
					"api_common",
				},
				Grants: []string{
					"/api/bookmarks/write",
					"/api/bookmarks/collections/write",
				},
			},
			{
				Name: "admin:read",
				Parents: []string{
					"__token__",
					"api_common",
				},
				Grants: []string{
					"/api/admin/read",
					"/system/read",
				},
			},
			{
				Name: "admin:write",
				Parents: []string{
					"__token__",
					"api_common",
				},
				Grants: []string{
					"/api/admin/write",
				},
			},
			{
				Name: "less",
				Parents: []string{
					"user",
				},
				Grants: []string{
					"!/api/opds/*",
				},
			},
		}
}

func TestCheckPermission(t *testing.T) {
	tests := []struct {
		Group    string
		Obj      string
		Act      string
		Expected bool
	}{
		{"admin", "api:profile", "read", true},
		{"staff", "api:profile", "read", true},
		{"user", "api:profile", "read", true},
		{"", "api:profile", "read", false},

		{"admin", "api:profile:tokens", "delete", true},
		{"staff", "api:profile:tokens", "delete", true},
		{"user", "api:profile:tokens", "delete", true},
		{"", "api:profile:tokens", "delete", false},

		{"admin", "system", "read", true},
		{"staff", "system", "read", true},
		{"user", "system", "read", false},
		{"", "system", "read", false},

		{"admin", "api:admin:users", "read", true},
		{"staff", "api:admin:users", "read", false},
		{"user", "api:admin:users", "read", false},
		{"", "api:admin:users", "read", false},

		{"admin", "admin:users", "read", true},
		{"staff", "admin:users", "read", false},
		{"user", "admin:users", "read", false},
		{"", "admin:users", "read", false},

		{"admin", "bookmarks", "read", true},
		{"staff", "bookmarks", "read", true},
		{"user", "bookmarks", "read", true},
		{"", "bookmarks", "read", false},

		{"bookmarks:read", "api:bookmarks", "read", true},
		{"bookmarks:read", "api:bookmarks", "write", false},
		{"bookmarks:write", "api:bookmarks", "read", false},
		{"bookmarks:write", "api:bookmarks", "write", true},

		{"admin", "email", "send", true},
		{"staff", "email", "send", true},
		{"user", "email", "send", true},
		{"", "email", "send", true},

		{"unknown", "email", "send", false},

		{"user", "api:opds", "read", true},
		{"less", "api:opds", "read", false},
	}

	permissions, groups := basePolicy()
	policy := acls.NewPolicy(permissions, groups...)

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s-%s-%s", test.Group, test.Obj, test.Act), func(t *testing.T) {
			res := policy.Enforce(test.Group, test.Obj, test.Act)
			require.Equal(t, test.Expected, res)
		})
	}
}

func TestGetPermissions(t *testing.T) {
	tests := []struct {
		Groups   []string
		Expected []string
	}{
		{
			[]string{"admin:read"},
			[]string{"api:admin:users:read", "api:profile:read", "api:profile:tokens:delete", "system:read"},
		},
		{
			[]string{"admin:write"},
			[]string{"api:admin:users:write", "api:profile:read", "api:profile:tokens:delete"},
		},
		{
			[]string{"admin:read", "admin:write"},
			[]string{"api:admin:users:read", "api:admin:users:write", "api:profile:read", "api:profile:tokens:delete", "system:read"},
		},
		{
			[]string{"bookmarks:read"},
			[]string{"api:bookmarks:collections:read", "api:bookmarks:export", "api:bookmarks:read", "api:opds:read", "api:profile:read", "api:profile:tokens:delete", "bookmarks:read"},
		},
		{
			[]string{"bookmarks:write"},
			[]string{"api:bookmarks:collections:write", "api:bookmarks:write", "api:profile:read", "api:profile:tokens:delete"},
		},
		{
			[]string{"unknown"},
			nil,
		},
		{
			[]string{},
			nil,
		},
	}

	permissions, groups := basePolicy()
	policy := acls.NewPolicy(permissions, groups...)

	for _, test := range tests {
		t.Run(strings.Join(test.Groups, ","), func(t *testing.T) {
			res := policy.GetPermissions(test.Groups...)
			require.Equal(t, test.Expected, res)
		})
	}
}

func TestInGroup(t *testing.T) {
	tests := []struct {
		Src      string
		Dest     string
		Expected bool
	}{
		{"user", "user", true},
		{"user", "admin", true},
		{"admin", "user", false},
		{"bookmarks:read", "user", true},
		{"admin:read", "user", false},
		{"admin:read", "admin", true},
	}

	permissions, groups := basePolicy()
	policy := acls.NewPolicy(permissions, groups...)

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s in %s", test.Src, test.Dest), func(t *testing.T) {
			res := policy.InGroup(test.Src, test.Dest)
			require.Equal(t, test.Expected, res)
		})
	}
}

func TestListGroups(t *testing.T) {
	tests := []struct {
		parent   string
		expected []string
	}{
		{
			"__group__",
			[]string{"admin", "staff", "user"},
		},
		{
			"__token__",
			[]string{"admin:read", "admin:write", "bookmarks:read", "bookmarks:write"},
		},
		{
			"__oauth__",
			[]string{"bookmarks:read", "bookmarks:write"},
		},
	}

	permissions, groups := basePolicy()
	policy := acls.NewPolicy(permissions, groups...)

	for _, test := range tests {
		t.Run(test.parent, func(t *testing.T) {
			groups := policy.ListGroups(test.parent)
			require.Equal(t, test.expected, groups)
		})
	}
}

func TestDeletePermission(t *testing.T) {
	assert := require.New(t)

	permissions, groups := basePolicy()
	policy := acls.NewPolicy(permissions, groups...)

	assert.True(policy.Enforce("user", "email", "send"))
	assert.True(policy.Enforce("", "email", "send"))

	policy.DeletePermission("email", "send")
	assert.False(policy.Enforce("user", "email", "send"))
	assert.False(policy.Enforce("", "email", "send"))
}

func BenchmarkCheckPermission(b *testing.B) {
	tests := []struct {
		Group string
		Obj   string
		Act   string
	}{
		{"admin", "api:profile", "read"},
		{"", "api:profile", "read"},

		{"admin", "bookmarks", "read"},
		{"bookmarks:read", "api:bookmarks", "read"},
		{"", "bookmarks", "read"},

		{"user", "email", "send"},
		{"", "email", "send"},
	}

	permissions, groups := basePolicy()
	policy := acls.NewPolicy(permissions, groups...)

	for _, test := range tests {
		b.Run(fmt.Sprintf("%s-%s-%s", test.Group, test.Obj, test.Act), func(b *testing.B) {
			for b.Loop() {
				policy.Enforce(test.Group, test.Obj, test.Act)
			}
		})
	}
}

func BenchmarkGetPermissions(b *testing.B) {
	tests := [][]string{
		{"admin"},
		{"admin:read"},
		{"bookmarks:write"},
		{"admin", "user", "staff"},
		{"user", "bookmarks:write"},
	}

	permissions, groups := basePolicy()
	policy := acls.NewPolicy(permissions, groups...)

	for _, test := range tests {
		b.Run(strings.Join(test, ","), func(b *testing.B) {
			for b.Loop() {
				policy.GetPermissions(test...)
			}
		})
	}
}

func BenchmarkInGroup(t *testing.B) {
	tests := []struct {
		Src  string
		Dest string
	}{
		{"user", "user"},
		{"user", "admin"},
		{"admin", "user"},
		{"bookmarks:read", "user"},
		{"admin:read", "user"},
		{"admin:read", "admin"},
	}

	permissions, groups := basePolicy()
	policy := acls.NewPolicy(permissions, groups...)

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s in %s", test.Src, test.Dest), func(b *testing.B) {
			for b.Loop() {
				policy.InGroup(test.Src, test.Dest)
			}
		})
	}
}

func BenchmarkListGroups(b *testing.B) {
	permissions, groups := basePolicy()
	policy := acls.NewPolicy(permissions, groups...)

	for b.Loop() {
		policy.ListGroups("__group__")
	}
}
