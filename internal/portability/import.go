// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package portability

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

var (
	_ Importer = (*FullImporter)(nil)
	_ Importer = (*SingleUserImporter)(nil)
)

// Importer describes a data loader.
type Importer interface {
	Log(format string, a ...any)

	loadData() (*portableData, error)
	clearDB(*goqu.TxDatabase) error
	loadUsers(*goqu.TxDatabase, *portableData) error
	loadTokens(*goqu.TxDatabase, *portableData) error
	loadCollections(*goqu.TxDatabase, *portableData) error
	loadBookmarks(*goqu.TxDatabase, *portableData) error
}

// Import loads all the data into Readeck's database and content folder.
func Import(imp Importer) error {
	data, err := imp.loadData()
	if err != nil {
		return err
	}

	fnList := []func(*goqu.TxDatabase, *portableData) error{
		imp.loadUsers,
		imp.loadTokens,
		imp.loadCollections,
		imp.loadBookmarks,
	}

	tx, err := db.Q().Begin()
	if err != nil {
		return err
	}
	return tx.Wrap(func() error {
		if err = imp.clearDB(tx); err != nil {
			return err
		}

		for _, fn := range fnList {
			if err = fn(tx, data); err != nil {
				return err
			}
		}
		return nil
	})
}

// FullImporter is a content importer.
type FullImporter struct {
	usernames []string
	users     map[int]int
	clearData bool
	zr        *zip.Reader
	tr        translator
	logFn     func(string, ...any)
}

// SingleUserImporter is an importer that loads only one user.
type SingleUserImporter struct {
	*FullImporter
	user *users.User
}

// NewFullImporter creates a new [FullImporter].
func NewFullImporter(zr *zip.Reader, usernames []string, clearData bool, tr translator) *FullImporter {
	return &FullImporter{
		zr:        zr,
		usernames: usernames,
		users:     map[int]int{},
		clearData: clearData,
		tr:        tr,
	}
}

// Log implements [Importer].
func (imp *FullImporter) Log(format string, a ...any) {
	imp.logFn(format, a...)
}

// SetLogger sets a logging function.
func (imp *FullImporter) SetLogger(fn func(string, ...any)) {
	imp.logFn = fn
}

func (imp *FullImporter) loadData() (*portableData, error) {
	fd, err := imp.zr.Open("data.json")
	if err != nil {
		return nil, err
	}
	defer fd.Close() //nolint:errcheck

	var data portableData
	dec := json.NewDecoder(fd)
	if err = dec.Decode(&data); err != nil {
		return nil, err
	}

	return &data, nil
}

func (imp *FullImporter) clearDB(tx *goqu.TxDatabase) error {
	if !imp.clearData {
		return nil
	}

	if _, err := tx.Delete(bookmarks.TableName).Executor().Exec(); err != nil {
		return err
	}

	if _, err := tx.Delete(users.TableName).Executor().Exec(); err != nil {
		return err
	}
	return nil
}

func (imp *FullImporter) loadUsers(tx *goqu.TxDatabase, data *portableData) (err error) {
	allUsers := len(imp.usernames) == 0

	for _, item := range data.Users {
		if allUsers {
			imp.usernames = append(imp.usernames, item.Username)
		}

		if !slices.Contains(imp.usernames, item.Username) {
			continue
		}

		var count int64
		count, err = tx.Select().From(users.TableName).Where(
			goqu.Or(
				goqu.C("username").Eq(item.Username),
				goqu.C("email").Eq(item.Email),
			),
		).Prepared(true).Count()
		if err != nil {
			return err
		}
		if count > 0 {
			imp.Log("ERR: user \"%s\" or \"%s\" already exists", item.Username, item.Email)
			continue
		}

		originalID := item.ID
		if item.ID, err = insertInto(tx, users.TableName, item, func(x *users.User) {
			x.ID = 0
			x.SetSeed()
			if !imp.clearData || x.UID == "" {
				x.UID = base58.NewUUID()
			}
		}); err != nil {
			return
		}
		imp.users[originalID] = item.ID
	}

	imp.Log("%d user(s) imported", len(imp.users))
	return
}

func (imp *FullImporter) loadTokens(tx *goqu.TxDatabase, data *portableData) (err error) {
	ids := slices.Collect(maps.Keys(imp.users))

	i := 0
	for _, item := range data.Tokens {
		if item.UserID == nil {
			continue
		}
		if !slices.Contains(ids, *item.UserID) {
			continue
		}

		if item.ID, err = insertInto(tx, tokens.TableName, item, func(x *tokens.Token) {
			x.ID = 0
			x.UserID = ptrTo(imp.users[*x.UserID])
			if !imp.clearData || x.UID == "" {
				x.UID = base58.NewUUID()
			}
		}); err != nil {
			return
		}
		i++
	}

	imp.Log("%d token(s) imported", i)
	return
}

func (imp *FullImporter) loadCollections(tx *goqu.TxDatabase, data *portableData) (err error) {
	ids := slices.Collect(maps.Keys(imp.users))

	i := 0
	for _, item := range data.BookmarkCollections {
		if item.UserID == nil {
			continue
		}
		if !slices.Contains(ids, *item.UserID) {
			continue
		}

		if item.ID, err = insertInto(tx, bookmarks.CollectionTable, item, func(x *bookmarks.Collection) {
			x.ID = 0
			x.UserID = ptrTo(imp.users[*x.UserID])
			if !imp.clearData || x.UID == "" {
				x.UID = base58.NewUUID()
			}
		}); err != nil {
			return
		}
		i++
	}

	imp.Log("%d collection(s) imported", i)
	return
}

func (imp *FullImporter) loadBookmarks(tx *goqu.TxDatabase, data *portableData) (err error) {
	ids := slices.Collect(maps.Keys(imp.users))

	i := 0
	for _, item := range data.Bookmarks {
		if !slices.Contains(ids, item.UserID) {
			continue
		}

		if err = imp.loadBookmark(tx, &item); err != nil {
			return
		}
		i++
	}

	imp.Log("%d bookmark(s) imported\n", i)
	return
}

func (imp *FullImporter) loadBookmark(tx *goqu.TxDatabase, item *bookmarkItem) (err error) {
	p := path.Join("bookmarks", item.UID, "info.json")
	fd, err := imp.zr.Open(p)
	if err != nil {
		return err
	}
	defer fd.Close() //nolint:errcheck

	var b bookmarks.Bookmark
	dec := json.NewDecoder(fd)
	if err = dec.Decode(&b); err != nil {
		return
	}

	if b.ID, err = insertInto(tx, bookmarks.TableName, &b, func(x *bookmarks.Bookmark) {
		x.ID = 0
		x.UserID = ptrTo(imp.users[*x.UserID])
		if !imp.clearData || x.UID == "" {
			x.UID = base58.NewUUID()
		}
		x.FilePath, _ = x.GetBaseFileURL()
	}); err != nil {
		return
	}

	// Copy files to zipfile
	dest := filepath.Join(bookmarks.StoragePath(), b.FilePath+".zip")
	if err = os.MkdirAll(path.Dir(dest), 0o750); err != nil {
		return err
	}
	w, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer w.Close() //nolint:errcheck

	zw := zipfs.NewZipRW(w, nil, 0)
	defer zw.Close() // nolint:errcheck

	prefix := "bookmarks/" + item.UID + "/container/"
	for _, f := range imp.zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}

		h := f.FileHeader
		h.Name = strings.TrimPrefix(h.Name, prefix)

		rr, err := f.OpenRaw()
		if err != nil {
			return err
		}

		rw, err := zw.GetRawWriter(&h)
		if err != nil {
			return err
		}

		if _, err = io.Copy(rw, rr); err != nil {
			return err
		}
	}

	return
}

// NewSingleUserImporter returns a [SingleUserImporter] instance.
func NewSingleUserImporter(zr *zip.Reader, user *users.User, tr translator) *SingleUserImporter {
	return &SingleUserImporter{
		FullImporter: NewFullImporter(zr, []string{user.Username}, false, tr),
		user:         user,
	}
}

func (imp *SingleUserImporter) loadData() (*portableData, error) {
	data, err := imp.FullImporter.loadData()
	if err != nil {
		return nil, err
	}

	if len(data.Users) != 1 {
		return nil, errors.New(imp.tr.Gettext("The import file must contain one user only"))
	}

	imp.usernames = []string{imp.user.Username}
	imp.users[data.Users[0].ID] = imp.user.ID
	return data, nil
}

func (imp *SingleUserImporter) clearDB(_ *goqu.TxDatabase) error {
	return nil
}

func (imp *SingleUserImporter) loadUsers(tx *goqu.TxDatabase, data *portableData) error {
	_, err := tx.Update(users.TableName).Prepared(true).
		Set(map[string]any{
			"updated":  time.Now().UTC(),
			"settings": data.Users[0].Settings,
		}).
		Where(goqu.C("id").Eq(imp.user.ID)).
		Executor().Exec()

	return err
}

func (imp *SingleUserImporter) loadTokens(tx *goqu.TxDatabase, data *portableData) error {
	if _, err := tx.Delete(tokens.TableName).Prepared(true).
		Where(goqu.C("user_id").Eq(imp.user.ID)).
		Executor().Exec(); err != nil {
		return err
	}
	return imp.FullImporter.loadTokens(tx, data)
}

func (imp *SingleUserImporter) loadCollections(tx *goqu.TxDatabase, data *portableData) error {
	if _, err := tx.Delete(bookmarks.CollectionTable).Prepared(true).
		Where(goqu.C("user_id").Eq(imp.user.ID)).
		Executor().Exec(); err != nil {
		return err
	}
	return imp.FullImporter.loadCollections(tx, data)
}

func (imp *SingleUserImporter) loadBookmarks(tx *goqu.TxDatabase, data *portableData) (err error) {
	ds := tx.From(bookmarks.TableName).Prepared(true).
		Select(goqu.C("id"), goqu.C("file_path")).
		Where(goqu.C("user_id").Eq(imp.user.ID))

	items := []*bookmarks.Bookmark{}
	if err := ds.ScanStructs(&items); err != nil {
		return err
	}

	if _, err := tx.Delete(bookmarks.TableName).Prepared(true).
		Where(goqu.C("user_id").Eq(imp.user.ID)).
		Executor().Exec(); err != nil {
		return err
	}

	if err := imp.FullImporter.loadBookmarks(tx, data); err != nil {
		return err
	}

	for _, b := range items {
		b.RemoveFiles()
	}

	return nil
}
