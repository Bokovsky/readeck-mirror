// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package converter provides bookmark export/converter tooling.
package converter

import (
	"context"
	"io"
	"net/http"

	"codeberg.org/readeck/readeck/internal/bookmarks"
)

type (
	ctxURLReplaceKey         struct{}
	ctxAnnotationTagKey      struct{}
	ctxAnnotationCallbackKey struct{}
)

// Exporter describes a bookmarks exporter.
type Exporter interface {
	Export(ctx context.Context, w io.Writer, r *http.Request, bookmarks []*bookmarks.Bookmark) error
}

// URLReplacerFunc is a function that returns a URL replacement function.
// The returned function receives a name that always starts with "./_resources/".
type URLReplacerFunc func(b *bookmarks.Bookmark) func(name string) string

// WithURLReplacer adds to context the URL replacment values for image sources.
func WithURLReplacer(ctx context.Context, fn URLReplacerFunc) context.Context {
	return context.WithValue(ctx, ctxURLReplaceKey{}, fn)
}

func getURLReplacer(ctx context.Context) (fn URLReplacerFunc, ok bool) {
	fn, ok = ctx.Value(ctxURLReplaceKey{}).(URLReplacerFunc)
	return
}

// WithAnnotationTag adds to context the annotation tag and callback function.
func WithAnnotationTag(ctx context.Context, tag string, callback annotationCallback) context.Context {
	ctx = context.WithValue(ctx, ctxAnnotationTagKey{}, tag)
	ctx = context.WithValue(ctx, ctxAnnotationCallbackKey{}, callback)
	return ctx
}

func getAnnotationTag(ctx context.Context) (tag string, callback annotationCallback) {
	tag, _ = ctx.Value(ctxAnnotationTagKey{}).(string)
	callback, _ = ctx.Value(ctxAnnotationCallbackKey{}).(annotationCallback)
	return
}
