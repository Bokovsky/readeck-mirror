// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package users contains the models and functions to manage users.
package users

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/hlandau/passlib"

	"codeberg.org/readeck/readeck/internal/acls"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/base58"
)

func init() {
	if err := passlib.UseDefaults(passlib.Defaults20180601); err != nil {
		panic(err)
	}
}

const (
	// TableName is the user table name in database.
	TableName = "user"
)

var (
	// Users is the user manager.
	Users = Manager{}

	// ErrNotFound is returned when a user record was not found.
	ErrNotFound = errors.New("not found")

	availableGroups = [][2]string{
		{"none", "no group"},
		{"user", "user"},
		{"staff", "staff"},
		{"admin", "admin"},
	}
)

// User is a user record in database.
type User struct {
	ID       int           `db:"id" goqu:"skipinsert,skipupdate"`
	UID      string        `db:"uid"`
	Created  time.Time     `db:"created" goqu:"skipupdate"`
	Updated  time.Time     `db:"updated"`
	Username string        `db:"username"`
	Email    string        `db:"email"`
	Password string        `db:"password"`
	Group    string        `db:"group"`
	Settings *UserSettings `db:"settings"`
	Seed     int           `db:"seed"`
}

// Manager is a query helper for user entries.
type Manager struct{}

// Query returns a prepared goqu SelectDataset that can be extended later.
func (m *Manager) Query() *goqu.SelectDataset {
	return db.Q().From(goqu.T(TableName).As("u")).Prepared(true)
}

// GetOne executes the a select query and returns the first result or an error
// when there's no result.
func (m *Manager) GetOne(expressions ...goqu.Expression) (*User, error) {
	var u User
	found, err := m.Query().Where(expressions...).ScanStruct(&u)

	switch {
	case err != nil:
		return nil, err
	case !found:
		return nil, ErrNotFound
	}

	return &u, nil
}

// Count returns the number of user in the database.
func (m *Manager) Count() (int64, error) {
	return db.Q().From(TableName).Count()
}

// Create insert a new user in the database. The password
// must be present. It will be hashed and updated before insertion.
func (m *Manager) Create(user *User) error {
	if strings.TrimSpace(user.Password) == "" {
		return errors.New("password is empty")
	}
	hash, err := passlib.Hash(user.Password)
	if err != nil {
		return err
	}
	user.Password = hash

	user.Created = time.Now()
	user.Updated = user.Created
	user.UID = base58.NewUUID()
	user.SetSeed()

	ds := db.Q().Insert(TableName).
		Rows(user).
		Prepared(true)

	id, err := db.InsertWithID(ds, "id")
	if err != nil {
		return err
	}

	user.ID = id
	return nil
}

// Update updates some user values.
func (u *User) Update(v interface{}) error {
	if u.ID == 0 {
		return errors.New("no ID")
	}

	_, err := db.Q().Update(TableName).Prepared(true).
		Set(v).
		Where(goqu.C("id").Eq(u.ID)).
		Executor().Exec()

	return err
}

// Save updates all the user values.
func (u *User) Save() error {
	u.Updated = time.Now()
	return u.Update(u)
}

// Delete removes a user from the database.
func (u *User) Delete() error {
	_, err := db.Q().Delete(TableName).Prepared(true).
		Where(goqu.C("id").Eq(u.ID)).
		Executor().Exec()

	return err
}

// UpdateEtag implements [server.Etagger] interface.
func (u *User) UpdateEtag(h hash.Hash) {
	io.WriteString(h, u.UID+strconv.FormatInt(u.Updated.UTC().UnixNano(), 10))
}

// GetLastModified implements LastModer interface.
func (u *User) GetLastModified() []time.Time {
	return []time.Time{u.Updated}
}

// CheckPassword checks if the given password matches the
// current user password.
func (u *User) CheckPassword(password string) bool {
	newhash, err := passlib.Verify(password, u.Password)
	if err != nil {
		return false
	}

	// Update the password when needed
	if newhash != "" {
		_ = u.Update(goqu.Record{"password": newhash, "updated": time.Now()})
	}

	return true
}

// HashPassword returns a new hashed password.
func (u *User) HashPassword(password string) (string, error) {
	return passlib.Hash(password)
}

// SetPassword set a new user password.
func (u *User) SetPassword(password string) error {
	var err error
	if u.Password, err = u.HashPassword(password); err != nil {
		return err
	}

	return u.Update(goqu.Record{"password": u.Password, "updated": time.Now()})
}

// SetSeed sets a new seed to the user. It returns the seed as an integer value
// and does *not* save the data but the seed is accessible on the user instance.
func (u *User) SetSeed() int {
	u.Seed = rand.IntN(32767) //nolint:gosec
	return u.Seed
}

// IsAnonymous returns true when the instance is not set to any existing user
// (when ID is 0).
func (u *User) IsAnonymous() bool {
	return u.ID == 0
}

// Permissions returns all the user's implicit permissions.
func (u *User) Permissions() []string {
	r, _ := acls.GetPermissions(u.Group)
	return r
}

// HasPermission returns true if the user can perform "act" action
// on "obj" object.
func (u *User) HasPermission(obj, act string) bool {
	r, err := acls.Check(u.Group, obj, act)
	if err != nil {
		slog.Error("ACL check error", slog.Any("err", err))
		return false
	}
	return r
}

// UserSettings contains some user settings.
type UserSettings struct {
	DebugInfo      bool           `json:"debug_info"`
	Lang           string         `json:"lang"`
	AddonReminder  bool           `json:"addon_reminder"`
	EmailSettings  EmailSettings  `json:"email_settings"`
	ReaderSettings ReaderSettings `json:"reader_settings"`
}

// EmailSettings contains the user's email settings.
type EmailSettings struct {
	ReplyTo string `json:"reply_to"`
	EpubTo  string `json:"epub_to"`
}

// ReaderSettings contains the reader settings.
type ReaderSettings struct {
	Width       int    `json:"width"`
	Font        string `json:"font"`
	FontSize    int    `json:"font_size"`
	LineHeight  int    `json:"line_height"`
	Justify     int    `json:"justify"`
	Hyphenation int    `json:"hyphenation"`
}

// Scan loads a UserSettings instance from a column.
func (s *UserSettings) Scan(value any) error {
	if value == nil {
		return nil
	}

	// Default values
	s.AddonReminder = true
	s.ReaderSettings.Font = "lora"
	s.ReaderSettings.FontSize = 3
	s.ReaderSettings.LineHeight = 3

	v, err := types.JSONBytes(value)
	if err != nil {
		return err
	}
	json.Unmarshal(v, s) //nolint:errcheck
	return nil
}

// Value encodes a UserSettings value for storage.
func (s *UserSettings) Value() (driver.Value, error) {
	v, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(v), nil
}
