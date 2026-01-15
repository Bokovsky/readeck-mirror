// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// To compile with CGO_ENABLED=1
//go:build cgo && !nosqlite

package db

import (
	"database/sql"
	"log/slog"
	"net/url"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3" // dialect
	"github.com/mattn/go-sqlite3"                      // driver
)

func init() {
	drivers["sqlite3"] = &sqliteConnector{}

	sql.Register("sqlite3_extended", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.RegisterCollation("UNICODE", UnaccentCompare)
		},
	})
}

type sqliteConnector struct{}

func (c *sqliteConnector) Name() string {
	return "mattn/go-sqlite3"
}

func (c *sqliteConnector) Dialect() string {
	return "sqlite3"
}

func (c *sqliteConnector) Open(dsn *url.URL) (*sql.DB, error) {
	// Prepare URI
	uri, err := getSqliteDsn(dsn)
	if err != nil {
		return nil, err
	}
	query := uri.Query()

	if uri.Scheme == "memory" {
		// In-memory database (for tests)
		uri.Scheme = ""
		uri.Opaque = ":memory:"
		uri.Path = ""
	}

	query.Set("_txtlock", "immediate")
	uri.RawQuery = query.Encode()

	slog.Debug("connect to database",
		slog.String("driver", c.Name()),
		slog.String("dsn", uri.Redacted()),
	)
	db, err := sql.Open("sqlite3_extended", uri.String())
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		PRAGMA busy_timeout = 5000;
		PRAGMA foreign_keys = 1;
		PRAGMA journal_mode = WAL;
		PRAGMA mmap_size = 30000000000;
		PRAGMA cache_size = 1000000000;
		PRAGMA secure_delete = 1;
		PRAGMA synchronous = NORMAL;
		PRAGMA temp_store = memory;
	`)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(2)
	return db, nil
}

func (c *sqliteConnector) Version() string {
	var res string
	if _, err := Q().Select(goqu.Func("sqlite_version")).ScanVal(&res); err != nil {
		panic(err)
	}
	return res
}

func (c *sqliteConnector) DiskUsage() (uint64, error) {
	var sizeBytes uint64
	if _, err := Q().Select(
		goqu.L("page_count * page_size")).
		From(goqu.L("pragma_page_count()")).
		CrossJoin(goqu.L("pragma_page_size()")).
		ScanVal(&sizeBytes); err != nil {
		panic(err)
	}
	return sizeBytes, nil
}

func (c *sqliteConnector) HasTable(name string) (bool, error) {
	ds := Q().Select(goqu.C("name")).
		From(goqu.T("sqlite_master")).
		Where(
			goqu.C("type").Eq("table"),
			goqu.C("name").Eq(name),
		)
	var res string

	if _, err := ds.ScanVal(&res); err != nil {
		return false, err
	}

	return res == name, nil
}
