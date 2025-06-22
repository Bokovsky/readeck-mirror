// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package dataset contains the basic blocks to properly render bookmarks related items.
package dataset

import (
	"context"
	"os"
	"strings"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/pkg/ctxr"
)

type (
	ctxURLReplaceKey         struct{}
	ctxAnnotationTagKey      struct{}
	ctxAnnotationCallbackKey struct{}
)

// URLReplacerFunc is a function that returns a URL replacement function.
// The returned function receives a name that always starts with "./_resources/".
type URLReplacerFunc func(b *bookmarks.Bookmark) func(name string) string

var (
	// WithURLReplacer adds to context the URL replacment values for image sources.
	WithURLReplacer = ctxr.Setter[URLReplacerFunc](ctxURLReplaceKey{})
	getURLReplacer  = ctxr.Checker[URLReplacerFunc](ctxURLReplaceKey{})
)

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

type annotationCallback func(id string, n *html.Node, index int, color string)

// HTMLConverter provides HTML conversion and content retrieval tooling.
type HTMLConverter struct{}

// GetArticle returns a strings.Reader containing the
// HTML content of a bookmark. Only the body is retrieved.
//
// Note: this method will always return a non nil strings.Reader. In case of error
// it might be empty or the original one if some transformation failed.
// This lets us test for error and log them when needed.
//
// The converter will use whatever is passed to [WithURLReplacer] and
// [WithAnnotationTag].
func (c HTMLConverter) GetArticle(ctx context.Context, b *bookmarks.Bookmark) (*strings.Reader, error) {
	var err error
	var bc *bookmarks.BookmarkContainer
	bc, err = b.OpenContainer()
	if err != nil {
		return strings.NewReader(""), nil
	}
	defer bc.Close()

	if err = bc.LoadArticle(); err != nil {
		if os.IsNotExist(err) {
			return strings.NewReader(""), nil
		}
		return strings.NewReader(""), err
	}

	if fn, ok := getURLReplacer(ctx); ok {
		if err = bc.ReplaceLinks(fn(b)); err != nil {
			return strings.NewReader(""), err
		}
	}

	if err = bc.ExtractBody(); err != nil {
		return strings.NewReader(""), err
	}

	reader := strings.NewReader(bc.GetArticle())

	// Add bookmark annotations
	if len(b.Annotations) > 0 {
		return c.addAnnotations(ctx, b, reader)
	}

	return reader, nil
}

// addAnnotations marks the given document with its annotations.
func (c HTMLConverter) addAnnotations(ctx context.Context, b *bookmarks.Bookmark, input *strings.Reader) (*strings.Reader, error) {
	tag, callback := getAnnotationTag(ctx)
	if tag == "" {
		return input, nil
	}

	var err error
	var doc *html.Node

	if doc, err = html.Parse(input); err != nil {
		input.Seek(0, 0) //nolint:errcheck
		return input, err
	}
	root := dom.QuerySelector(doc, "body")

	err = b.Annotations.AddToNode(root, tag, callback)
	if err != nil {
		input.Seek(0, 0) //nolint:errcheck
		return input, err
	}

	buf := new(strings.Builder)
	if err = html.Render(buf, doc); err != nil {
		input.Seek(0, 0) //nolint:errcheck
		return input, err
	}

	return strings.NewReader(bookmarks.ExtractHTMLBody(buf.String())), nil
}
