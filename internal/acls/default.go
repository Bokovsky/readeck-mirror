// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package acls

// This is Readeck default permission policy.

var defaultPolicy *Policy

var defaultPermissions = Permissions{
	// System routes
	"/system/read": {"system", "read"},

	// Email sending
	"/email/send": {"email", "send"},

	// Admin
	"/api/admin/read":  {"api:admin:users", "read"},
	"/api/admin/write": {"api:admin:users", "write"},
	"/web/admin/read":  {"admin:users", "read"},
	"/web/admin/write": {"admin:users", "write"},

	// Cookbook
	"/api/cookbook/read": {"api:cookbook", "read"},
	"/web/cookbook/read": {"cookbook", "read"},

	// Documentation
	"/web/docs/read": {"docs", "read"},

	// User profile
	"/api/profile/info":   {"api:profile", "info"},
	"/api/profile/read":   {"api:profile", "read"},
	"/api/profile/write":  {"api:profile", "write"},
	"/web/profile/read":   {"profile", "read"},
	"/web/profile/write":  {"profile", "write"},
	"/web/profile/export": {"profile", "export"},
	"/web/profile/import": {"profile", "import"},

	// API Tokens
	"/api/profile/tokens/read":   {"api:profile:tokens", "read"},
	"/api/profile/tokens/delete": {"api:profile:tokens", "delete"},
	"/web/profile/tokens/read":   {"profile:tokens", "read"},
	"/web/profile/tokens/write":  {"profile:tokens", "write"},

	// Bookmarks
	"/api/bookmarks/read":   {"api:bookmarks", "read"},
	"/api/bookmarks/write":  {"api:bookmarks", "write"},
	"/api/bookmarks/export": {"api:bookmarks", "export"},
	"/api/bookmarks/import": {"api:bookmarks", "import"},
	"/api/bookmarks/share":  {"api:bookmarks", "share"},
	"/web/bookmarks/read":   {"bookmarks", "read"},
	"/web/bookmarks/write":  {"bookmarks", "write"},
	"/web/bookmarks/export": {"bookmarks", "export"},
	"/web/bookmarks/import": {"bookmarks", "import"},
	"/web/bookmarks/share":  {"bookmarks", "share"},

	// Bookmark collections
	"/api/bookmarks/collections/read":  {"api:bookmarks:collections", "read"},
	"/api/bookmarks/collections/write": {"api:bookmarks:collections", "write"},
	"/web/bookmarks/collections/read":  {"bookmarks:collections", "read"},
	"/web/bookmarks/collections/write": {"bookmarks:collections", "write"},

	// OPDS catalog
	"/api/opds/read": {"api:opds", "read"},
}

var defaultGroups = []Group{
	{
		// Empty group, for unauthenticated users
		Grants: []string{
			"/email/send",
		},
	},
	{
		// These are needed for any groups or scope
		Name: "api_common",
		Grants: []string{
			"/api/profile/info",
			"/api/profile/tokens/delete",
		},
	},
	{
		// Group "user"
		Name: "user",
		Parents: []string{
			"@group",
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
			"/*/bookmarks/import",
			"/*/bookmarks/share",
			"/*/bookmarks/collections/read",
			"/*/bookmarks/collections/write",
			"/api/opds/*",
		},
	},
	{
		// Group "staff"
		Name: "staff",
		Parents: []string{
			"@group",
			"user",
		},
		Grants: []string{
			"/system/*",
		},
	},
	{
		// Group "admin"
		Name: "admin",
		Parents: []string{
			"@group",
			"staff",
		},
		Grants: []string{
			"/*/admin/*",
			"/*/cookbook/*",
		},
	},

	// Scoped groups, used by API tokens
	{
		// Profile
		Name: "profile:read",
		Parents: []string{
			"@token_scope",
			"@oauth_scope",
			"api_common",
		},
		Grants: []string{
			"/api/profile/read",
		},
	},
	{
		// Bookmarks read only
		Name: "bookmarks:read",
		Parents: []string{
			"@token_scope",
			"@oauth_scope",
			"api_common",
		},
		Grants: []string{
			"/email/send",
			"/api/bookmarks/read",
			"/api/bookmarks/share",
			"/api/bookmarks/export",
			"/api/bookmarks/collections/read",
			"/api/opds/read",
			"/web/bookmarks/read",
		},
	},
	{
		// Bookmarks write only
		Name: "bookmarks:write",
		Parents: []string{
			"@token_scope",
			"@oauth_scope",
			"api_common",
		},
		Grants: []string{
			"/api/bookmarks/write",
			"/api/bookmarks/collections/write",
		},
	},
	{
		// Admin read only
		Name: "admin:read",
		Parents: []string{
			"@token_scope",
			"api_common",
		},
		Grants: []string{
			"/api/admin/read",
			"/system/read",
		},
	},
	{
		// Admin write only
		Name: "admin:write",
		Parents: []string{
			"@token_scope",
			"api_common",
		},
		Grants: []string{
			"/api/admin/write",
		},
	},
}
