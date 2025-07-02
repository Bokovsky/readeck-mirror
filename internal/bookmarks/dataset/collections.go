// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package dataset

import (
	"context"
	"time"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/db/scanner"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"github.com/doug-martin/goqu/v9"
)

// CollectionList is a list of [*Collection] items.
type CollectionList struct {
	Count      int64
	Pagination server.Pagination
	Items      []*Collection
}

// NewCollectionList returns a new [*CollectionList].
//
//nolint:dupl
func NewCollectionList(ctx context.Context, ds *goqu.SelectDataset) (*CollectionList, error) {
	res := &CollectionList{
		Items: []*Collection{},
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

	for item, err := range scanner.IterTransform(ctx, ds, NewCollection) {
		if err != nil {
			return nil, err
		}
		res.Items = append(res.Items, item)
	}

	return res, nil
}

// Collection is a serialized collection instance that can
// be used directly on the API or by an HTML template.
type Collection struct {
	*bookmarks.Collection `json:"-"`

	ID        string    `json:"id"`
	Href      string    `json:"href"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	Name      string    `json:"name"`
	IsPinned  bool      `json:"is_pinned"`
	IsDeleted bool      `json:"is_deleted"`

	// Filters
	Search     string        `json:"search"`
	Title      string        `json:"title"`
	Author     string        `json:"author"`
	Site       string        `json:"site"`
	Type       types.Strings `json:"type"`
	Labels     string        `json:"labels"`
	ReadStatus types.Strings `json:"read_status"`
	IsMarked   *bool         `json:"is_marked"`
	IsArchived *bool         `json:"is_archived"`
	IsLoaded   *bool         `json:"is_loaded"`
	HasErrors  *bool         `json:"has_errors"`
	HasLabels  *bool         `json:"has_labels"`
	RangeStart string        `json:"range_start"`
	RangeEnd   string        `json:"range_end"`
}

// NewCollection returns a new [*Collection] from a [*bookmarks.Collection].
func NewCollection(ctx context.Context, c *bookmarks.Collection) *Collection {
	return &Collection{
		Collection: c,
		ID:         c.UID,
		Href:       urls.AbsoluteURLContext(ctx, "/api/collections", c.UID).String(),
		Created:    c.Created,
		Updated:    c.Updated,
		Name:       c.Name,
		IsPinned:   c.IsPinned,
		IsDeleted:  tasks.DeleteCollectionTask.IsRunning(c.ID),

		// Filters
		Search:     c.Filters.Search,
		Title:      c.Filters.Title,
		Author:     c.Filters.Author,
		Site:       c.Filters.Site,
		Type:       c.Filters.Type,
		Labels:     c.Filters.Labels,
		ReadStatus: c.Filters.ReadStatus,
		IsMarked:   c.Filters.IsMarked,
		IsArchived: c.Filters.IsArchived,
		IsLoaded:   c.Filters.IsLoaded,
		HasErrors:  c.Filters.HasErrors,
		HasLabels:  c.Filters.HasLabels,
		RangeStart: c.Filters.RangeStart,
		RangeEnd:   c.Filters.RangeEnd,
	}
}
