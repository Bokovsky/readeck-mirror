// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package ctxr provides generic functions to work with context storage.
package ctxr

import (
	"context"
)

type (
	// ContextSetter is a function that returns a new context with a given value.
	ContextSetter[T any] func(context.Context, T) context.Context
	// ContextChecker returns a value from a context, and a boolean if the value was present
	// and of the correct type.
	ContextChecker[T any] func(context.Context) (T, bool)
	// ContextGetter returns a value from a context and panics if the type is not correct.
	ContextGetter[T any] func(context.Context) T
)

// Setter returns a [ContextSetter].
func Setter[T any](key any) ContextSetter[T] {
	return func(ctx context.Context, val T) context.Context {
		return context.WithValue(ctx, key, val)
	}
}

// Checker returns a [ContextChecker].
func Checker[T any](key any) ContextChecker[T] {
	return func(ctx context.Context) (v T, ok bool) {
		v, ok = ctx.Value(key).(T)
		return
	}
}

// Getter returns a [ContextGetter].
func Getter[T any](key any) ContextGetter[T] {
	return func(ctx context.Context) T {
		return ctx.Value(key).(T)
	}
}

// WithChecker returns a [ContextSetter] and [ContextChecker].
func WithChecker[T any](key any) (ContextSetter[T], ContextChecker[T]) {
	return Setter[T](key), Checker[T](key)
}

// WithGetter returns a [ContextSetter] and [ContextGetter].
func WithGetter[T any](key any) (ContextSetter[T], ContextGetter[T]) {
	return Setter[T](key), Getter[T](key)
}

// WithAll returns a [ContextSetter], [ContextGetter] and [ContextChecker].
func WithAll[T any](key any) (ContextSetter[T], ContextGetter[T], ContextChecker[T]) {
	return Setter[T](key), Getter[T](key), Checker[T](key)
}
