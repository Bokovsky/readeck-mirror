// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/annotate"
)

// AnnotationCallback is a function called on each annotation text node.
type AnnotationCallback func(a *BookmarkAnnotation, n *html.Node, index, ln int)

// BookmarkAnnotations is a mapping of annotations.
type BookmarkAnnotations []*BookmarkAnnotation

// BookmarkAnnotation is an annotation that can be serialized in a database JSON column.
type BookmarkAnnotation struct {
	ID            string    `json:"id"`
	StartSelector string    `json:"start_selector"`
	StartOffset   int       `json:"start_offset"`
	EndSelector   string    `json:"end_selector"`
	EndOffset     int       `json:"end_offset"`
	Color         string    `json:"color"`
	Created       time.Time `json:"created"`
	Text          string    `json:"text"`
	Note          string    `json:"note"`
}

// Scan loads a BookmarkAnnotations instance from a column.
func (a *BookmarkAnnotations) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	v, err := types.JSONBytes(value)
	if err != nil {
		return err
	}
	json.Unmarshal(v, a) //nolint:errcheck
	return nil
}

// Value encodes a BookmarkAnnotations instance for storage.
func (a BookmarkAnnotations) Value() (driver.Value, error) {
	for _, x := range a {
		if x.Color == "" {
			x.Color = "yellow"
		}
	}

	v, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(v), nil
}

// AddToNode adds one annotation to a DOM node (the designated root).
func (a *BookmarkAnnotation) AddToNode(root *html.Node, tagName string, options ...annotate.WrapCallback) error {
	return annotate.AddAnnotation(
		root, tagName,
		a.StartSelector, a.StartOffset,
		a.EndSelector, a.EndOffset,
		options...,
	)
}

// AddToNode adds all annotations to a DOM node (the designated root).
func (a BookmarkAnnotations) AddToNode(root *html.Node, tagName string, options ...AnnotationCallback) error {
	for _, annotation := range a {
		err := annotation.AddToNode(root, tagName, func(n *html.Node, index, ln int) {
			for _, f := range options {
				if f != nil {
					f(annotation, n, index, ln)
				}
			}
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Get retrieves an annotation or returns nil if it does not exist.
func (a *BookmarkAnnotations) Get(id string) *BookmarkAnnotation {
	for _, x := range *a {
		if x.ID == id {
			return x
		}
	}
	return nil
}

// Add adds a new annotation to the list.
func (a *BookmarkAnnotations) Add(item *BookmarkAnnotation) {
	set := *a
	set = append(set, item)
	*a = set
}

// Sort sorts the annotations based on their position in the root document.
func (a *BookmarkAnnotations) Sort(root *html.Node, tagName string) {
	set := BookmarkAnnotations{}
	for _, n := range dom.QuerySelectorAll(root, tagName) {
		if dom.GetAttribute(n, "id") == "" {
			continue
		}
		id := dom.GetAttribute(n, "data-annotation-id-value")
		if item := a.Get(id); item != nil {
			set = append(set, item)
		}
	}
	*a = set
}

// Delete removes an annotation from the list.
func (a *BookmarkAnnotations) Delete(id string) {
	item := a.Get(id)
	if item == nil {
		return
	}
	set := BookmarkAnnotations{}
	for _, x := range *a {
		if x.ID == id {
			continue
		}
		set = append(set, x)
	}
	*a = set
}
