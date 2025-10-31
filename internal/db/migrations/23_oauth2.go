// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package migrations

import (
	"io/fs"

	"github.com/doug-martin/goqu/v9"
)

// M23oauth adds a client_info column in the token table.
// It also takes care of removing the never released (but in nightly builds)
// oauth2_client table and the token.client_id column.
func M23oauth(db *goqu.TxDatabase, _ fs.FS) error {
	var err error

	// What should be the script without nightly clean-up
	switch db.Dialect() {
	case "sqlite3":
		_, err = db.Exec(`ALTER TABLE token ADD COLUMN client_info json NULL`)
	case "postgres":
		_, err = db.Exec(`ALTER TABLE token ADD COLUMN client_info jsonb NULL`)
	}
	if err != nil {
		return err
	}

	// Remove unreleased token.client_id column
	var found bool

	switch db.Dialect() {
	case "sqlite3":
		var cname string
		found, err = db.ScanVal(&cname, `SELECT name FROM pragma_table_info('token') WHERE name = 'client_id'`)
		if err != nil {
			return err
		}
	case "postgres":
		var cname string
		found, err = db.ScanVal(&cname, `SELECT column_name FROM information_schema.columns WHERE table_name = 'token' AND column_name = 'client_id' `)
		if err != nil {
			return err
		}
	}
	if found {
		if _, err = db.Exec(`DELETE FROM token WHERE client_id IS NOT NULL`); err != nil {
			return err
		}
		if _, err = db.Exec(`ALTER TABLE token DROP COLUMN client_id`); err != nil {
			return err
		}
	}

	// Clean up unreleased oauth2_client table
	_, err = db.Exec(`DROP TABLE IF EXISTS oauth2_client`)
	return err
}
