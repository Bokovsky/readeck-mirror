// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package migrations

import (
	"io/fs"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/pkg/base58"
)

// M24useruidFix adds the potentially missing user UID fields
// for users created with a bugged "user" command.
func M24useruidFix(db *goqu.TxDatabase, _ fs.FS) error {
	ds, err := db.Select(goqu.C("id")).
		From("user").
		Where(goqu.C("uid").Eq("")).
		Executor().Query()
	if err != nil {
		return err
	}

	for ds.Next() {
		var id int
		if err := ds.Scan(&id); err != nil {
			return err
		}

		if _, err = db.Update("user").Set(goqu.Record{
			"uid": base58.NewUUID(),
		}).Where(goqu.C("id").Eq(id)).Executor().Exec(); err != nil {
			return err
		}
	}

	return nil
}
