// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package exp provides query expressions for specific operations.
package exp

import (
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"

	"codeberg.org/readeck/readeck/internal/db"
)

// Boolean returns the provided [exp.Expression] or its negation when
// "value" is false.
func Boolean(expr exp.Expression, value bool) exp.Expression {
	if value {
		return expr
	}
	return goqu.Func("NOT", expr)
}

// DateTime returns a datetime expression. It wraps the expression
// in a "datetime" function with SQLite.
func DateTime(value any) interface {
	exp.Comparable
	exp.Orderable
	exp.Rangeable
} {
	if db.Driver().Dialect() == "sqlite3" {
		return goqu.Func("datetime", value)
	}

	return goqu.V(value)
}

// JSONArrayLength returns a json(b)_array_length statement of the given identifier.
func JSONArrayLength(dialect goqu.SQLDialect, identifier exp.IdentifierExpression) exp.SQLFunctionExpression {
	switch dialect.Dialect() {
	case "postgres":
		return goqu.Func(
			"jsonb_array_length",
			goqu.Case().
				When(goqu.Func("jsonb_typeof", identifier).Eq("array"), identifier).
				Else(goqu.V("[]")),
		)
	case "sqlite3":
		return goqu.Func(
			"json_array_length",
			goqu.Case().
				When(goqu.Func("json_valid", identifier), identifier).
				Else(goqu.V("[]")),
		)
	}

	return nil
}

// JSONStringsDataset returns a dataset with all the values from a given string list column in a table.
// The input [*goqu.SelectDataset] must contain exactly one From() clause and exactly one
// Select() clause, which are the table and columnd it operates one.
func JSONStringsDataset(ds *goqu.SelectDataset, name string) *goqu.SelectDataset {
	f := ds.GetClauses().From().Columns()
	c := ds.GetClauses().Select().Columns()

	if len(f) != 1 {
		panic(`"From" clause must contain exactly one element`)
	}
	if len(c) != 1 {
		panic(`"Select" clause must contain exactly one element`)
	}

	switch ds.Dialect().Dialect() {
	case "postgres":
		return ds.Select(
			goqu.C(name),
		).
			From(
				f[0],
				goqu.Func("jsonb_array_elements_text",
					goqu.Case().
						Value(goqu.Func("jsonb_typeof", c[0])).
						When(goqu.L("'array'"), c[0]).
						Else(goqu.L("'[]'")),
				).As(name),
			).
			Order(goqu.C(name).Asc())
	case "sqlite3":
		return ds.Select(
			goqu.C("value").As(name),
		).From(
			f[0],
			goqu.Func("json_each", c[0]).As("string_values"),
		).
			Where(goqu.C("value").Table("string_values").Neq(nil)).
			Order(goqu.L(fmt.Sprintf("`%s` COLLATE UNICODE", name)).Asc())
	}

	return nil
}

// JSONListFilter appends filters on list value to an existing dataset.
// It adds statements in order to find rows with JSON arrays containing the
// given expressions.
// Supported comparaisons are "Eq", "Neq", "Like" and "NotLike"
//
//	JSONListFilter(ds, goqu.T("books").C("tags").Eq("fiction"), goqu.T("books").C("tags").Neq("space"))
func JSONListFilter(ds *goqu.SelectDataset, expressions ...exp.BooleanExpression) *goqu.SelectDataset {
	if len(expressions) == 0 {
		return ds
	}

	res := goqu.And()

	switch dialect := ds.Dialect().Dialect(); dialect {
	case "postgres":
		for _, e := range expressions {
			col := e.LHS()
			cmp := "eq"
			op := "EXISTS"

			from := goqu.Dialect(dialect).Select(goqu.C("value")).From(goqu.Func(
				"jsonb_array_elements_text",
				goqu.Case().Value(goqu.Func("jsonb_typeof", col)).
					When(goqu.L("'array'"), col).
					Else(goqu.L("'[]'")),
			))

			switch e.Op() {
			case exp.LikeOp:
				cmp = "ilike"
			case exp.NotLikeOp:
				cmp = "ilike"
				op = "NOT EXISTS"
			case exp.NeqOp:
				op = "NOT EXISTS"
			}

			res = res.Append(goqu.L("? ?", goqu.L(op),
				from.Where(goqu.Ex{"value": goqu.Op{cmp: e.RHS()}}),
			))
		}
	case "sqlite3":
		for _, e := range expressions {
			col := e.LHS()
			cmp := "eq"
			op := "EXISTS"

			from := goqu.Dialect(dialect).From(goqu.Func(
				"json_each",
				goqu.Case().Value(goqu.Func("json_type",
					goqu.Case().Value(goqu.Func("json_valid", col)).
						When(goqu.L("true"), col).
						Else(goqu.L("'[]'")),
				)).
					When(goqu.L("'array'"), col).
					Else(goqu.L("'[]'")),
			))

			switch e.Op() {
			case exp.LikeOp:
				cmp = "like"
			case exp.NotLikeOp:
				cmp = "like"
				op = "NOT EXISTS"
			case exp.NeqOp:
				op = "NOT EXISTS"
			}

			res = res.Append(
				goqu.L("? ?", goqu.L(op),
					from.Where(goqu.Ex{"json_each.value": goqu.Op{cmp: e.RHS()}})),
			)
		}
	}

	return ds.Where(res)
}
