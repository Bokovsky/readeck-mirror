// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package acls

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func basePolicy() io.Reader {
	return bytes.NewBuffer([]byte(`
p, /system/read,    system, read
p, /email/send,     email, send
p, /api/admin/read,     api:admin:users,    read
p, /api/admin/write,    api:admin:users,    write
p, /web/admin/read,     admin:users,        read
p, /web/admin/write,    admin:users,        write
p, /api/cookbook/read,  api:cookbook,   read
p, /web/cookbook/read,  cookbook,       read
p, /web/docs/read,      docs,           read
p, /api/profile/read,   api:profile,    read
p, /api/profile/write,  api:profile,    write
p, /web/profile/read,   profile,        read
p, /web/profile/write,  profile,        write
p, /api/profile/tokens/delete,  api:profile:tokens, delete
p, /web/profile/tokens/read,    profile:tokens,     read
p, /web/profile/tokens/write,   profile:tokens,     write
p, /api/bookmarks/read,     api:bookmarks,  read
p, /api/bookmarks/write,    api:bookmarks,  write
p, /api/bookmarks/export,   api:bookmarks,  export
p, /web/bookmarks/read,     bookmarks,      read
p, /web/bookmarks/write,    bookmarks,      write
p, /web/bookmarks/export,   bookmarks,      export
p, /api/bookmarks/collections/read,     api:bookmarks:collections,  read
p, /api/bookmarks/collections/write,    api:bookmarks:collections,  write
p, /web/bookmarks/collections/read,     bookmarks:collections,      read
p, /web/bookmarks/collections/write,    bookmarks:collections,      write
p, /api/bookmarks/import/write,  api:bookmarks:import,  write
p, /web/bookmarks/import/write,  bookmarks:import,      write
p, /api/opds/read,  api:opds,   read

# groups
g, api_common, /api/profile/read
g, api_common, /api/profile/tokens/delete
g,, /email/send
g, user, __group__
g, user, api_common
g, user, /email/send
g, user, /*/docs/read
g, user, /*/profile/*
g, user, /*/profile/tokens/*
g, user, /*/bookmarks/read
g, user, /*/bookmarks/write
g, user, /*/bookmarks/export
g, user, /*/bookmarks/collections/read
g, user, /*/bookmarks/collections/write
g, user, /*/bookmarks/import/write
g, user, /api/opds/*
g, staff, __group__
g, staff, user
g, staff, /system/*
g, admin, __group__
g, admin, staff
g, admin, /*/admin/*
g, admin, /*/cookbook/*

# scopes
g, bookmarks:read, __token__
g, bookmarks:read, __oauth__
g, bookmarks:read, api_common
g, bookmarks:read, /api/bookmarks/read
g, bookmarks:read, /api/bookmarks/export
g, bookmarks:read, /api/bookmarks/collections/read
g, bookmarks:read, /api/opds/read
g, bookmarks:read, /web/bookmarks/read

g, bookmarks:write, __token__
g, bookmarks:write, __oauth__
g, bookmarks:write, api_common
g, bookmarks:write, /api/bookmarks/write
g, bookmarks:write, /api/bookmarks/collections/write

g, admin:read, __token__
g, admin:read, api_common
g, admin:read, /api/admin/read
g, admin:read, /system/read

g, admin:write, __token__
g, admin:write, api_common
g, admin:write, /api/admin/write
`))
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
	}

	policy, err := LoadPolicy(basePolicy())
	require.NoError(t, err)

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
			[]string{},
		},
	}

	policy, err := LoadPolicy(basePolicy())
	require.NoError(t, err)

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

	policy, err := LoadPolicy(basePolicy())
	require.NoError(t, err)

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

	policy, err := LoadPolicy(basePolicy())
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.parent, func(t *testing.T) {
			groups := policy.ListGroups(test.parent)
			require.Equal(t, test.expected, groups)
		})
	}
}

func TestLoad(t *testing.T) {
	policy, err := LoadPolicy(basePolicy())
	require.NoError(t, err)

	for name, role := range policy {
		println(">>", name)
		for g := range role.Parents {
			println("  G:", g)
		}
		for _, p := range role.ListPermissions() {
			println("   -", p)
		}
	}
}

func TestDeletePermission(t *testing.T) {
	assert := require.New(t)

	policy, err := LoadPolicy(basePolicy())
	assert.NoError(err)

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

	policy, err := LoadPolicy(basePolicy())
	require.NoError(b, err)

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

	policy, err := LoadPolicy(basePolicy())
	require.NoError(b, err)

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

	policy, err := LoadPolicy(basePolicy())
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s in %s", test.Src, test.Dest), func(b *testing.B) {
			for b.Loop() {
				policy.InGroup(test.Src, test.Dest)
			}
		})
	}
}

func BenchmarkListGroups(b *testing.B) {
	policy, err := LoadPolicy(basePolicy())
	require.NoError(b, err)

	for b.Loop() {
		policy.ListGroups("__group__")
	}
}
