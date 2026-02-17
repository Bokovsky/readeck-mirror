// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package searchstring

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/require"
)

type queryExpect struct {
	sql  string
	args []any
}

var rxSpace = regexp.MustCompile(`\s+`)

func TestRegroupFields(t *testing.T) {
	tests := []struct {
		terms    []SearchTerm
		expected map[string][]SearchTerm
	}{
		{
			[]SearchTerm{
				{Value: "test"},
				{Field: "label", Value: "L1"},
				{Value: "  test & some | again !"},
				{Field: "foo", Value: "F1"},
			},
			map[string][]SearchTerm{
				"": {
					{Value: "test"},
					{Value: "test some again"},
					{Value: "foo F1", Exact: true},
				},
				"label": {
					{Field: "label", Value: "L1"},
				},
			},
		},
		{
			[]SearchTerm{
				{Field: "title", Value: "test 🦊"},
				{Field: "label", Value: "L1"},
				{Value: " t1 <> t2 \" ?t3 !t4 "},
				{Value: "  ?#  "},
			},
			map[string][]SearchTerm{
				"": {
					{Value: "t1 t2 t3 t4"},
				},
				"label": {
					{Field: "label", Value: "L1"},
				},
				"title": {
					{Field: "title", Value: "test 🦊"},
				},
			},
		},
	}

	cfg := NewBuilderConfig(
		goqu.I("T.id"),
		goqu.I("FTS.t_id"),
		[][2]string{
			{"", ""},
			{"label", ""},
			{"title", ""},
		},
	)

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			groups := regroupFields(test.terms, cfg)
			require.Equal(t, test.expected, groups)
		})
	}
}

func TestSQLBuilder(t *testing.T) {
	tests := []struct {
		terms    []SearchTerm
		expected map[string]queryExpect
	}{
		{
			[]SearchTerm{{Value: "test"}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + " AND -catchall:\"test\""},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE FTS.title || FTS.label @@ to_tsquery('ts', $1)
					ORDER BY ts_rank_cd(FTS.title || FTS.label, to_tsquery('ts', $2)) DESC
					`,
					[]any{"(test)", "(test)"},
				},
			},
		},
		{
			[]SearchTerm{{Value: "test  🦊   キツネ", Exact: true}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + " AND -catchall:\"test 🦊 キツネ\""},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE FTS.title || FTS.label @@ to_tsquery('ts', $1)
					ORDER BY ts_rank_cd(FTS.title || FTS.label, to_tsquery('ts', $2)) DESC
					`,
					[]any{"('test 🦊 キツネ')", "('test 🦊 キツネ')"},
				},
			},
		},
		{
			[]SearchTerm{{Field: "label", Value: "test"}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + " AND label:\"test\""},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE FTS.label @@ to_tsquery('ts', $1)
					ORDER BY ts_rank_cd(FTS.label, to_tsquery('ts', $2)) DESC
					`,
					[]any{"(test)", "(test)"},
				},
			},
		},
		{
			[]SearchTerm{{Field: "label", Value: "test", Exclude: true}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + " NOT label:\"test\""},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE FTS.label @@ to_tsquery('ts', $1)
					ORDER BY ts_rank_cd(FTS.label, to_tsquery('ts', $2)) DESC
					`,
					[]any{"!(test)", "!(test)"},
				},
			},
		},
		{
			[]SearchTerm{{Field: "label", Value: "L1 L2", Exact: true, Exclude: true}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + " NOT label:\"L1 L2\""},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE FTS.label @@ to_tsquery('ts', $1)
					ORDER BY ts_rank_cd(FTS.label, to_tsquery('ts', $2)) DESC
					`,
					[]any{"!('L1 L2')", "!('L1 L2')"},
				},
			},
		},
		{
			[]SearchTerm{{Field: "foo", Value: "test"}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + " AND -catchall:\"foo test\""},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE FTS.title || FTS.label @@ to_tsquery('ts', $1)
					ORDER BY ts_rank_cd(FTS.title || FTS.label, to_tsquery('ts', $2)) DESC
					`,
					[]any{"('foo test')", "('foo test')"},
				},
			},
		},
		{
			[]SearchTerm{{Field: "foo", Value: "test", Wildcard: true}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + " AND -catchall:\"foo test\"*"},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE FTS.title || FTS.label @@ to_tsquery('ts', $1)
					ORDER BY ts_rank_cd(FTS.title || FTS.label, to_tsquery('ts', $2)) DESC
					`,
					[]any{"('foo test':*)", "('foo test':*)"},
				},
			},
		},
		{
			[]SearchTerm{{Field: "foo ! @ bar", Value: "test <-> @! testing"}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + " AND -catchall:\"foo bar test testing\""},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE FTS.title || FTS.label @@ to_tsquery('ts', $1)
					ORDER BY ts_rank_cd(FTS.title || FTS.label, to_tsquery('ts', $2)) DESC
					`,
					[]any{"('foo bar test testing')", "('foo bar test testing')"},
				},
			},
		},
		{
			[]SearchTerm{{Value: "C1"}, {Value: "C2"}, {Field: "title", Value: "T1"}, {Field: "title", Value: "T2", Exclude: true}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + ` AND -catchall:"C1" AND -catchall:"C2" AND title:"T1" NOT title:"T2"`},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE (FTS.title || FTS.label @@ to_tsquery('ts', $1)
					AND FTS.title @@ to_tsquery('ts', $2))
					ORDER BY ts_rank_cd(FTS.title || FTS.label, to_tsquery('ts', $3)) DESC,
					ts_rank_cd(FTS.title, to_tsquery('ts', $4)) DESC
					`,
					[]any{"(C1) & (C2)", "(T1) & !(T2)", "(C1) & (C2)", "(T1) & !(T2)"},
				},
			},
		},
		{
			[]SearchTerm{{Value: "C1", Wildcard: true}, {Value: "C2"}, {Field: "title", Value: "T1"}, {Field: "title", Value: "T2", Wildcard: true, Exclude: true}},
			map[string]queryExpect{
				"sqlite3": {
					"SELECT * FROM `T` INNER JOIN `FTS` ON (`FTS`.`rowid` = `T`.`id`) WHERE `FTS` match ? ORDER BY rank ASC",
					[]any{sqliteCatchAll + ` AND -catchall:"C1"* AND -catchall:"C2" AND title:"T1" NOT title:"T2"*`},
				},
				"postgres": {
					`
					SELECT * FROM "T" INNER JOIN "FTS" ON ("FTS"."t_id" = "T"."id")
					WHERE (FTS.title || FTS.label @@ to_tsquery('ts', $1)
					AND FTS.title @@ to_tsquery('ts', $2))
					ORDER BY ts_rank_cd(FTS.title || FTS.label, to_tsquery('ts', $3)) DESC,
					ts_rank_cd(FTS.title, to_tsquery('ts', $4)) DESC
					`,
					[]any{"(C1:*) & (C2)", "(T1) & !(T2:*)", "(C1:*) & (C2)", "(T1) & !(T2:*)"},
				},
			},
		},
	}

	configs := map[string]*BuilderConfig{
		"postgres": NewBuilderConfig(
			goqu.I("T.id"),
			goqu.I("FTS.t_id"),
			[][2]string{
				{"", `FTS.title || FTS.label`},
				{"title", "FTS.title"},
				{"label", "FTS.label"},
			},
		),
		"sqlite3": NewBuilderConfig(
			goqu.I("T.id"),
			goqu.I("FTS.rowid"),
			[][2]string{
				{"", "-catchall"},
				{"title", "title"},
				{"label", "label"},
			},
		),
	}

	for i, test := range tests {
		for _, dialect := range []string{"sqlite3", "postgres"} {
			if _, ok := test.expected[dialect]; !ok {
				continue
			}
			t.Run(fmt.Sprintf("%d-%s", i+1, dialect), func(t *testing.T) {
				q := SearchQuery{Terms: test.terms}
				ds := goqu.Dialect(dialect).Select().From("T")
				ds = BuildSQL(ds, q, configs[dialect])

				sql, args, err := ds.Prepared(true).ToSQL()
				require.NoError(t, err)

				require.Equal(t,
					rxSpace.ReplaceAllString(strings.TrimSpace(test.expected[dialect].sql), " "),
					sql,
				)
				require.Equal(t, test.expected[dialect].args, args)
			})
		}
	}
}
