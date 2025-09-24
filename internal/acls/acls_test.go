// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package acls_test

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"codeberg.org/readeck/readeck/internal/acls"
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
g, scoped_bookmarks_r, api_common
g, scoped_bookmarks_r, /api/bookmarks/read
g, scoped_bookmarks_r, /api/bookmarks/export
g, scoped_bookmarks_r, /api/bookmarks/collections/read
g, scoped_bookmarks_r, /api/opds/read
g, scoped_bookmarks_r, /web/bookmarks/read
g, scoped_bookmarks_w, api_common
g, scoped_bookmarks_w, /api/bookmarks/write
g, scoped_bookmarks_w, /api/bookmarks/collections/write
g, scoped_admin_r, api_common
g, scoped_admin_r, /api/admin/read
g, scoped_admin_r, /system/read
g, scoped_admin_w, api_common
g, scoped_admin_w, /api/admin/write
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

		{"admin", "email", "send", true},
		{"staff", "email", "send", true},
		{"user", "email", "send", true},
		{"", "email", "send", true},
	}

	enforcer, err := acls.NewEnforcer(basePolicy())
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s-%s-%s", test.Group, test.Obj, test.Act), func(t *testing.T) {
			res, err := enforcer.Enforce(test.Group, test.Obj, test.Act)
			require.NoError(t, err)
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
			[]string{"scoped_admin_r"},
			[]string{"api:admin:users:read", "api:profile:read", "api:profile:tokens:delete", "system:read"},
		},
		{
			[]string{"scoped_admin_w"},
			[]string{"api:admin:users:write", "api:profile:read", "api:profile:tokens:delete"},
		},
		{
			[]string{"scoped_admin_r", "scoped_admin_w"},
			[]string{"api:admin:users:read", "api:admin:users:write", "api:profile:read", "api:profile:tokens:delete", "system:read"},
		},
		{
			[]string{"scoped_bookmarks_r"},
			[]string{"api:bookmarks:collections:read", "api:bookmarks:export", "api:bookmarks:read", "api:opds:read", "api:profile:read", "api:profile:tokens:delete", "bookmarks:read"},
		},
		{
			[]string{"scoped_bookmarks_w"},
			[]string{"api:bookmarks:collections:write", "api:bookmarks:write", "api:profile:read", "api:profile:tokens:delete"},
		},
		{
			[]string{"unknown"},
			[]string{},
		},
	}

	enforcer, err := acls.NewEnforcer(basePolicy())
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(strings.Join(test.Groups, ","), func(t *testing.T) {
			res, err := enforcer.GetPermissions(test.Groups...)
			require.NoError(t, err)
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
		{"scoped_bookmarks_r", "user", true},
		{"scoped_admin_r", "user", false},
		{"scoped_admin_r", "admin", true},
	}

	enforcer, err := acls.NewEnforcer(basePolicy())
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s in %s", test.Src, test.Dest), func(t *testing.T) {
			res := enforcer.InGroup(test.Src, test.Dest)
			require.Equal(t, test.Expected, res)
		})
	}
}

func TestListGroups(t *testing.T) {
	enforcer, err := acls.NewEnforcer(basePolicy())
	require.NoError(t, err)

	groups, err := enforcer.ListGroups("__group__")
	require.NoError(t, err)

	require.Equal(t, []string{"user", "staff", "admin"}, groups)
}

func BenchmarkGetPermissions(b *testing.B) {
	enforcer, err := acls.NewEnforcer(basePolicy())
	require.NoError(b, err)

	for b.Loop() {
		_, err := enforcer.GetPermissions("user")
		require.NoError(b, err)
	}
}

func BenchmarkInGroup(b *testing.B) {
	enforcer, err := acls.NewEnforcer(basePolicy())
	require.NoError(b, err)

	for b.Loop() {
		enforcer.InGroup("scoped_bookmarks_r", "user")
	}
}

func BenchmarkListGroup(b *testing.B) {
	enforcer, err := acls.NewEnforcer(basePolicy())
	require.NoError(b, err)

	for b.Loop() {
		_, err := enforcer.ListGroups("__group__")
		require.NoError(b, err)
	}
}
