// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package portability handles data export and import.
package portability

import (
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/db"
)

// Translator describes a type that implements a translation method.
type translator interface {
	Gettext(string, ...interface{}) string
}

type exportInfo struct {
	Date           time.Time
	Version        string
	ReadeckVersion string
}

type portableData struct {
	Info                exportInfo
	Users               []*users.User
	Tokens              []*tokens.Token
	BookmarkCollections []*bookmarks.Collection
	Bookmarks           []bookmarkItem
}

type exportManifest struct {
	Date  time.Time
	Files map[string]string
}

type bookmarkItem struct {
	UID    string `db:"uid"`
	UserID int    `db:"user_id"`
}

func ptrTo[T any](v T) *T {
	return &v
}

func marshalItems[T any](ds *goqu.SelectDataset) ([]T, error) {
	var items []T
	err := ds.ScanStructs(&items)
	return items, err
}

func insertInto[T any](tx *goqu.TxDatabase, table string, item T, prep func(T)) (int, error) {
	prep(item)
	ds := tx.Insert(table).Rows(item).Prepared(true)
	return db.InsertWithID(ds, "id")
}
