// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package scanner provides tools for scanning goqu results.
package scanner

import (
	"context"
	"iter"

	"github.com/doug-martin/goqu/v9"
)

type builderFunc[SRC any, RES any] func(context.Context, *SRC) *RES

// Iterator is the [iter.Seq2] returned by [Iter] and [IterTransform].
type Iterator[T any] iter.Seq2[*T, error]

// Iter returns an [iter.Seq2] that performs a [*goqu.SelectDataset] query
// and yields a pointer to T and an error after scanning each row into T.
func Iter[T any](ds *goqu.SelectDataset) Iterator[T] {
	return func(yield func(*T, error) bool) {
		s, err := ds.Executor().Scanner()
		if err != nil {
			yield(nil, err)
			return
		}
		defer s.Close() //nolint:errcheck

		for s.Next() {
			r := new(T)
			if err = s.ScanStruct(r); err != nil {
				yield(nil, err)
				return
			}
			if !yield(r, nil) {
				return
			}
		}
	}
}

// IterTransform returns a [iter.Seq2] that runs [Iter] and yields
// each item and an error after running a transformation function.
func IterTransform[SRC, RES any](ctx context.Context, ds *goqu.SelectDataset, builder builderFunc[SRC, RES]) Iterator[RES] {
	return func(yield func(*RES, error) bool) {
		for src, err := range Iter[SRC](ds) {
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(builder(ctx, src), nil) {
				return
			}
		}
	}
}
