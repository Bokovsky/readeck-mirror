// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package searchstring

import (
	"fmt"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
)

const sqliteCatchAll = "catchall:oooooo"

var valueCleanup = func() *strings.Replacer {
	excluded := [][2]int{
		{0x00, 0x1f},
		{0x21, 0x2f},
		{0x3a, 0x40},
		{0x5b, 0x60},
		{0x7b, 0xbf},

		{0xFDD0, 0xFDDF},
		{0xFFFE, 0xFFFF},

		{0x1FFFE, 0x1FFFF},
		{0x2FFFE, 0x2FFFF},
		{0x3FFFE, 0x3FFFF},
		{0x4FFFE, 0x4FFFF},
		{0x5FFFE, 0x5FFFF},
		{0x6FFFE, 0x6FFFF},
		{0x7FFFE, 0x7FFFF},
		{0x8FFFE, 0x8FFFF},
		{0x9FFFE, 0x9FFFF},
		{0xAFFFE, 0xAFFFF},
		{0xBFFFE, 0xBFFFF},
		{0xCFFFE, 0xCFFFF},
		{0xDFFFE, 0xDFFFF},
		{0xEFFFE, 0xEFFFF},
		{0xFFFFE, 0xFFFFF},
		{0x10FFFE, 0x10FFFF},
	}
	repl := []string{}
	for _, t := range excluded {
		for i := t[0]; i <= t[1]; i++ {
			repl = append(repl, string(rune(i)), " ")
		}
	}
	return strings.NewReplacer(repl...)
}()

// BuilderConfig contains the configuration for the SQL builder.
type BuilderConfig struct {
	relation      [2]exp.IdentifierExpression
	fieldList     [][2]string
	allowedFields map[string]string
}

// NewBuilderConfig returns a new BuilderConfig.
func NewBuilderConfig(left, right exp.IdentifierExpression, fields [][2]string) *BuilderConfig {
	res := &BuilderConfig{
		relation:      [2]exp.IdentifierExpression{left, right},
		fieldList:     fields,
		allowedFields: map[string]string{},
	}

	for _, x := range res.fieldList {
		res.allowedFields[x[0]] = x[1]
	}

	return res
}

// BuildSQL returns a new dataset with the search query.
func BuildSQL(ds *goqu.SelectDataset, q SearchQuery, conf *BuilderConfig) *goqu.SelectDataset {
	switch ds.Dialect().Dialect() {
	case "postgres":
		return builderPostgres(ds, q, conf)
	case "sqlite3":
		return buildSqlite(ds, q, conf)
	}

	panic("dialect not implemented")
}

func buildSqlite(ds *goqu.SelectDataset, q SearchQuery, cfg *BuilderConfig) *goqu.SelectDataset {
	// We need the first catchall query in order to use
	// any operator (AND and NOT)later.
	match := []string{sqliteCatchAll}
	groups := regroupFields(q.Terms, cfg)

	for _, x := range cfg.fieldList {
		terms, ok := groups[x[0]]

		if !ok {
			continue
		}
		f := x[1]
		for _, t := range terms {
			op := "AND"
			wildcard := ""
			if t.Exclude {
				op = "NOT"
			}
			if t.Wildcard {
				wildcard = "*"
			}
			match = append(match, fmt.Sprintf(`%s %s:"%s"%s`, op, f, t.Value, wildcard))
		}
	}

	return ds.Join(
		goqu.T(cfg.relation[1].GetTable()),
		goqu.On(cfg.relation[1].Eq(cfg.relation[0])),
	).
		Where(goqu.L(
			"? match ?",
			goqu.T(cfg.relation[1].GetTable()),
			goqu.V(strings.Join(match, " ")),
		)).
		Order(goqu.L("rank").Asc())
}

func builderPostgres(ds *goqu.SelectDataset, q SearchQuery, cfg *BuilderConfig) *goqu.SelectDataset {
	where := goqu.And()
	order := []exp.OrderedExpression{}
	groups := regroupFields(q.Terms, cfg)

	for _, x := range cfg.fieldList {
		terms, ok := groups[x[0]]
		if !ok {
			continue
		}
		f := x[1]
		values := []string{}

		for _, t := range terms {
			words := strings.Fields(t.Value)
			neg := ""
			if t.Exclude {
				neg = "!"
			}

			var value string
			if t.Exact {
				value = fmt.Sprintf("'%s'", strings.Join(words, " "))
				if t.Wildcard {
					value += ":*"
				}
			} else {
				if t.Wildcard {
					for i := range words {
						words[i] += ":*"
					}
				}
				value = strings.Join(words, " & ")
			}

			values = append(values, fmt.Sprintf("%s(%s)", neg, value))
		}

		value := goqu.V(strings.Join(values, " & "))
		where = where.Append(goqu.L(
			"? @@ to_tsquery('ts', ?)",
			goqu.L(f), value,
		))
		order = append(order, goqu.L(`ts_rank_cd(?, to_tsquery('ts', ?))`, goqu.L(f), value).Desc())
	}

	return ds.Join(
		goqu.T(cfg.relation[1].GetTable()),
		goqu.On(cfg.relation[1].Eq(cfg.relation[0])),
	).
		Where(where).
		Order(order...)
}

func regroupFields(terms []SearchTerm, cfg *BuilderConfig) map[string][]SearchTerm {
	groups := map[string][]SearchTerm{}

	for _, t := range terms {
		if _, ok := cfg.allowedFields[t.Field]; !ok {
			t.Value = t.Field + ":" + t.Value
			t.Field = ""
			t.Exact = true
		}

		values := strings.Fields(valueCleanup.Replace(t.Value))
		if len(values) == 0 {
			continue
		}

		t.Value = strings.Join(values, " ")

		groups[t.Field] = append(groups[t.Field], t)
	}

	return groups
}
