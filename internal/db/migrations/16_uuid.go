// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package migrations

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/base58"
)

type m16BbookmarkAnnotations []*m16BookmarkAnnotation

// m16BookmarkAnnotation is an annotation that can be serialized in a database JSON column.
type m16BookmarkAnnotation struct {
	ID            string    `json:"id"`
	StartSelector string    `json:"start_selector"`
	StartOffset   int       `json:"start_offset"`
	EndSelector   string    `json:"end_selector"`
	EndOffset     int       `json:"end_offset"`
	Color         string    `json:"color"`
	Created       time.Time `json:"created"`
	Text          string    `json:"text"`
}

// Scan loads a BookmarkAnnotations instance from a column.
func (a *m16BbookmarkAnnotations) Scan(value any) error {
	if value == nil {
		return nil
	}

	v, err := types.JSONBytes(value)
	if err != nil {
		return err
	}
	json.Unmarshal(v, a) //nolint:errcheck
	return nil
}

// Value encodes a BookmarkAnnotations instance for storage.
func (a m16BbookmarkAnnotations) Value() (driver.Value, error) {
	for _, x := range a {
		if x.Color == "" {
			x.Color = "yellow"
		}
	}

	v, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(v), nil
}

// M16uuidFields performs a migration that updates all short UUIDs to
// base-58 encoded ones. It then renames all files in the data/bookmarks folder
// so they match the new IDs.
func M16uuidFields(db *goqu.TxDatabase, _ fs.FS) error {
	type postUpdate func(table string, id int) error
	var updateTable func(table string, column string, fn postUpdate) error

	renameMap := [][2]string{}

	type rowUpdate struct {
		id  int
		uid string
	}

	type bookmarkFile struct {
		ID          int                     `db:"id"`
		UID         string                  `db:"uid"`
		FilePath    string                  `db:"file_path"`
		Annotations m16BbookmarkAnnotations `db:"annotations"`
	}

	renameBookmarkFile := func(table string, id int) error {
		var bf bookmarkFile

		if _, err := db.Select().From(table).Where(goqu.C("id").Eq(id)).ScanStruct(&bf); err != nil {
			return err
		}

		// Update annotations
		record := map[string]any{}
		if len(bf.Annotations) > 0 {
			for i := range bf.Annotations {
				bf.Annotations[i].ID = base58.NewUUID()
			}
			record["annotations"] = bf.Annotations
		}

		// Update file_path
		newPath := path.Join(bf.UID[0:2], bf.UID)
		record["file_path"] = newPath

		if _, err := db.Update(table).
			Set(record).
			Where(goqu.C("id").Eq(bf.ID)).
			Executor().Exec(); err != nil {
			return err
		}

		renameMap = append(renameMap, [2]string{bf.FilePath, newPath})

		return nil
	}

	updateTable = func(table, column string, fn postUpdate) error {
		ds, err := db.Select(goqu.C("id"), goqu.C(column)).From(table).Executor().Query()
		if err != nil {
			return err
		}

		updates := []rowUpdate{}

		for ds.Next() {
			var id int
			var uid string
			if err := ds.Scan(&id, &uid); err != nil {
				return err
			}
			newUID := base58.NewUUID()
			updates = append(updates, rowUpdate{
				id:  id,
				uid: newUID,
			})
		}

		for _, x := range updates {
			if _, err = db.Update(table).Set(goqu.Record{
				column: x.uid,
			}).Where(
				goqu.C("id").Eq(x.id),
			).Executor().Exec(); err != nil {
				return err
			}
			if fn != nil {
				if err = fn(table, x.id); err != nil {
					return err
				}
			}
		}

		return nil
	}

	tables := []struct {
		name   string
		column string
		fn     postUpdate
	}{
		{"token", "uid", nil},
		{"credential", "uid", nil},
		{"bookmark_collection", "uid", nil},
		{"bookmark", "uid", renameBookmarkFile},
	}

	for _, t := range tables {
		if err := updateTable(t.name, t.column, t.fn); err != nil {
			return err
		}
	}

	// Database operations are done, we can now proceed with renaming files.
	curroot := filepath.Join(configs.Config.Main.DataDirectory, "bookmarks")
	newroot := filepath.Join(configs.Config.Main.DataDirectory, "bookmarks-new")

	// Create a new bookmark folder
	if err := os.Mkdir(newroot, 0o750); err != nil {
		return err
	}

	// We hardlink new bookmark files into a new folder.
	// If something goes wrong, the new folder is removed and
	// the DB transaction is rolledback.
	if err := func() error {
		for _, x := range renameMap {
			oldpath, newpath := x[0], x[1]
			newpath = filepath.Join(newroot, newpath) + ".zip"
			oldpath = filepath.Join(curroot, oldpath) + ".zip"

			slog.Debug("link file",
				slog.String("old", oldpath),
				slog.String("new", newpath),
			)

			_, err := os.Stat(oldpath)
			if errors.Is(err, os.ErrNotExist) {
				slog.Warn("file does not exist", slog.String("path", oldpath))
				continue
			}

			if err = os.MkdirAll(filepath.Dir(newpath), 0o750); err != nil {
				return err
			}

			// Hardlink new file
			if err := os.Link(oldpath, newpath); err != nil {
				if !errors.Is(err, os.ErrExist) {
					return err
				}
			}
		}
		return nil
	}(); err != nil {
		if err := os.RemoveAll(newroot); err != nil {
			slog.Error("removing new folder", slog.String("path", newroot))
		}
		return err
	}

	// Remove old bookmark folder and rename the new one
	if err := os.RemoveAll(curroot); err != nil {
		return err
	}
	return os.Rename(newroot, curroot)
}
