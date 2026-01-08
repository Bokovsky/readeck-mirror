// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package dataset

import (
	"context"
	"hash"
	"io"
	"strconv"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/db/scanner"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
)

// AnnotationTag is the default HTML tag for highlights.
const AnnotationTag = "rd-annotation"

// AnnotationCallback returns a [bookmarks.AnnotationCallback] that adds the
// necessary attributes and the note when "withNote" is true.
func AnnotationCallback(withNote bool) bookmarks.AnnotationCallback {
	return func(a *bookmarks.BookmarkAnnotation, n *html.Node, index, ln int) {
		if index == 0 {
			dom.SetAttribute(n, "id", "annotation-"+a.ID)
		}
		color := a.Color
		if color == "" {
			color = "yellow"
		}
		dom.SetAttribute(n, "data-annotation-id-value", a.ID)
		dom.SetAttribute(n, "data-annotation-color", color)

		// If there is a note, we add an extra, empty, rd-annotation tag that contains the note.
		if withNote && a.Note != "" && index+1 == ln {
			noteNode := dom.CreateElement(AnnotationTag)
			dom.SetAttribute(noteNode, "data-annotation-id-value", a.ID)
			dom.SetAttribute(noteNode, "data-annotation-note", "")
			dom.SetAttribute(noteNode, "title", a.Note)
			dom.SetAttribute(noteNode, "data-annotation-color", color)

			n.Parent.InsertBefore(noteNode, n.NextSibling)
		}
	}
}

// AnnotationList is a collection of [Annotation] items.
type AnnotationList struct {
	Count      int64
	Pagination server.Pagination
	Items      []*Annotation
}

// NewAnnotationList returns a new [*AnnotationList].
//
//nolint:dupl
func NewAnnotationList(ctx context.Context, ds *goqu.SelectDataset) (*AnnotationList, error) {
	res := &AnnotationList{
		Items: []*Annotation{},
	}

	var err error
	if res.Count, err = ds.ClearOrder().ClearLimit().ClearOffset().Count(); err != nil {
		return nil, err
	}

	if limit, ok := ds.GetClauses().Limit().(uint); ok {
		res.Pagination = server.NewPagination(ctx,
			int(res.Count), int(limit), int(ds.GetClauses().Offset()),
		)
	}

	if res.Count == 0 {
		return res, nil
	}

	for item, err := range scanner.IterTransform(ctx, ds, NewAnnotation) {
		if err != nil {
			return nil, err
		}
		res.Items = append(res.Items, item)
	}

	return res, nil
}

// UpdateEtag implements [server.Etagger].
func (al AnnotationList) UpdateEtag(h hash.Hash) {
	for _, item := range al.Items {
		io.WriteString(h, item.ID+strconv.FormatInt(item.Created.UTC().UnixNano(), 10))
	}
}

// Annotation is a serialized annotation instance than be used directly
// on the API or by an HTML template.
type Annotation struct {
	ID               string    `json:"id"`
	Href             string    `json:"href"`
	Text             string    `json:"text"`
	Created          time.Time `json:"created"`
	Color            string    `json:"color"`
	Note             string    `json:"note"`
	BookmarkID       string    `json:"bookmark_id"`
	BookmarkHref     string    `json:"bookmark_href"`
	BookmarkURL      string    `json:"bookmark_url"`
	BookmarkTitle    string    `json:"bookmark_title"`
	BookmarkSiteName string    `json:"bookmark_site_name"`
}

// NewAnnotation builds an [Annotation] for a [bookmarks.AnnotationQueryResult] instance.
func NewAnnotation(ctx context.Context, a *bookmarks.AnnotationQueryResult) *Annotation {
	return &Annotation{
		ID:               a.ID,
		Href:             urls.AbsoluteURLContext(ctx, "/api/bookmarks", a.Bookmark.UID, "annotations", a.ID).String(),
		Text:             a.Text,
		Created:          time.Time(a.Created),
		Color:            a.Color,
		Note:             a.Note,
		BookmarkID:       a.Bookmark.UID,
		BookmarkHref:     urls.AbsoluteURLContext(ctx, "/api/bookmarks", a.Bookmark.UID).String(),
		BookmarkURL:      a.Bookmark.URL,
		BookmarkTitle:    a.Bookmark.Title,
		BookmarkSiteName: a.Bookmark.SiteName,
	}
}
