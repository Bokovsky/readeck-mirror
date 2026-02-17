// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package migrations

import (
	"database/sql/driver"
	"encoding/json"
	"io/fs"

	"codeberg.org/readeck/readeck/internal/db/types"
	"github.com/doug-martin/goqu/v9"
)

type filterMap map[string]any

func (m *filterMap) Scan(value any) error {
	if value == nil {
		return nil
	}

	v, err := types.JSONBytes(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(v, m)
}

func (m filterMap) Value() (driver.Value, error) {
	v, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(v), nil
}

// M14collectionBookmarkType converts the "type" entry of each collection
// filter to a list of [string].
func M14collectionBookmarkType(db *goqu.TxDatabase, _ fs.FS) error {
	ds := db.Select(goqu.C("id"), goqu.C("filters")).From("bookmark_collection")

	collections := []struct {
		ID      int       `db:"id"`
		Filters filterMap `db:"filters"`
	}{}

	err := ds.ScanStructs(&collections)
	if err != nil {
		return err
	}

	ids := []int{}
	cases := goqu.Case()
	casePlaceholder := "?"
	if db.Dialect() == "postgres" {
		casePlaceholder = "?::jsonb"
	}

	for _, row := range collections {
		newValue := []string{}
		if v, ok := row.Filters["type"]; ok {
			if s, ok := v.(string); ok && s != "" {
				newValue = []string{s}
			}
		}
		row.Filters["type"] = newValue

		cases = cases.When(goqu.C("id").Eq(row.ID), goqu.L(casePlaceholder, row.Filters))
		ids = append(ids, row.ID)
	}

	if len(cases.GetWhens()) == 0 {
		return nil
	}

	_, err = db.Update("bookmark_collection").Prepared(true).
		Set(goqu.Record{"filters": cases}).
		Where(goqu.C("id").In(ids)).
		Executor().Exec()
	return err
}
