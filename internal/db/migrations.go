// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package db

import (
	"io"
	"io/fs"
	"math/rand/v2"
	"path"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/db/migrations"
)

type migrationFunc func(*goqu.TxDatabase, fs.FS) error

type migrationEntry struct {
	id       int
	name     string
	funcList []migrationFunc
}

// newMigrationEntry creates a new migration which contains an id, a name and a list
// of functions performing the migration.
func newMigrationEntry(id int, name string, funcList ...migrationFunc) migrationEntry {
	res := migrationEntry{
		id:       id,
		name:     name,
		funcList: []migrationFunc{},
	}
	res.funcList = funcList
	return res
}

func applyMigrationFile(name string) func(td *goqu.TxDatabase, _ fs.FS) (err error) {
	return func(td *goqu.TxDatabase, _ fs.FS) (err error) {
		var fd fs.File
		if fd, err = migrations.Files.Open(path.Join(td.Dialect(), name)); err != nil {
			return
		}

		var sql []byte
		if sql, err = io.ReadAll(fd); err != nil {
			return
		}

		_, err = td.Exec(string(sql))
		return
	}
}

// migrationList is our full migration list.
var migrationList = []migrationEntry{
	newMigrationEntry(1, "user_seed", func(td *goqu.TxDatabase, _ fs.FS) (err error) {
		// Add a seed column to the user table
		sql := `ALTER TABLE "user" ADD COLUMN seed INTEGER NOT NULL DEFAULT 0;`

		if _, err = td.Exec(sql); err != nil {
			return
		}

		// Set a new seed on every user
		var ids []int64
		if err = td.From("user").Select("id").ScanVals(&ids); err != nil {
			return
		}
		for _, id := range ids {
			seed := rand.IntN(32767) //nolint:gosec
			_, err = td.Update("user").
				Set(goqu.Record{"seed": seed}).
				Where(goqu.C("id").Eq(id)).
				Executor().Exec()
			if err != nil {
				return
			}
		}

		return
	}),

	newMigrationEntry(2, "bookmark_collection", applyMigrationFile("02_bookmark_collection.sql")),
	newMigrationEntry(3, "bookmark_annotations", applyMigrationFile("03_bookmark_annotations.sql")),
	newMigrationEntry(4, "bookmark_links", applyMigrationFile("04_bookmark_links.sql")),
	newMigrationEntry(5, "bookmark_dates_idx", applyMigrationFile("05_bookmark_dates_idx.sql")),
	newMigrationEntry(6, "credential", applyMigrationFile("06_credential.sql")),
	newMigrationEntry(7, "bookmark_html_ids", migrations.M07migrateBookmarkIDs),
	newMigrationEntry(8, "bookmark_duration", applyMigrationFile("08_bookmark_duration.sql")),
	newMigrationEntry(9, "bookmark_dir", applyMigrationFile("09_bookmark_dir.sql")),
	newMigrationEntry(10, "bookmark_fts", applyMigrationFile("10_bookmark_fts.sql")),
	newMigrationEntry(11, "bookmark_sort_labels", migrations.M11sortLabels),
	newMigrationEntry(12, "bookmark_initial_url", applyMigrationFile("12_bookmark_initial_url.sql")),
	newMigrationEntry(13, "bookmark_progress", applyMigrationFile("13_bookmark_progress.sql")),
	newMigrationEntry(14, "collection_bookmark_type", migrations.M14collectionBookmarkType),
	newMigrationEntry(15, "sqlite_dates", migrations.M15sqliteDates),
	newMigrationEntry(16, "uuid_fields", migrations.M16uuidFields),
	newMigrationEntry(17, "user_uid", migrations.M17useruid),
	newMigrationEntry(18, "auth_last_used", applyMigrationFile("18_auth_last_used.sql")),
	newMigrationEntry(19, "bookmark_text_normalization", migrations.M19bookmarkTextNormalization),
	newMigrationEntry(20, "bookmark_removed", applyMigrationFile("20_bookmark_removed.sql")),
	newMigrationEntry(21, "token_roles", migrations.M21tokenRoles),
	newMigrationEntry(22, "oauth2", applyMigrationFile("22_oauth2.sql")),
}
