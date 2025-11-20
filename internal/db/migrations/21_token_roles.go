// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package migrations

import (
	"io/fs"
	"slices"

	"codeberg.org/readeck/readeck/internal/db/types"
	"github.com/doug-martin/goqu/v9"
)

// M21tokenRoles renames roles in token table.
func M21tokenRoles(db *goqu.TxDatabase, _ fs.FS) error {
	roleMap := map[string]string{
		"scoped_bookmarks_r": "bookmarks:read",
		"scoped_bookmarks_w": "bookmarks:write",
		"scoped_admin_r":     "admin:read",
		"scoped_admin_w":     "admin:write",
	}

	ds := db.Select(
		goqu.C("id"), goqu.C("roles"),
	).From("token")

	tokenList := []struct {
		ID    int           `db:"id"`
		Roles types.Strings `db:"roles"`
	}{}

	if err := ds.ScanStructs(&tokenList); err != nil {
		return err
	}

	for _, x := range tokenList {
		if len(x.Roles) == 0 {
			continue
		}
		roles := types.Strings{}
		for k, v := range roleMap {
			if slices.ContainsFunc(x.Roles, func(s string) bool {
				return s == k || s == v
			}) {
				roles = append(roles, v)
			}
		}

		slices.Sort(roles)

		_, err := db.Update("token").Prepared(true).
			Set(goqu.Record{"roles": roles}).
			Where(goqu.C("id").Eq(x.ID)).
			Executor().Exec()
		if err != nil {
			return err
		}
	}

	return nil
}
