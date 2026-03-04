// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package exp_test

import (
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	goquexp "github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/db/exp"
)

type queryExpect struct {
	sql  string
	args []any
}

func TestFunctions(t *testing.T) {
	tests := []struct {
		ds       func(ds *goqu.SelectDataset) *goqu.SelectDataset
		expected map[string]queryExpect
	}{
		{
			func(ds *goqu.SelectDataset) *goqu.SelectDataset {
				return ds.Select(exp.Boolean(goqu.C("x"), true))
			},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT `x` FROM `T`",
					[]any{},
				},
				"postgres": {
					`SELECT "x" FROM "T"`,
					[]any{},
				},
			},
		},
		{
			func(ds *goqu.SelectDataset) *goqu.SelectDataset {
				return ds.Select(exp.Boolean(goqu.C("x"), false))
			},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT NOT(`x`) FROM `T`",
					[]any{},
				},
				"postgres": {
					`SELECT NOT("x") FROM "T"`,
					[]any{},
				},
			},
		},
		{
			func(ds *goqu.SelectDataset) *goqu.SelectDataset {
				return ds.Select(exp.DateTime(goqu.C("x")))
			},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT datetime(`x`) FROM `T`",
					[]any{},
				},
				"postgres": {
					`SELECT "x" FROM "T"`,
					[]any{},
				},
			},
		},
		{
			func(ds *goqu.SelectDataset) *goqu.SelectDataset {
				return ds.Select(exp.Greatest(goqu.V(1), goqu.V(2)))
			},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT max(?, ?) FROM `T`",
					[]any{int64(1), int64(2)},
				},
				"postgres": {
					`SELECT greatest($1, $2) FROM "T"`,
					[]any{int64(1), int64(2)},
				},
			},
		},
	}

	for i, test := range tests {
		for _, dialect := range []string{"sqlite3", "postgres"} {
			if _, ok := test.expected[dialect]; !ok {
				continue
			}
			t.Run(fmt.Sprintf("%d-%s", i+1, dialect), func(t *testing.T) {
				db.SetDriver(dialect)
				defer db.SetDriver("")

				ds := goqu.Dialect(dialect).Select().From("T")
				ds = test.ds(ds)

				sql, args, err := ds.Prepared(true).ToSQL()
				require.NoError(t, err)
				// t.Logf("%#v -- %#v", sql, args)

				require.Equal(t,
					test.expected[dialect].sql,
					sql,
				)
				require.Equal(t, test.expected[dialect].args, args)
			})
		}
	}
}

func TestStringsFilter(t *testing.T) {
	tests := []struct {
		expressions []goquexp.BooleanExpression
		expected    map[string]queryExpect
	}{
		{
			[]goquexp.BooleanExpression{},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T`",
					[]any{},
				},
				"postgres": {
					`SELECT * FROM "T"`,
					[]any{},
				},
			},
		},
		{
			[]goquexp.BooleanExpression{goqu.I("T.tags").Eq("test")},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` WHERE EXISTS (SELECT * FROM json_each(CASE json_type(CASE json_valid(`T`.`tags`) WHEN true THEN `T`.`tags` ELSE '[]' END) WHEN 'array' THEN `T`.`tags` ELSE '[]' END) WHERE (`json_each`.`value` = ?))",
					[]any{"test"},
				},
				"postgres": {
					`SELECT * FROM "T" WHERE EXISTS (SELECT "value" FROM jsonb_array_elements_text(CASE jsonb_typeof("T"."tags") WHEN 'array' THEN "T"."tags" ELSE '[]' END) WHERE ("value" = $1))`,
					[]any{"test"},
				},
			},
		},
		{
			[]goquexp.BooleanExpression{goqu.I("T.tags").Neq("test")},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` WHERE NOT EXISTS (SELECT * FROM json_each(CASE json_type(CASE json_valid(`T`.`tags`) WHEN true THEN `T`.`tags` ELSE '[]' END) WHEN 'array' THEN `T`.`tags` ELSE '[]' END) WHERE (`json_each`.`value` = ?))",
					[]any{"test"},
				},
				"postgres": {
					`SELECT * FROM "T" WHERE NOT EXISTS (SELECT "value" FROM jsonb_array_elements_text(CASE jsonb_typeof("T"."tags") WHEN 'array' THEN "T"."tags" ELSE '[]' END) WHERE ("value" = $1))`,
					[]any{"test"},
				},
			},
		},
		{
			[]goquexp.BooleanExpression{goqu.I("T.tags").Like("test")},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` WHERE EXISTS (SELECT * FROM json_each(CASE json_type(CASE json_valid(`T`.`tags`) WHEN true THEN `T`.`tags` ELSE '[]' END) WHEN 'array' THEN `T`.`tags` ELSE '[]' END) WHERE (`json_each`.`value` LIKE ?))",
					[]any{"test"},
				},
				"postgres": {
					`SELECT * FROM "T" WHERE EXISTS (SELECT "value" FROM jsonb_array_elements_text(CASE jsonb_typeof("T"."tags") WHEN 'array' THEN "T"."tags" ELSE '[]' END) WHERE ("value" ILIKE $1))`,
					[]any{"test"},
				},
			},
		},
		{
			[]goquexp.BooleanExpression{goqu.I("T.tags").NotLike("test")},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` WHERE NOT EXISTS (SELECT * FROM json_each(CASE json_type(CASE json_valid(`T`.`tags`) WHEN true THEN `T`.`tags` ELSE '[]' END) WHEN 'array' THEN `T`.`tags` ELSE '[]' END) WHERE (`json_each`.`value` LIKE ?))",
					[]any{"test"},
				},
				"postgres": {
					`SELECT * FROM "T" WHERE NOT EXISTS (SELECT "value" FROM jsonb_array_elements_text(CASE jsonb_typeof("T"."tags") WHEN 'array' THEN "T"."tags" ELSE '[]' END) WHERE ("value" ILIKE $1))`,
					[]any{"test"},
				},
			},
		},
		{
			[]goquexp.BooleanExpression{goqu.I("T.tags").Eq("test"), goqu.I("T.labels").Neq("test2")},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` WHERE (EXISTS (SELECT * FROM json_each(CASE json_type(CASE json_valid(`T`.`tags`) WHEN true THEN `T`.`tags` ELSE '[]' END) WHEN 'array' THEN `T`.`tags` ELSE '[]' END) WHERE (`json_each`.`value` = ?)) AND NOT EXISTS (SELECT * FROM json_each(CASE json_type(CASE json_valid(`T`.`labels`) WHEN true THEN `T`.`labels` ELSE '[]' END) WHEN 'array' THEN `T`.`labels` ELSE '[]' END) WHERE (`json_each`.`value` = ?)))",
					[]any{"test", "test2"},
				},
				"postgres": {
					`SELECT * FROM "T" WHERE (EXISTS (SELECT "value" FROM jsonb_array_elements_text(CASE jsonb_typeof("T"."tags") WHEN 'array' THEN "T"."tags" ELSE '[]' END) WHERE ("value" = $1)) AND NOT EXISTS (SELECT "value" FROM jsonb_array_elements_text(CASE jsonb_typeof("T"."labels") WHEN 'array' THEN "T"."labels" ELSE '[]' END) WHERE ("value" = $2)))`,
					[]any{"test", "test2"},
				},
			},
		},
	}

	for i, test := range tests {
		for _, dialect := range []string{"sqlite3", "postgres"} {
			if _, ok := test.expected[dialect]; !ok {
				continue
			}
			t.Run(fmt.Sprintf("%d-%s", i+1, dialect), func(t *testing.T) {
				ds := goqu.Dialect(dialect).Select().From("T")
				ds = exp.JSONListFilter(ds, test.expressions...)

				sql, args, err := ds.Prepared(true).ToSQL()
				require.NoError(t, err)

				require.Equal(t,
					test.expected[dialect].sql,
					sql,
				)
				require.Equal(t, test.expected[dialect].args, args)
			})
		}
	}
}
