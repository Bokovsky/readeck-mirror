// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package portability

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

var (
	_ Exporter = (*FullExporter)(nil)
	_ Exporter = (*SingleUserExporter)(nil)
)

// Exporter describes a data exporter.
type Exporter interface {
	Log(format string, a ...any)

	getUsers() ([]*users.User, error)
	getTokens() ([]*tokens.Token, error)
	getCollections() ([]*bookmarks.Collection, error)
	getBookmarks() ([]bookmarkItem, error)
	saveBookmark(item bookmarkItem) error
	saveData(*portableData) error
}

// Export export data using the [Exporter].
func Export(ex Exporter) error {
	var err error

	data := &portableData{
		Info: exportInfo{
			Date:           time.Now(),
			Version:        "1",
			ReadeckVersion: configs.Version(),
		},
	}

	if data.Users, err = ex.getUsers(); err != nil {
		return err
	}
	ex.Log("%d user(s) exported", len(data.Users))

	if data.Tokens, err = ex.getTokens(); err != nil {
		return err
	}
	ex.Log("%d tokens(s) exported", len(data.Tokens))

	if data.BookmarkCollections, err = ex.getCollections(); err != nil {
		return err
	}
	ex.Log("%d collection(s) exported", len(data.BookmarkCollections))

	if data.Bookmarks, err = ex.getBookmarks(); err != nil {
		return err
	}

	// Save each bookmark now
	for _, item := range data.Bookmarks {
		if err = ex.saveBookmark(item); err != nil {
			return err
		}
	}
	ex.Log("%d bookmark(s) exported", len(data.Bookmarks))

	// Save data.json
	return ex.saveData(data)
}

// FullExporter is an [Exporter] than exports data to a zip file.
// It receives a list of usernames (or an empty list for all users)
// and exports all their related data.
type FullExporter struct {
	userIDs  []int
	zfs      *zipfs.ZipRW
	logFn    func(string, ...any)
	manifest exportManifest
}

// SingleUserExporter is an [Exporter] that exports only one user.
// The resulting file is the same as [FullExporter].
type SingleUserExporter struct {
	*FullExporter
}

// NewFullExporter returns a [FullExporter] instance.
func NewFullExporter(w io.Writer, usernames []string) (*FullExporter, error) {
	var userIDs []int
	ds := users.Users.Query().Select(goqu.C("id"))

	if len(usernames) > 0 {
		ds = ds.Where(goqu.C("username").In(usernames))
	}
	if err := ds.ScanVals(&userIDs); err != nil {
		return nil, err
	}

	if len(userIDs) == 0 {
		return nil, errors.New("no user to export")
	}

	ex := &FullExporter{
		userIDs: userIDs,
		zfs:     zipfs.NewZipRW(w, nil, 0),
		manifest: exportManifest{
			Date:  time.Now(),
			Files: make(map[string]string),
		},
		logFn: func(_ string, _ ...any) {},
	}

	return ex, nil
}

// Log implements [Exporter].
func (ex *FullExporter) Log(format string, a ...any) {
	ex.logFn(format, a...)
}

// Close flushes and closes the underlying zipfile.
func (ex *FullExporter) Close() error {
	return ex.zfs.Close()
}

// SetLogger sets a logging function.
func (ex *FullExporter) SetLogger(fn func(string, ...any)) {
	ex.logFn = fn
}

func (ex *FullExporter) getUsers() ([]*users.User, error) {
	return marshalItems[*users.User](
		users.Users.Query().
			SelectAppend(goqu.V("0").As("seed")).
			Where(goqu.C("id").In(ex.userIDs)).
			Order(goqu.C("username").Asc()),
	)
}

func (ex *FullExporter) getTokens() ([]*tokens.Token, error) {
	return marshalItems[*tokens.Token](
		tokens.Tokens.Query().
			Where(goqu.C("user_id").In(ex.userIDs)).
			Order(goqu.C("created").Asc()),
	)
}

func (ex *FullExporter) getCollections() ([]*bookmarks.Collection, error) {
	return marshalItems[*bookmarks.Collection](
		bookmarks.Collections.Query().
			Where(goqu.C("user_id").In(ex.userIDs)).
			Order(goqu.C("created").Asc()),
	)
}

func (ex *FullExporter) getBookmarks() ([]bookmarkItem, error) {
	return marshalItems[bookmarkItem](
		bookmarks.Bookmarks.Query().
			Select("uid", "user_id").
			Where(goqu.C("user_id").In(ex.userIDs)).
			Order(goqu.C("created").Asc()),
	)
}

func (ex *FullExporter) saveBookmark(item bookmarkItem) error {
	b, err := bookmarks.Bookmarks.GetOne(goqu.C("uid").Eq(item.UID))
	if err != nil {
		return err
	}

	dest := path.Join("bookmarks", b.UID, "info.json")
	w, err := ex.zfs.GetWriter(&zip.FileHeader{
		Name:   dest,
		Method: zip.Deflate,
	})
	if err != nil {
		return err
	}

	enc := json.NewEncoder(w)
	if err = enc.Encode(b); err != nil {
		return err
	}

	c, err := b.OpenContainer()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, x := range c.File {
		if x.FileInfo().IsDir() {
			continue
		}

		h := x.FileHeader
		h.Name = path.Join("bookmarks", b.UID, "container", h.Name)

		r, err := x.OpenRaw()
		if err != nil {
			return err
		}

		w, err := ex.zfs.GetRawWriter(&h)
		if err != nil {
			return err
		}

		if _, err = io.Copy(w, r); err != nil {
			return err
		}
	}

	return nil
}

func (ex *FullExporter) saveData(data *portableData) error {
	w, err := ex.zfs.GetWriter(&zip.FileHeader{Name: "data.json", Method: zip.Deflate})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// NewSingleUserExporter returns a [SingleUserExporter] instance.
func NewSingleUserExporter(w io.Writer, user *users.User) (*SingleUserExporter, error) {
	return &SingleUserExporter{
		&FullExporter{
			userIDs: []int{user.ID},
			zfs:     zipfs.NewZipRW(w, nil, 0),
			manifest: exportManifest{
				Date:  time.Now(),
				Files: make(map[string]string),
			},
		},
	}, nil
}

func (ex *SingleUserExporter) getUsers() ([]*users.User, error) {
	return marshalItems[*users.User](
		users.Users.Query().
			SelectAppend(
				goqu.V("0").As("seed"),
				goqu.V("").As("password"),
			).
			Where(goqu.C("id").In(ex.userIDs)).
			Order(goqu.C("username").Asc()),
	)
}
