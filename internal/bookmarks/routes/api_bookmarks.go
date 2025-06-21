// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/converter"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/annotate"
	"codeberg.org/readeck/readeck/pkg/forms"
	"codeberg.org/readeck/readeck/pkg/utils"
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

type (
	ctxAnnotationListKey     struct{}
	ctxBookmarkKey           struct{}
	ctxBookmarkListKey       struct{}
	ctxBookmarkListTaggerKey struct{}
	ctxBookmarkOrderKey      struct{}
	ctxBookmarkSyncListKey   struct{}
	ctxLabelKey              struct{}
	ctxLabelListKey          struct{}
	ctxSharedInfoKey         struct{}
	ctxFiltersKey            struct{}
	ctxDefaultLimitKey       struct{}
)

// bookmarkList renders a paginated list of the connected
// user bookmarks in JSON.
func (api *apiRouter) bookmarkList(w http.ResponseWriter, r *http.Request) {
	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)

	bl.Items = make([]bookmarkItem, len(bl.items))
	for i, item := range bl.items {
		bl.Items[i] = newBookmarkItem(r, item, ".")
	}

	server.SendPaginationHeaders(w, r, bl.Pagination)
	server.Render(w, r, http.StatusOK, bl.Items)
}

func (api *apiRouter) bookmarkSyncList(w http.ResponseWriter, r *http.Request) {
	bl := r.Context().Value(ctxBookmarkSyncListKey{}).(bookmarkSyncList)

	urlPrefix := urls.AbsoluteURL(r, "./..").String()
	for _, item := range bl {
		item.Href = urlPrefix + item.ID
	}
	server.Render(w, r, http.StatusOK, bl)
}

// bookmarkInfo renders a given bookmark items in JSON.
func (api *apiRouter) bookmarkInfo(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
	item := newBookmarkItem(r, b, "./..")
	item.Errors = b.Errors
	if err := item.setEmbed(); err != nil {
		server.Log(r).Error("", slog.Any("err", err))
	}

	if server.IsTurboRequest(r) {
		server.RenderTurboStream(w, r,
			"/bookmarks/components/card", "replace",
			"bookmark-card-"+b.UID, item, nil)
		return
	}

	server.Render(w, r, http.StatusOK, item)
}

// bookmarkArticle renders the article HTML content of a bookmark.
// Note that only the body's content is rendered.
func (api *apiRouter) bookmarkArticle(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)

	bi := newBookmarkItem(r, b, "")
	buf, err := bi.getArticle()
	if err != nil {
		server.Log(r).Error("", slog.Any("err", err))
	}

	if server.IsTurboRequest(r) {
		server.RenderTurboStream(w, r,
			"/bookmarks/components/content_block", "replace",
			"bookmark-content-"+b.UID, map[string]interface{}{
				"Item": bi,
				"HTML": buf,
				"Out":  w,
			},
			nil,
		)
		server.RenderTurboStream(w, r,
			"/bookmarks/components/sidebar", "replace",
			"bookmark-sidebar-"+b.UID, map[string]interface{}{
				"Item": bi,
			}, nil,
		)
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	io.Copy(w, buf)
}

func (api *apiRouter) bookmarkListFeed(w http.ResponseWriter, r *http.Request) {
	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)

	ctx := converter.WithURLReplacer(context.Background(), func(b *bookmarks.Bookmark) func(name string) string {
		return func(name string) string {
			return urls.AbsoluteURL(r, "/bm", b.FilePath, name).String()
		}
	})

	if err := converter.NewAtomExporter().Export(ctx, w, r, bl.items); err != nil {
		server.Err(w, r, err)
	}
}

// bookmarkExport renders a list of bookmarks in the requested export format.
func (api *apiRouter) bookmarkExport(w http.ResponseWriter, r *http.Request) {
	var exporter converter.Exporter
	switch chi.URLParam(r, "format") {
	case "epub":
		exp := converter.NewEPUBExporter()
		if collection, ok := r.Context().Value(ctxCollectionKey{}).(*bookmarks.Collection); ok {
			exp.Collection = collection
		}
		exporter = exp
	case "md.zip":
		// Support the special "md.zip" extension that forces the request for a zipfile
		// and then move on to the next, markdown, case.
		r.Header.Set("Accept", "application/zip")
		fallthrough
	case "md":
		exporter = converter.NewMarkdownExporter(
			urls.AbsoluteURL(r, "/"),
			urls.AbsoluteURL(r, "/bm/"),
		)
	}

	if exporter == nil {
		server.Status(w, r, http.StatusNotFound)
		return
	}

	var items []*bookmarks.Bookmark
	// Bookmark list or just one item
	if bl, ok := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList); ok {
		items = bl.items
	} else {
		if b, ok := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark); ok {
			items = []*bookmarks.Bookmark{b}
		}
	}

	if len(items) == 0 {
		server.Status(w, r, http.StatusNotFound)
		return
	}

	if err := exporter.Export(context.Background(), w, r, items); err != nil {
		server.Err(w, r, err)
	}
}

// bookmarkCreate creates a new bookmark.
func (api *apiRouter) bookmarkCreate(w http.ResponseWriter, r *http.Request) {
	f := newCreateForm(server.Locale(r), auth.GetRequestUser(r).ID, server.GetReqID(r))
	forms.Bind(f, r)

	if !f.IsValid() {
		server.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	var err error
	b, err := f.createBookmark()
	if err != nil {
		server.Err(w, r, err)
		return
	}

	w.Header().Add(
		"Location",
		urls.AbsoluteURL(r, ".", b.UID).String(),
	)
	w.Header().Add("bookmark-id", b.UID)
	server.NewLink(urls.AbsoluteURL(r, "/bookmarks", b.UID).String()).
		WithRel("alternate").
		WithType("text/html").
		Write(w)

	server.TextMsg(w, r, http.StatusAccepted, "Link submited")
}

// bookmarkUpdate updates an existing bookmark.
func (api *apiRouter) bookmarkUpdate(w http.ResponseWriter, r *http.Request) {
	f := newUpdateForm(server.Locale(r))
	forms.Bind(f, r)

	if !f.IsValid() {
		server.Render(w, r, http.StatusBadRequest, f)
		return
	}

	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)

	updated, err := f.update(b)
	if err != nil {
		server.Err(w, r, err)
		return
	}

	updated["href"] = urls.AbsoluteURL(r).String()

	// On a turbo request, we'll return the updated components.
	if server.IsTurboRequest(r) {
		item := newBookmarkItem(r, b, "./..")

		_, withTitle := updated["title"]
		_, withLabels := updated["labels"]
		_, withMarked := updated["is_marked"]
		_, withArchived := updated["is_archived"]
		_, withDeleted := updated["is_deleted"]
		_, withProgress := updated["read_progress"]

		if withTitle {
			server.RenderTurboStream(w, r,
				"/bookmarks/components/title_form", "replace",
				"bookmark-title-"+b.UID, item, nil)
		}
		if withLabels {
			server.RenderTurboStream(w, r,
				"/bookmarks/components/labels", "replace",
				"bookmark-label-list-"+b.UID, item, nil)
		}
		if withMarked || withArchived || withDeleted || withProgress {
			server.RenderTurboStream(w, r,
				"/bookmarks/components/actions", "replace",
				"bookmark-actions-"+b.UID, item, nil)
			server.RenderTurboStream(w, r,
				"/bookmarks/components/card", "replace",
				"bookmark-card-"+b.UID, item, nil)
		}
		if withMarked || withArchived {
			server.RenderTurboStream(w, r,
				"/bookmarks/components/bottom_actions", "replace",
				"bookmark-bottom-actions-"+b.UID, item, nil)
		}
		return
	}

	w.Header().Add(
		"Location",
		updated["href"].(string),
	)
	server.Render(w, r, http.StatusOK, updated)
}

// bookmarkDelete deletes a bookmark.
func (api *apiRouter) bookmarkDelete(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
	if err := b.Delete(); err != nil {
		server.Err(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// bookmarkShareLink returns a publicly shared bookmark link.
func (api *apiRouter) bookmarkShareLink(w http.ResponseWriter, r *http.Request) {
	info := r.Context().Value(ctxSharedInfoKey{}).(linkShareInfo)
	server.Render(w, r, http.StatusCreated, info)
}

// bookmarkShareEmail sends a bookmark by email.
func (api *apiRouter) bookmarkShareEmail(w http.ResponseWriter, r *http.Request) {
	info := r.Context().Value(ctxSharedInfoKey{}).(emailShareInfo)
	if !info.Form.IsValid() {
		server.Render(w, r, 0, info.Form) // status is already set by the middleware
		return
	}

	server.TextMsg(w, r, http.StatusOK, "Email sent to "+info.Form.Get("email").String())
}

// bookmarkResource is the route returning any resource
// from a given bookmark. The resource is extracted from
// the sidecar zip file of a bookmark.
// Note that for images, we'll use another route that is not
// authenticated and thus, much faster.
func (api *apiRouter) bookmarkResource(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
	p := path.Clean(chi.URLParam(r, "*"))

	r2 := new(http.Request)
	*r2 = *r
	r2.URL = new(url.URL)
	*r2.URL = *r.URL
	r2.URL.Path = p

	fs := zipfs.HTTPZipFile(b.GetFilePath())
	fs.ServeHTTP(w, r2)
}

// labelList returns the list of all labels.
func (api *apiRouter) labelList(w http.ResponseWriter, r *http.Request) {
	base := urls.AbsoluteURL(r, "/api/bookmarks")
	labels := r.Context().Value(ctxLabelListKey{}).([]*labelItem)
	for _, item := range labels {
		item.setURLs(base)
	}
	server.Render(w, r, http.StatusOK, labels)
}

// labelInfo return the information about a label.
func (api *apiRouter) labelInfo(w http.ResponseWriter, r *http.Request) {
	label := r.Context().Value(ctxLabelKey{}).(string)
	ds := bookmarks.Bookmarks.GetLabels().
		Where(
			goqu.C("user_id").Table("b").Eq(auth.GetRequestUser(r).ID),
			goqu.I("name").Eq(label),
		)

	var res labelItem
	exists, err := ds.ScanStruct(&res)
	if err != nil {
		server.Err(w, r, err)
	}
	if !exists {
		server.Status(w, r, http.StatusNotFound)
		return
	}
	res.setURLs(urls.AbsoluteURL(r, "/api/bookmarks"))

	server.Render(w, r, http.StatusOK, res)
}

func (api *apiRouter) labelUpdate(w http.ResponseWriter, r *http.Request) {
	label := r.Context().Value(ctxLabelKey{}).(string)
	f := newLabelForm(server.Locale(r))
	forms.Bind(f, r)

	if !f.IsValid() {
		server.Render(w, r, http.StatusBadRequest, f)
		return
	}

	ids, err := bookmarks.Bookmarks.RenameLabel(auth.GetRequestUser(r), label, f.Get("name").String())
	if err != nil {
		server.Err(w, r, err)
		return
	}
	if len(ids) == 0 {
		server.Status(w, r, http.StatusNotFound)
		return
	}
}

func (api *apiRouter) labelDelete(w http.ResponseWriter, r *http.Request) {
	label := r.Context().Value(ctxLabelKey{}).(string)

	ids, err := bookmarks.Bookmarks.RenameLabel(auth.GetRequestUser(r), label, "")
	if err != nil {
		server.Err(w, r, err)
		return
	}
	if len(ids) == 0 {
		server.Status(w, r, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *apiRouter) bookmarkAnnotations(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
	if b.Annotations != nil {
		server.Render(w, r, http.StatusOK, b.Annotations)
		return
	}

	server.Render(w, r, http.StatusOK, bookmarks.BookmarkAnnotations{})
}

func (api *apiRouter) annotationCreate(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
	f := newAnnotationForm(server.Locale(r))
	forms.Bind(f, r)
	if !f.IsValid() {
		server.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	bi := newBookmarkItem(r, b, "")
	annotation, err := f.addToBookmark(&bi)
	if err != nil {
		if errors.As(err, &annotate.ErrAnotate) {
			server.Msg(w, r, &server.Message{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
			})
		} else {
			server.Err(w, r, err)
		}
		return
	}

	w.Header().Add("Location", urls.AbsoluteURL(r, ".", annotation.ID).String())
	server.Render(w, r, http.StatusCreated, annotation)
}

func (api *apiRouter) annotationUpdate(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
	id := chi.URLParam(r, "id")
	if b.Annotations == nil {
		server.Status(w, r, http.StatusNotFound)
		return
	}

	if b.Annotations.Get(id) == nil {
		server.Status(w, r, http.StatusNotFound)
		return
	}

	f := newAnnotationUpdateForm(server.Locale(r))
	forms.Bind(f, r)
	if !f.IsValid() {
		server.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	annotation := b.Annotations.Get(id)
	annotation.Color = f.Get("color").String()
	update := map[string]interface{}{
		"annotations": b.Annotations,
	}
	err := b.Update(update)
	if err != nil {
		server.Err(w, r, err)
		return
	}

	server.Render(w, r, http.StatusOK, update)
}

func (api *apiRouter) annotationDelete(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
	id := chi.URLParam(r, "id")
	if b.Annotations == nil {
		server.Status(w, r, http.StatusNotFound)
		return
	}
	if b.Annotations.Get(id) == nil {
		server.Status(w, r, http.StatusNotFound)
		return
	}

	b.Annotations.Delete(id)
	err := b.Update(map[string]interface{}{
		"annotations": b.Annotations,
	})
	if err != nil {
		server.Err(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *apiRouter) annotationList(w http.ResponseWriter, r *http.Request) {
	al := r.Context().Value(ctxAnnotationListKey{}).(annotationList)

	server.SendPaginationHeaders(w, r, al.Pagination)
	server.Render(w, r, 200, al.Items)
}

// withBookmark returns a router that will fetch a bookmark and add it into the
// request's context. It also deals with if-modified-since header.
func (api *apiRouter) withBookmark(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := chi.URLParam(r, "uid")

		b, err := bookmarks.Bookmarks.GetOne(
			goqu.C("uid").Eq(uid),
			goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
		)
		if err != nil {
			server.Status(w, r, http.StatusNotFound)
			return
		}
		ctx := context.WithValue(r.Context(), ctxBookmarkKey{}, b)

		if b.State == bookmarks.StateLoaded {
			server.WriteLastModified(w, r, b, auth.GetRequestUser(r))
			server.WriteEtag(w, r, b, auth.GetRequestUser(r))
		}

		w.Header().Add("bookmark-id", b.UID)
		server.NewLink(urls.AbsoluteURL(r, "/bookmarks", b.UID).String()).
			WithRel("alternate").
			WithType("text/html").
			Write(w)

		server.WithCaching(next).ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withBookmarkFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := chi.URLParam(r, "filter")
		filters := newFilterForm(server.Locale(r))

		switch filter {
		case "unread":
			filters.setArchived(false)
		case "archives":
			filters.setArchived(true)
		case "favorites":
			filters.setMarked(true)
		case "articles":
			filters.setType("article")
		case "pictures":
			filters.setType("photo")
		case "videos":
			filters.setType("video")
		}

		next.ServeHTTP(w, r.WithContext(filters.saveContext(r.Context())))
	})
}

func (api *apiRouter) withLabel(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		label, err := url.QueryUnescape(chi.URLParam(r, "label"))
		if err != nil {
			server.Err(w, r, err)
			return
		}

		ctx := context.WithValue(r.Context(), ctxLabelKey{}, label)

		filters := newFilterForm(server.Locale(r))
		filters.Get("labels").Set(strconv.Quote(label))
		ctx = filters.saveContext(ctx)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withDefaultLimit(limit int) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxDefaultLimitKey{}, limit)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (api *apiRouter) withoutPagination(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := newContextFilterForm(r.Context(), server.Locale(r))
		f.noPagination = true
		next.ServeHTTP(w, r.WithContext(f.saveContext(r.Context())))
	})
}

func (api *apiRouter) withFixedLimit(limit uint) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f := newContextFilterForm(r.Context(), server.Locale(r))
			f.fixedLimit = limit
			next.ServeHTTP(w, r.WithContext(f.saveContext(r.Context())))
		})
	}
}

func (api *apiRouter) withCollectionFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c *bookmarks.Collection
		var ok bool
		var err error
		ctx := r.Context()
		c, ok = ctx.Value(ctxCollectionKey{}).(*bookmarks.Collection)
		if !ok {
			// No collection in context, let's see if we have an ID
			uid := r.URL.Query().Get("collection")
			if uid == "" {
				next.ServeHTTP(w, r)
				return
			}

			c, err = bookmarks.Collections.GetOne(
				goqu.C("uid").Eq(uid),
				goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
			)
			if err != nil {
				server.Status(w, r, http.StatusNotFound)
				return
			}
			ctx = context.WithValue(r.Context(), ctxCollectionKey{}, c)
		}

		// Apply filters
		f := newCollectionForm(server.Locale(r), r)
		f.setCollection(c)
		filters := newContextFilterForm(r.Context(), server.Locale(r))
		f.setFilters(filters)
		ctx = filters.saveContext(ctx)

		if ctx.Value(ctxBookmarkOrderKey{}) == nil {
			ctx = context.WithValue(ctx, ctxBookmarkOrderKey{}, orderExpressionList{goqu.T("b").Col("created").Desc()})
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withBookmarkOrdering(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := newBookmarkOrderForm()
		forms.BindURL(f, r)

		order := f.toOrderedExpressions()
		ctx := r.Context()
		if order != nil {
			ctx = context.WithValue(ctx, ctxBookmarkOrderKey{}, order)
		}

		// When we have a template context, we add the current order
		// and ordering options
		if c, ok := ctx.Value(ctxBaseContextKey{}).(server.TC); ok {
			f.addToTemplateContext(r, server.Locale(r), c)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withBookmarkList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := bookmarkList{}

		limit, ok := r.Context().Value(ctxDefaultLimitKey{}).(int)
		if !ok {
			limit = 50
		}

		pf := server.GetPageParams(r, limit)
		if pf == nil {
			server.Status(w, r, http.StatusNotFound)
			return
		}

		ds := bookmarks.Bookmarks.Query().
			Select(
				"b.id", "b.uid", "b.created", "b.updated", "b.published", "b.state",
				"b.url", "b.title", "b.domain", "b.site", "b.site_name", "b.authors",
				"b.lang", "b.dir", "b.type", "b.is_marked", "b.is_archived", "b.read_progress",
				"b.labels", "b.description", "b.word_count", "b.duration", "b.file_path", "b.files").
			Where(
				goqu.C("user_id").Table("b").Eq(auth.GetRequestUser(r).ID),
			)

		ds = ds.Order(goqu.I("created").Desc())

		// Filters (search and other filterForm)
		filterForm := newContextFilterForm(r.Context(), server.Locale(r))
		forms.BindURL(filterForm, r)

		if filterForm.IsValid() {
			filters := bookmarks.NewFiltersFromForm(filterForm)
			filters.UpdateForm(filterForm)
			ds = filters.ToSelectDataSet(ds)
		}

		// Filtering by ids. In this case we include all the given IDs and we sort the
		// result according to the IDs order.
		if !filterForm.Get("id").IsNil() {
			ids := filterForm.Get("id").Value().([]string)
			ds = ds.Where(goqu.C("uid").Table("b").In(ids))

			orderging := goqu.Case().Value(goqu.C("uid").Table("b"))
			for i, x := range ids {
				orderging = orderging.When(x, goqu.Cast(goqu.V(i), "NUMERIC"))
			}
			ds = ds.Order(orderging.Asc())
		}

		ds = ds.
			Limit(uint(pf.Limit())).
			Offset(uint(pf.Offset()))

		// Apply sorting given by a query string
		if order, ok := r.Context().Value(ctxBookmarkOrderKey{}).(orderExpressionList); ok {
			ds = ds.Order(order...)
		}

		// If pagination is disabled, remove all limits.
		if filterForm.noPagination {
			ds = ds.ClearLimit().ClearOffset()
		}

		// If fixed limit is set, override limit and offset
		if filterForm.fixedLimit > 0 {
			ds = ds.Limit(uint(filterForm.fixedLimit)).Offset(0)
		}

		var count int64
		var err error
		if count, err = ds.ClearOrder().ClearLimit().ClearOffset().Count(); err != nil {
			if errors.Is(err, bookmarks.ErrBookmarkNotFound) {
				server.TextMsg(w, r, http.StatusNotFound, "not found")
			} else {
				server.Err(w, r, err)
			}
			return
		}

		res.items = []*bookmarks.Bookmark{}
		if err = ds.ScanStructs(&res.items); err != nil {
			server.Err(w, r, err)
			return
		}

		res.Pagination = server.NewPagination(r, int(count), pf.Limit(), pf.Offset())

		ctx := filterForm.saveContext(r.Context())
		ctx = context.WithValue(ctx, ctxBookmarkListKey{}, res)

		taggers := []server.Etagger{res}
		t, ok := r.Context().Value(ctxBookmarkListTaggerKey{}).([]server.Etagger)
		if ok {
			taggers = append(taggers, t...)
		}

		if r.Method == http.MethodGet {
			server.WriteEtag(w, r, taggers...)
		}
		server.WithCaching(next).ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withBookmarkSyncList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ds := bookmarks.Bookmarks.Query().
			Select("b.uid", "b.created", "b.updated").
			Where(goqu.C("user_id").Table("b").Eq(auth.GetRequestUser(r).ID)).
			Order(
				goqu.I("updated").Desc(),
				goqu.I("created").Desc(),
			)

		res := bookmarkSyncList{}

		if err := ds.ScanStructs(&res); err != nil {
			server.Err(w, r, err)
			return
		}

		ctx := context.WithValue(r.Context(), ctxBookmarkSyncListKey{}, res)
		taggers := []server.Etagger{res}
		server.WriteEtag(w, r, taggers...)

		server.WithCaching(next).ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withAnnotationList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := annotationList{}

		limit, ok := r.Context().Value(ctxDefaultLimitKey{}).(int)
		if !ok {
			limit = 50
		}

		pf := server.GetPageParams(r, limit)
		if pf == nil {
			server.Status(w, r, http.StatusNotFound)
			return
		}

		ds := bookmarks.Bookmarks.GetAnnotations().
			Where(
				goqu.C("user_id").Table("b").Eq(auth.GetRequestUser(r).ID),
			)

		ds = ds.
			Limit(uint(pf.Limit())).
			Offset(uint(pf.Offset())).
			Order(goqu.I("annotation_created").Desc())

		var count int64
		var err error

		if count, err = ds.ClearOrder().ClearLimit().ClearOffset().Count(); err != nil {
			server.Err(w, r, err)
			return
		}

		res.Pagination = server.NewPagination(r, int(count), pf.Limit(), pf.Offset())

		res.items = []*bookmarks.AnnotationQueryResult{}
		if err = ds.ScanStructs(&res.items); err != nil {
			server.Err(w, r, err)
			return
		}
		res.Items = make([]annotationItem, len(res.items))
		for i, item := range res.items {
			res.Items[i] = newAnnotationItem(r, item)
		}

		ctx := context.WithValue(r.Context(), ctxAnnotationListKey{}, res)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withLabelList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ds := bookmarks.Bookmarks.GetLabels().
			Where(
				goqu.C("user_id").Table("b").Eq(auth.GetRequestUser(r).ID),
			)

		f := newLabelSearchForm(server.Locale(r))
		forms.BindURL(f, r)
		if f.Get("q").String() != "" {
			q := strings.ReplaceAll(f.Get("q").String(), "*", "%")
			ds = ds.Where(goqu.I("name").ILike(q))
		}

		res := []*labelItem{}
		if err := ds.ScanStructs(&res); err != nil {
			server.Err(w, r, err)
			return
		}

		ctx := context.WithValue(r.Context(), ctxLabelListKey{}, res)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withShareLink(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Disable HTTP caching
		server.WriteLastModified(w, r)
		server.WriteEtag(w, r)

		b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
		if b.State != bookmarks.StateLoaded {
			server.Err(w, r, errors.New("bookmark not loaded yet"))
			return
		}

		expires := time.Now().Round(time.Minute).Add(
			time.Duration(configs.Config.Bookmarks.PublicShareTTL) * time.Hour,
		)

		rr, err := bookmarks.EncodeID(uint64(b.ID), expires)
		if err != nil {
			server.Err(w, r, err)
			return
		}

		info := linkShareInfo{
			URL:     urls.AbsoluteURL(r, "/@b", rr).String(),
			Expires: expires,
			Title:   b.Title,
			ID:      b.UID,
		}
		ctx := context.WithValue(r.Context(), ctxSharedInfoKey{}, info)
		w.Header().Set("Location", info.URL)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *apiRouter) withShareEmail(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Disable HTTP caching
		server.WriteLastModified(w, r)
		server.WriteEtag(w, r)

		b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
		if b.State != bookmarks.StateLoaded {
			server.Err(w, r, errors.New("bookmark not loaded yet"))
			return
		}

		info := emailShareInfo{
			Form:  newShareForm(server.Locale(r)),
			Title: b.Title,
			ID:    b.UID,
		}

		if r.Method == http.MethodPost {
			forms.Bind(info.Form, r)

			if info.Form.IsValid() {
				info.Error = info.Form.sendBookmark(r, b)
			}
			if info.Error != nil {
				server.Log(r).Error("could not send email", slog.Any("err", info.Error))
			}
			if !info.Form.IsValid() {
				w.WriteHeader(http.StatusUnprocessableEntity)
			}
		}

		ctx := context.WithValue(r.Context(), ctxSharedInfoKey{}, info)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bookmarkList is a paginated list of BookmarkItem instances.
type bookmarkList struct {
	items      []*bookmarks.Bookmark
	Pagination server.Pagination
	Items      []bookmarkItem
}

func (bl bookmarkList) UpdateEtag(h hash.Hash) {
	for i := range bl.items {
		io.WriteString(h, bl.items[i].UID+strconv.FormatInt(bl.items[i].Updated.UTC().Unix(), 10))
	}
}

// bookmarkItem is a serialized bookmark instance that can
// be used directly on the API or by an HTML template.
type bookmarkItem struct {
	*bookmarks.Bookmark `json:"-"`

	ID              string                        `json:"id"`
	Href            string                        `json:"href"`
	Created         time.Time                     `json:"created"`
	Updated         time.Time                     `json:"updated"`
	State           bookmarks.BookmarkState       `json:"state"`
	Loaded          bool                          `json:"loaded"`
	URL             string                        `json:"url"`
	Title           string                        `json:"title"`
	SiteName        string                        `json:"site_name"`
	Site            string                        `json:"site"`
	Published       *time.Time                    `json:"published,omitempty"`
	Authors         []string                      `json:"authors"`
	Lang            string                        `json:"lang"`
	TextDirection   string                        `json:"text_direction"`
	DocumentType    string                        `json:"document_type"`
	Type            string                        `json:"type"`
	HasArticle      bool                          `json:"has_article"`
	Description     string                        `json:"description"`
	OmitDescription *bool                         `json:"omit_description,omitempty"`
	IsDeleted       bool                          `json:"is_deleted"`
	IsMarked        bool                          `json:"is_marked"`
	IsArchived      bool                          `json:"is_archived"`
	Labels          []string                      `json:"labels"`
	ReadProgress    int                           `json:"read_progress"`
	ReadAnchor      string                        `json:"read_anchor,omitempty"`
	Annotations     bookmarks.BookmarkAnnotations `json:"-"`
	Resources       map[string]*bookmarkFile      `json:"resources"`
	Embed           string                        `json:"embed,omitempty"`
	EmbedHostname   string                        `json:"embed_domain,omitempty"`
	Errors          []string                      `json:"errors,omitempty"`
	Links           bookmarks.BookmarkLinks       `json:"links,omitempty"`
	WordCount       int                           `json:"word_count,omitempty"`
	ReadingTime     int                           `json:"reading_time,omitempty"`

	baseURL            *url.URL
	mediaURL           *url.URL
	annotationTag      string
	annotationCallback func(id string, n *html.Node, index int, color string)
}

// bookmarkFile is a file attached to a bookmark. If the file is
// an image, the "Width" and "Height" values will be filled.
type bookmarkFile struct {
	Src    string `json:"src"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// newBookmarkItem builds a BookmarkItem from a Bookmark instance.
func newBookmarkItem(r *http.Request, b *bookmarks.Bookmark, base string) bookmarkItem {
	res := bookmarkItem{
		Bookmark:      b,
		ID:            b.UID,
		Href:          urls.AbsoluteURL(r, base, b.UID).String(),
		Created:       b.Created,
		Updated:       b.Updated,
		State:         b.State,
		Loaded:        b.State != bookmarks.StateLoading,
		URL:           b.URL,
		Title:         b.Title,
		SiteName:      b.SiteName,
		Site:          b.Site,
		Published:     b.Published,
		Authors:       b.Authors,
		Lang:          b.Lang,
		TextDirection: b.TextDirection,
		DocumentType:  b.DocumentType,
		Description:   b.Description,
		IsDeleted:     tasks.DeleteBookmarkTask.IsRunning(b.ID),
		IsMarked:      b.IsMarked,
		IsArchived:    b.IsArchived,
		ReadProgress:  b.ReadProgress,
		ReadAnchor:    b.ReadAnchor,
		WordCount:     b.WordCount,
		ReadingTime:   b.ReadingTime(),
		Labels:        make([]string, 0),
		Annotations:   b.Annotations,
		Resources:     make(map[string]*bookmarkFile),
		Links:         b.Links,

		baseURL:       urls.AbsoluteURL(r, "/"),
		annotationTag: "rd-annotation",
		annotationCallback: func(id string, n *html.Node, index int, color string) {
			if index == 0 {
				dom.SetAttribute(n, "id", "annotation-"+id)
			}
			if color == "" {
				color = "yellow"
			}
			dom.SetAttribute(n, "data-annotation-id-value", id)
			dom.SetAttribute(n, "data-annotation-color", color)
		},
	}

	// Set a relative media base URL when we're not querying the API.
	if !strings.HasPrefix(r.URL.EscapedPath(), urls.AbsoluteURL(r, "/api/").EscapedPath()) {
		res.baseURL.Scheme = ""
		res.baseURL.Host = ""
	}

	res.mediaURL = res.baseURL.JoinPath("/bm", b.FilePath)

	if b.Labels != nil {
		res.Labels = b.Labels
	}

	switch res.DocumentType {
	case "video":
		res.Type = "video"
	case "image", "photo":
		res.Type = "photo"
	default:
		res.Type = "article"
	}

	// Check if description is somewhere at the beginning of the content.
	// Only when we have a text content (full bookmark info)
	if b.Text != "" && b.Description != "" {
		omitDescription := strings.Contains(
			utils.ToLowerTextOnly(b.Text[:min(len(b.Text), int(len(b.Description)*3))]),
			utils.ToLowerTextOnly(b.Description),
		)
		res.OmitDescription = &omitDescription
	}

	// Files and resources
	for k, v := range b.Files {
		if path.Dir(v.Name) != "img" {
			continue
		}

		f := &bookmarkFile{
			Src: res.mediaURL.String() + "/" + v.Name,
		}

		if v.Size != [2]int{0, 0} {
			f.Width = v.Size[0]
			f.Height = v.Size[1]
		}
		res.Resources[k] = f
	}

	if v, ok := b.Files["props"]; ok {
		res.Resources["props"] = &bookmarkFile{Src: urls.AbsoluteURL(r, base, b.UID, "x", v.Name).String()}
	}
	if v, ok := b.Files["log"]; ok {
		res.Resources["log"] = &bookmarkFile{Src: urls.AbsoluteURL(r, base, b.UID, "x", v.Name).String()}
	}
	if _, ok := b.Files["article"]; ok {
		res.HasArticle = true
		res.Resources["article"] = &bookmarkFile{Src: urls.AbsoluteURL(r, base, b.UID, "article").String()}
	}

	return res
}

// getArticle calls [converter.HTMLConverter.GetArticle]
// with URL replacer and annotation tag properly setup.
func (bi bookmarkItem) getArticle() (*strings.Reader, error) {
	ctx := context.Background()

	// Set resource URL replacer, for images
	ctx = converter.WithURLReplacer(ctx, func(_ *bookmarks.Bookmark) func(name string) string {
		return func(name string) string {
			return bi.mediaURL.JoinPath(name).String()
		}
	})

	// Set annotation tag and callback
	ctx = converter.WithAnnotationTag(ctx, bi.annotationTag, bi.annotationCallback)

	// Get article from converter
	return converter.HTMLConverter{}.GetArticle(
		ctx,
		bi.Bookmark,
	)
}

// setEmbed sets the Embed and EmbedHostname item properties.
// The original embed value must be an iframe. We extract the "src"
// URL and store its hostname that we can later use in the CSP policy.
// A special case for youtube for which we force
// the use of youtube-nocookie.com.
func (bi *bookmarkItem) setEmbed() error {
	if bi.Bookmark.Embed == "" || bi.EmbedHostname != "" {
		return nil
	}
	node, err := html.Parse(strings.NewReader(bi.Bookmark.Embed))
	if err != nil {
		return err
	}
	embed := dom.QuerySelector(node, "iframe,hls,video")
	if embed == nil {
		return nil
	}

	src, err := url.Parse(dom.GetAttribute(embed, "src"))
	if err != nil {
		return err
	}

	// Force youtube iframes to use the "nocookie" variant.
	if src.Host == "www.youtube.com" {
		src.Host = "www.youtube-nocookie.com"
	}

	switch dom.TagName(embed) {
	case "iframe":
		// Set the embed block and its hostname
		dom.SetAttribute(embed, "src", src.String())
		dom.SetAttribute(embed, "credentialless", "true")
		dom.SetAttribute(embed, "allowfullscreen", "true")
		dom.SetAttribute(embed, "referrerpolicy", "no-referrer")
		dom.SetAttribute(embed, "sandbox", "allow-scripts allow-same-origin")
		dom.SetAttribute(embed, "allow", "accelerometer 'none'; ambient-light-sensor 'none'; autoplay 'none'; battery 'none'; browsing-topics 'none'; camera 'none'; display-capture 'none'; domain-agent 'none'; document-domain 'none'; encrypted-media 'none'; execution-while-not-rendered 'none'; execution-while-out-of-viewport ''; gamepad 'none'; geolocation 'none'; gyroscope 'none'; hid 'none'; identity-credentials-get 'none'; idle-detection 'none'; local-fonts 'none'; magnetometer 'none'; microphone 'none'; midi 'none'; otp-credentials 'none'; payment 'none'; publickey-credentials-create 'none'; publickey-credentials-get 'none'; screen-wake-lock 'none'; serial 'none'; speaker-selection 'none'; usb 'none'; window-management 'none'; xr-spatial-tracking 'none'")
		dom.SetAttribute(embed, "csp", "sandbox allow-scripts allow-same-origin")
		bi.Embed = dom.OuterHTML(embed)
		bi.EmbedHostname = src.Hostname()
	case "hls":
		if bi.Resources["image"] == nil {
			return nil
		}
		playerURL := bi.baseURL.JoinPath("/videoplayer")
		playerURL.RawQuery = url.Values{
			"type": {"hls"},
			"src":  {src.String()},
			"w":    {strconv.Itoa(bi.Resources["image"].Width)},
			"h":    {strconv.Itoa(bi.Resources["image"].Height)},
		}.Encode()
		bi.Embed = fmt.Sprintf(
			`<iframe src="%s" width="%d" height="%d" frameborder="0" scrolling="no" sandbox="allow-scripts"></iframe>`,
			playerURL,
			bi.Resources["image"].Width,
			bi.Resources["image"].Height,
		)
	case "video":
		if bi.Resources["image"] == nil {
			return nil
		}
		playerURL := bi.baseURL.JoinPath("/videoplayer")
		playerURL.RawQuery = url.Values{
			"src": {src.String()},
			"w":   {strconv.Itoa(bi.Resources["image"].Width)},
			"h":   {strconv.Itoa(bi.Resources["image"].Height)},
		}.Encode()
		bi.Embed = fmt.Sprintf(
			`<iframe src="%s" width="%d" height="%d" frameborder="0" scrolling="no" sandbox="allow-scripts"></iframe>`,
			playerURL,
			bi.Resources["image"].Width,
			bi.Resources["image"].Height,
		)
	}

	return nil
}

type bookmarkSyncList []*bookmarkSyncItem

func (bl bookmarkSyncList) UpdateEtag(h hash.Hash) {
	for _, b := range bl {
		io.WriteString(h, b.ID+strconv.FormatInt(b.Updated.UTC().Unix(), 10))
	}
}

type bookmarkSyncItem struct {
	ID      string    `json:"id" db:"uid"`
	Href    string    `json:"href" db:"-"`
	Created time.Time `json:"created" db:"created"`
	Updated time.Time `json:"updated" db:"updated"`
}

type labelItem struct {
	Name          labelString `db:"name"  json:"name"`
	Count         int         `db:"count" json:"count"`
	Href          string      `db:"-"     json:"href"`
	HrefBookmarks string      `db:"-"     json:"href_bookmarks"`
}

func (i *labelItem) setURLs(bookmarkBase *url.URL) {
	i.Href = bookmarkBase.JoinPath("labels", i.Name.Path()).String()
	i.HrefBookmarks = bookmarkBase.String() + "?" + url.Values{"labels": []string{strconv.Quote(string(i.Name))}}.Encode()
}

type labelString string

func (s labelString) Path() string {
	return url.QueryEscape(string(s))
}

type annotationList struct {
	items      []*bookmarks.AnnotationQueryResult
	Pagination server.Pagination
	Items      []annotationItem
}

type annotationItem struct {
	ID               string    `json:"id"`
	Href             string    `json:"href"`
	Text             string    `json:"text"`
	Created          time.Time `json:"created"`
	Color            string    `json:"color"`
	BookmarkID       string    `json:"bookmark_id"`
	BookmarkHref     string    `json:"bookmark_href"`
	BookmarkURL      string    `json:"bookmark_url"`
	BookmarkTitle    string    `json:"bookmark_title"`
	BookmarkSiteName string    `json:"bookmark_site_name"`
}

func newAnnotationItem(r *http.Request, a *bookmarks.AnnotationQueryResult) annotationItem {
	res := annotationItem{
		ID:               a.ID,
		Href:             urls.AbsoluteURL(r, "/api/bookmarks", a.Bookmark.UID, "annotations", a.ID).String(),
		Text:             a.Text,
		Created:          time.Time(a.Created),
		Color:            a.Color,
		BookmarkID:       a.Bookmark.UID,
		BookmarkHref:     urls.AbsoluteURL(r, "/api/bookmarks", a.Bookmark.UID).String(),
		BookmarkURL:      a.Bookmark.URL,
		BookmarkTitle:    a.Bookmark.Title,
		BookmarkSiteName: a.Bookmark.SiteName,
	}
	return res
}

type linkShareInfo struct {
	URL     string    `json:"url"`
	Expires time.Time `json:"expires"`
	Title   string    `json:"title"`
	ID      string    `json:"id"`
}

type emailShareInfo struct {
	Form  *shareForm
	Title string
	ID    string
	Error error
}
