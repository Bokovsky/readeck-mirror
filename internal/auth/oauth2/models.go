// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oauth2

import (
	"errors"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/base58"
)

const (
	// TableName is the database table.
	TableName = "oauth2_client"
)

var (
	// Clients is the model manager for [Client] instances.
	Clients = Manager{}

	// ErrNotFound is returned when a token record was not found.
	ErrNotFound = errors.New("not found")
)

// Client is an oauth2 client record in database.
type Client struct {
	ID              int           `db:"id" goqu:"skipinsert,skipupdate"`
	UID             string        `db:"uid"`
	Created         time.Time     `db:"created" goqu:"skipupdate"`
	Name            string        `db:"name"`
	Website         string        `db:"website"`
	Logo            string        `db:"logo"`
	RedirectURIs    types.Strings `db:"redirect_uris"`
	SoftwareID      string        `db:"software_id"`
	SoftwareVersion string        `db:"software_version"`
}

// Manager is a query helper for client entries.
type Manager struct{}

// Query returns a prepared [goqu.SelectDataset] that can be extended later.
func (m *Manager) Query() *goqu.SelectDataset {
	return db.Q().From(goqu.T(TableName).As("c")).Prepared(true)
}

// GetOne executes the a select query and returns the first result or an error
// when there's no result.
func (m *Manager) GetOne(expressions ...goqu.Expression) (*Client, error) {
	var c Client
	found, err := m.Query().Where(expressions...).ScanStruct(&c)

	switch {
	case err != nil:
		return nil, err
	case !found:
		return nil, ErrNotFound
	}

	return &c, nil
}

// Create insert a new client in the database.
func (m *Manager) Create(client *Client) error {
	client.Created = time.Now().UTC()
	client.UID = base58.NewUUID()

	ds := db.Q().Insert(TableName).
		Rows(client).
		Prepared(true)

	id, err := db.InsertWithID(ds, "id")
	if err != nil {
		return err
	}

	client.ID = id
	return nil
}

// Update updates some bookmark values.
func (c *Client) Update(v interface{}) error {
	if c.ID == 0 {
		return errors.New("no ID")
	}

	_, err := db.Q().Update(TableName).Prepared(true).
		Set(v).
		Where(goqu.C("id").Eq(c.ID)).
		Executor().Exec()

	return err
}

// Save updates all the token values.
func (c *Client) Save() error {
	return c.Update(c)
}

// Delete removes a token from the database.
func (c *Client) Delete() error {
	_, err := db.Q().Delete(TableName).Prepared(true).
		Where(goqu.C("id").Eq(c.ID)).
		Executor().Exec()

	return err
}
