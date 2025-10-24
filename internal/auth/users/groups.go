// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package users

import "codeberg.org/readeck/readeck/internal/acls"

type translator interface {
	Pgettext(ctx, str string, vars ...any) string
}

func pgettext(_, s string) string {
	// dummy pgettext for string extraction
	return s
}

var roleMap = map[string]string{
	"user":  pgettext("role", "user"),
	"staff": pgettext("role", "staff"),
	"admin": pgettext("role", "admin"),

	"profile:read":    pgettext("role", "Profile : Read Only"),
	"bookmarks:read":  pgettext("role", "Bookmarks : Read Only"),
	"bookmarks:write": pgettext("role", "Bookmarks : Write Only"),
	"admin:read":      pgettext("role", "Admin : Read Only"),
	"admin:write":     pgettext("role", "Admin : Write Only"),
}

// GroupList returns a list of available groups identified by a permission name
// and a [User]. When the user is nil, returns all the available groups.
func GroupList(tr translator, name string, user *User) [][2]string {
	res := [][2]string{}
	groups := acls.ListGroups(name)
	for _, g := range groups {
		if user != nil && !acls.InGroup(g, user.Group) {
			continue
		}

		label := g
		if n, ok := roleMap[g]; ok {
			label = tr.Pgettext("role", n)
		}

		res = append(res, [2]string{g, label})
	}

	return res
}

// GroupNames converts a role list to a list of translated names.
func GroupNames(tr translator, groups []string) []string {
	res := make([]string, len(groups))

	for i, g := range groups {
		res[i] = g
		if n, ok := roleMap[g]; ok {
			res[i] = tr.Pgettext("role", n)
		}
	}

	return res
}
