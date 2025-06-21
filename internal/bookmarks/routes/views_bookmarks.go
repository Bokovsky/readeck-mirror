// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/forms"
	"codeberg.org/readeck/readeck/pkg/http/csp"
)

type (
	ctxBaseContextKey struct{}
)

const listDefaultLimit = 36

func (h *viewsRouter) withBaseContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count, err := bookmarks.Bookmarks.CountAll(auth.GetRequestUser(r))
		if err != nil {
			h.srv.Error(w, r, err)
			return
		}

		c := server.TC{
			"Count": count,
		}

		ctx := context.WithValue(r.Context(), ctxBaseContextKey{}, c)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *viewsRouter) bookmarkList(w http.ResponseWriter, r *http.Request) {
	f := newCreateForm(h.srv.Locale(r), auth.GetRequestUser(r).ID, h.srv.GetReqID(r))
	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	ctx["MaybeSearch"] = false

	// POST => create a new bookmark
	if r.Method == http.MethodPost {
		forms.Bind(f, r)
		if f.IsValid() {
			if b, err := f.createBookmark(); err != nil {
				h.srv.Log(r).Error("", slog.Any("err", err))
			} else {
				redir := []string{"/bookmarks"}
				if h.srv.IsTurboRequest(r) {
					redir = append(redir, "unread")
				} else {
					redir = append(redir, b.UID)
				}
				h.srv.Redirect(w, r, redir...)
				return

			}
		}

		// If the URL is not valid, set MaybeSearch so we can suggest it later
		if len(f.Get("url").Errors()) > 0 && errors.Is(f.Get("url").Errors()[0], forms.ErrInvalidURL) {
			// User entered a wrong URL, we can mark it.
			ctx["MaybeSearch"] = true
		}

		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	// Retrieve the bookmark list
	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)

	bl.Items = make([]bookmarkItem, len(bl.items))
	for i, item := range bl.items {
		bl.Items[i] = newBookmarkItem(r, item, ".")
	}

	tr := h.srv.Locale(r)

	ctx["Form"] = f
	ctx["Pagination"] = bl.Pagination
	ctx["Bookmarks"] = bl.Items
	ctx["Filters"] = newContextFilterForm(r.Context(), tr)
	title := tr.Gettext("All your Bookmarks")

	if filters, ok := r.Context().Value(ctxFiltersKey{}).(*filterForm); ok {
		ctx["Filters"] = filters
		if filters.IsActive() {
			title = tr.Gettext("Bookmark Search")
		} else {
			switch filters.title {
			case filtersTitleUnread:
				title = tr.Gettext("Unread Bookmarks")
			case filtersTitleArchived:
				title = tr.Gettext("Archived Bookmarks")
			case filtersTitleFavorites:
				title = tr.Gettext("Favorite Bookmarks")
			case filtersTitleArticles:
				title = tr.Gettext("Articles")
			case filtersTitlePictures:
				title = tr.Gettext("Pictures")
			case filtersTitleVideos:
				title = tr.Gettext("Videos")
			}
		}
	}
	ctx["PageTitle"] = title

	h.srv.RenderTemplate(w, r, 200, "/bookmarks/index", ctx)
}

func (h *viewsRouter) bookmarkInfo(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
	user := auth.GetRequestUser(r)
	item := newBookmarkItem(r, b, "../bookmarks")
	if err := item.setEmbed(); err != nil {
		h.srv.Log(r).Error("", slog.Any("err", err))
	}
	item.Errors = b.Errors

	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	ctx["Item"] = item

	var err error
	ctx["HTML"], err = item.getArticle()
	if err != nil {
		h.srv.Log(r).Error("", slog.Any("err", err))
	}

	// Load bookmark debug information if the user needs them.
	if user.Settings.DebugInfo {
		c, err := b.OpenContainer()
		if err != nil && !os.IsNotExist(err) {
			h.srv.Error(w, r, err)
			return
		}

		if c != nil {
			defer c.Close()

			for k, x := range map[string]string{
				"_props": "props.json",
				"_log":   "log",
			} {
				if r, err := c.GetFile(x); err != nil {
					ctx[k] = err.Error()
				} else {
					ctx[k] = string(r)
				}
			}
		}
	}

	// Set CSP for video playback
	if item.Type == "video" && item.EmbedHostname != "" {
		policy := server.GetCSPHeader(r).Clone()
		policy.Add("frame-src", item.EmbedHostname)
		policy.Write(w.Header())
	}

	h.srv.RenderTemplate(w, r, 200, "/bookmarks/bookmark", ctx)
}

func (h *viewsRouter) bookmarkUpdate(w http.ResponseWriter, r *http.Request) {
	f := newUpdateForm(h.srv.Locale(r))
	forms.Bind(f, r)

	if !f.IsValid() {
		h.srv.Render(w, r, http.StatusBadRequest, f)
		return
	}

	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)

	if _, err := f.update(b); err != nil {
		h.srv.Error(w, r, err)
		return
	}

	redir := "/bookmarks/" + b.UID
	if f.Get("_to").String() != "" {
		redir = f.Get("_to").String()
	}

	h.srv.Redirect(w, r, redir)
}

func (h *viewsRouter) bookmarkDelete(w http.ResponseWriter, r *http.Request) {
	b := r.Context().Value(ctxBookmarkKey{}).(*bookmarks.Bookmark)
	f := newDeleteForm(h.srv.Locale(r))
	forms.Bind(f, r)

	if err := b.Update(map[string]interface{}{}); err != nil {
		h.srv.Error(w, r, err)
		return
	}

	if err := f.trigger(b); err != nil {
		h.srv.Error(w, r, err)
		return
	}

	redir := "/bookmarks"
	if f.Get("_to").String() != "" {
		redir = f.Get("_to").String()
	}

	h.srv.Redirect(w, r, redir)
}

func (h *viewsRouter) bookmarkShareLink(w http.ResponseWriter, r *http.Request) {
	info := r.Context().Value(ctxSharedInfoKey{}).(linkShareInfo)

	ctx := server.TC{
		"URL":     info.URL,
		"Expires": info.Expires,
		"Title":   info.Title,
		"ID":      info.ID,
	}

	if h.srv.IsTurboRequest(r) {
		h.srv.RenderTurboStream(w, r,
			"/bookmarks/components/share_link", "replace",
			"bookmark-share-"+info.ID, info, nil)
		return
	}

	h.srv.RenderTemplate(w, r, http.StatusCreated, "bookmarks/bookmark_share_link", ctx)
}

func (h *viewsRouter) bookmarkShareEmail(w http.ResponseWriter, r *http.Request) {
	info := r.Context().Value(ctxSharedInfoKey{}).(emailShareInfo)
	tc := server.TC{
		"Form":  info.Form,
		"Title": info.Title,
		"ID":    info.ID,
		"Sent":  false,
	}

	// Get format from query string
	if format := r.URL.Query().Get("format"); format != "" {
		info.Form.Get("format").Set(format)
	}

	// Set default email address when sending an EPUB
	if u := auth.GetRequestUser(r); u != nil && info.Form.Get("format").String() == "epub" && info.Form.Get("email").String() == "" {
		info.Form.Get("email").Set(u.Settings.EmailSettings.EpubTo)
	}

	if r.Method == http.MethodPost {
		tc["Sent"] = info.Error == nil && info.Form.IsValid()
	}

	if h.srv.IsTurboRequest(r) {
		h.srv.RenderTurboStream(w, r,
			"/bookmarks/components/share_email", "replace",
			"bookmark-share-"+info.ID, tc, nil)
		return
	}

	h.srv.RenderTemplate(w, r, http.StatusOK, "bookmarks/bookmark_share_email", tc)
}

func (h *viewsRouter) labelList(w http.ResponseWriter, r *http.Request) {
	base := urls.AbsoluteURL(r, "/bookmarks")
	base.Scheme = ""
	base.Host = ""
	labels := r.Context().Value(ctxLabelListKey{}).([]*labelItem)
	for _, item := range labels {
		item.setURLs(base)
	}

	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	ctx["Labels"] = labels

	h.srv.RenderTemplate(w, r, 200, "/bookmarks/labels", ctx)
}

func (h *viewsRouter) labelInfo(w http.ResponseWriter, r *http.Request) {
	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)
	label := r.Context().Value(ctxLabelKey{}).(string)

	if bl.Pagination.TotalCount == 0 {
		h.srv.Status(w, r, http.StatusNotFound)
		return
	}

	// POST, update label name
	if r.Method == http.MethodPost {
		f := newLabelForm(h.srv.Locale(r))
		forms.Bind(f, r)

		if f.IsValid() {
			_, err := bookmarks.Bookmarks.RenameLabel(auth.GetRequestUser(r), label, f.Get("name").String())
			if err != nil {
				h.srv.Error(w, r, err)
				return
			}

			// We can't use redirect here, since we must escape the label
			redir := urls.AbsoluteURL(r, "/bookmarks/labels/")
			redir.Path += url.QueryEscape(f.Get("name").String())
			w.Header().Set("Location", redir.String())
			w.WriteHeader(http.StatusSeeOther)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	bl.Items = make([]bookmarkItem, len(bl.items))
	for i, item := range bl.items {
		bl.Items[i] = newBookmarkItem(r, item, ".")
	}

	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	ctx["Label"] = label
	ctx["Pagination"] = bl.Pagination
	ctx["Bookmarks"] = bl.Items
	ctx["IsDeleted"] = tasks.DeleteLabelTask.IsRunning(fmt.Sprintf("%d@%s", auth.GetRequestUser(r).ID, label))

	h.srv.RenderTemplate(w, r, 200, "/bookmarks/label", ctx)
}

func (h *viewsRouter) labelDelete(w http.ResponseWriter, r *http.Request) {
	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)
	label := r.Context().Value(ctxLabelKey{}).(string)

	if bl.Pagination.TotalCount == 0 {
		h.srv.Status(w, r, http.StatusNotFound)
		return
	}

	f := newLabelDeleteForm(h.srv.Locale(r))
	forms.Bind(f, r)
	if err := f.trigger(auth.GetRequestUser(r), label); err != nil {
		h.srv.Error(w, r, err)
		return
	}

	// We can't use redirect here, since we must escape the label
	redir := urls.AbsoluteURL(r, "/bookmarks/labels/")
	redir.Path += url.QueryEscape(label)
	w.Header().Set("Location", redir.String())
	w.WriteHeader(http.StatusSeeOther)
}

func (h *viewsRouter) annotationList(w http.ResponseWriter, r *http.Request) {
	al := r.Context().Value(ctxAnnotationListKey{}).(annotationList)

	h.srv.SendPaginationHeaders(w, r, al.Pagination)

	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	ctx["Pagination"] = al.Pagination
	ctx["Annotations"] = al.Items

	h.srv.RenderTemplate(w, r, 200, "/bookmarks/annotation_list", ctx)
}

func (h *publicViewsRouter) withBookmark(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := chi.URLParam(r, "id")
		id, expires, err := bookmarks.DecodeID(data)
		if err != nil {
			h.srv.Log(r).Warn("shared bookmark", slog.Any("err", err))
			h.srv.Status(w, r, 404)
			return
		}

		expired := expires.Before(time.Now())
		status := http.StatusOK
		ct := server.TC{
			"Expired": expired,
		}

		if !expired {
			var bu struct {
				User     *users.User         `db:"u"`
				Bookmark *bookmarks.Bookmark `db:"b"`
			}
			ds := bookmarks.Bookmarks.
				Query().
				Join(goqu.T(users.TableName).As("u"), goqu.On(goqu.I("u.id").Eq(goqu.I("b.user_id")))).
				Where(
					goqu.I("b.id").Eq(id),
					goqu.I("b.state").Eq(bookmarks.StateLoaded),
				)
			found, err := ds.ScanStruct(&bu)

			if !found || err != nil {
				status = http.StatusNotFound
			} else {
				item := newBookmarkItem(r, bu.Bookmark, "../@b")
				if err := item.setEmbed(); err != nil {
					h.srv.Error(w, r, err)
					return
				}
				ct["Username"] = bu.User.Username
				ct["Item"] = item

				w.Header().Add("readeck-original", item.URL)
				server.NewLink(item.URL).WithRel("original").Write(w)
				server.NewLink(item.URL).WithRel("cite-as").Write(w)
				h.srv.WriteLastModified(w, r, bu.Bookmark)
				h.srv.WriteEtag(w, r, bu.Bookmark)
			}
		} else {
			status = http.StatusGone
		}

		ct["Status"] = status

		ctx := context.WithValue(r.Context(), ctxBaseContextKey{}, ct)
		h.srv.WithCaching(next).ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *publicViewsRouter) get(w http.ResponseWriter, r *http.Request) {
	ct := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	status := ct["Status"].(int)

	if status == http.StatusOK {
		item := ct["Item"].(bookmarkItem)
		article, err := item.getArticle()
		if err != nil {
			h.srv.Error(w, r, err)
			return
		}

		ct["HTML"] = article

		// Harden CSP
		policy := server.GetCSPHeader(r).Clone()
		policy.Set("connect-src", csp.None)
		policy.Set("form-action", csp.None)

		// Relax CSP for video playback
		if item.Type == "video" && item.EmbedHostname != "" {
			policy.Add("frame-src", item.EmbedHostname)
		}
		policy.Write(w.Header())
	}

	h.srv.RenderTemplate(w, r, status, "bookmarks/bookmark_public", ct)
}
