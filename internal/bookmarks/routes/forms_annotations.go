// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	"strings"
	"time"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type annotationForm struct {
	*forms.Form
}

func newAnnotationUpdateForm(tr forms.Translator) *forms.Form {
	return forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("color", forms.Trim, forms.Required),
	)
}

func newAnnotationForm(tr forms.Translator) *annotationForm {
	return &annotationForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("start_selector", forms.Required, forms.Trim),
		forms.NewIntegerField("start_offset", forms.Required, forms.Gte(0)),
		forms.NewTextField("end_selector", forms.Required, forms.Trim),
		forms.NewIntegerField("end_offset", forms.Required, forms.Gte(0)),
		forms.NewTextField("color", forms.Required, forms.Trim),
	)}
}

func (f *annotationForm) addToBookmark(bi *dataset.Bookmark) (*bookmarks.BookmarkAnnotation, error) {
	annotation := &bookmarks.BookmarkAnnotation{
		ID:            base58.NewUUID(),
		StartSelector: f.Get("start_selector").String(),
		StartOffset:   f.Get("start_offset").Value().(int),
		EndSelector:   f.Get("end_selector").String(),
		EndOffset:     f.Get("end_offset").Value().(int),
		Color:         f.Get("color").String(),
		Created:       time.Now().UTC(),
	}

	// Try to insert the new annotation
	reader, err := bi.GetArticle()
	if err != nil {
		return nil, err
	}

	var doc *html.Node
	if doc, err = html.Parse(reader); err != nil {
		return nil, err
	}
	root := dom.QuerySelector(doc, "body")

	// Add annotation and store its text content
	contents := &strings.Builder{}
	err = annotation.AddToNode(root, bi.AnnotationTag, func(n *html.Node, index int) {
		contents.WriteString(n.FirstChild.Data)
		bi.AnnotationCallback(annotation.ID, n, index, annotation.Color)
	})
	if err != nil {
		return nil, err
	}

	annotation.Text = strings.TrimSpace(contents.String())

	// All good? Create the annotation now
	b := bi.Bookmark
	if b.Annotations == nil {
		b.Annotations = bookmarks.BookmarkAnnotations{}
	}

	b.Annotations.Add(annotation)
	b.Annotations.Sort(root, bi.AnnotationTag)

	err = b.Update(map[string]interface{}{
		"annotations": b.Annotations,
	})
	if err != nil {
		return nil, err
	}

	return annotation, nil
}
