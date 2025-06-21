// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type (
	ctxCollectionListKey struct{}
	ctxCollectionKey     struct{}
)

func (api *apiRouter) collectionList(w http.ResponseWriter, r *http.Request) {
	cl := r.Context().Value(ctxCollectionListKey{}).(collectionList)

	cl.Items = make([]collectionItem, len(cl.items))
	for i, item := range cl.items {
		cl.Items[i] = newCollectionItem(r, item, ".")
	}

	api.srv.SendPaginationHeaders(w, r, cl.Pagination)
	api.srv.Render(w, r, http.StatusOK, cl.Items)
}

func (api *apiRouter) collectionInfo(w http.ResponseWriter, r *http.Request) {
	c := r.Context().Value(ctxCollectionKey{}).(*bookmarks.Collection)
	item := newCollectionItem(r, c, "./..")

	api.srv.Render(w, r, http.StatusOK, item)
}

func (api *apiRouter) collectionCreate(w http.ResponseWriter, r *http.Request) {
	f := newCollectionForm(api.srv.Locale(r), r)

	forms.Bind(f, r)
	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	c, err := f.createCollection(auth.GetRequestUser(r).ID)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	w.Header().Set("Location", urls.AbsoluteURL(r, ".", c.UID).String())
	api.srv.TextMessage(w, r, http.StatusCreated, "Collection created")
}

func (api *apiRouter) collectionUpdate(w http.ResponseWriter, r *http.Request) {
	c := r.Context().Value(ctxCollectionKey{}).(*bookmarks.Collection)

	f := newCollectionForm(api.srv.Locale(r), r)
	f.setCollection(c)
	forms.Bind(f, r)

	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	updated, err := f.updateCollection(c)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	api.srv.Render(w, r, http.StatusOK, updated)
}

func (api *apiRouter) collectionDelete(w http.ResponseWriter, r *http.Request) {
	c := r.Context().Value(ctxCollectionKey{}).(*bookmarks.Collection)
	if err := c.Delete(); err != nil {
		api.srv.Error(w, r, err)
		return
	}

	api.srv.Status(w, r, http.StatusNoContent)
}

func (api *apiRouter) withColletionList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := collectionList{}

		pf := api.srv.GetPageParams(r, 30)
		if pf == nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}

		ds := bookmarks.Collections.Query().
			Select(
				"c.id", "c.uid", "c.user_id", "c.created", "c.updated",
				"c.name", "c.is_pinned", "c.filters",
			).
			Where(
				goqu.C("user_id").Table("c").Eq(auth.GetRequestUser(r).ID),
			)

		ds = ds.Order(goqu.I("name").Asc()).
			Limit(uint(pf.Limit())).
			Offset(uint(pf.Offset()))

		var count int64
		var err error
		if count, err = ds.ClearOrder().ClearLimit().ClearOffset().Count(); err != nil {
			if errors.Is(err, bookmarks.ErrCollectionNotFound) {
				api.srv.TextMessage(w, r, http.StatusNotFound, "not found")
			} else {
				api.srv.Error(w, r, err)
			}
			return
		}

		res.items = []*bookmarks.Collection{}
		if err := ds.ScanStructs(&res.items); err != nil {
			api.srv.Error(w, r, err)
			return
		}

		res.Pagination = api.srv.NewPagination(r, int(count), pf.Limit(), pf.Offset())

		ctx := context.WithValue(r.Context(), ctxCollectionListKey{}, res)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withCollection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := chi.URLParam(r, "uid")

		c, err := bookmarks.Collections.GetOne(
			goqu.C("uid").Eq(uid),
			goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
		)
		if err != nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}

		ctx := context.WithValue(r.Context(), ctxCollectionKey{}, c)
		ctx = context.WithValue(ctx, ctxBookmarkListTaggerKey{}, []server.Etagger{c})

		if ctx.Value(ctxBookmarkOrderKey{}) == nil {
			ctx = context.WithValue(ctx, ctxBookmarkOrderKey{}, orderExpressionList{goqu.T("b").Col("created").Desc()})
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type collectionList struct {
	items      []*bookmarks.Collection
	Pagination server.Pagination
	Items      []collectionItem
}

type collectionItem struct {
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

func newCollectionItem(r *http.Request, c *bookmarks.Collection, base string) collectionItem {
	return collectionItem{
		Collection: c,
		ID:         c.UID,
		Href:       urls.AbsoluteURL(r, base, c.UID).String(),
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
