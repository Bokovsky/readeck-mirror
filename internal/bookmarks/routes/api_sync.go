// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	"net/http"

	"github.com/doug-martin/goqu/v9"
	goquExp "github.com/doug-martin/goqu/v9/exp"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/converter"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/db/exp"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/forms"
)

func (api *apiRouter) bookmarkSyncList(w http.ResponseWriter, r *http.Request) {
	f := newSyncListForm(server.Locale(r))
	forms.BindURL(f, r)

	if !f.IsValid() {
		server.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	ds := bookmarks.Bookmarks.Query().
		Select(
			goqu.C("uid").Table("b"),
			goqu.C("updated").Table("b").As("time"),
			goqu.V("update").As("type"),
		).
		Where(goqu.C("user_id").Table("b").Eq(auth.GetRequestUser(r).ID))

	if !f.Get("since").IsEmpty() {
		// When querying with ?since=, we perform a union with bookmark_removed
		// to build some kind of update/delete log.
		since := f.Get("since").(*forms.DatetimeField).V().UTC()

		ds = ds.Where(
			exp.DateTime(goqu.C("updated").Table("b")).
				Gte(exp.DateTime(since)),
		)
		ds = ds.Union(
			db.Q().
				From(goqu.T(bookmarks.TableNameRemoved).As("r")).
				Select(
					goqu.C("uid").Table("r"),
					goqu.C("deleted").Table("r").As("time"),
					goqu.V("delete").As("type"),
				).
				Where(
					goqu.C("user_id").Table("r").Eq(auth.GetRequestUser(r).ID),
					exp.DateTime(goqu.C("deleted").Table("r")).
						Gte(exp.DateTime(since)),
				),
		)
	}

	ds = ds.Order(goqu.C("time").Desc())

	bl, err := dataset.NewBookmarkSyncList(r.Context(), ds)
	if err != nil {
		server.Err(w, r, err)
		return
	}

	server.WriteEtag(w, r, bl)
	server.WriteLastModified(w, r, bl)
	if !server.HandleCaching(w, r) {
		server.Render(w, r, http.StatusOK, bl)
	}
}

func (api *apiRouter) bookmarkSync(w http.ResponseWriter, r *http.Request) {
	of := newOrderForm("sort", map[string]goquExp.Orderable{
		"updated": goqu.C("updated"),
		"created": goqu.C("created"),
	})
	f := forms.Join(context.Background(),
		newSyncForm(server.Locale(r)),
		of,
	)
	forms.Bind(f, r)

	if !f.IsValid() {
		server.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	ds := bookmarks.Bookmarks.Query().
		Where(
			goqu.C("user_id").Table("b").Eq(auth.GetRequestUser(r).ID),
		).
		Order(goqu.C("updated").Asc())

	if order := of.toOrderedExpressions(); order != nil {
		ds = ds.Order(order...)
	}

	ids := f.Get("id").(*forms.TextListField).V()
	if len(ids) > 0 {
		ds = ds.Where(goqu.C("uid").In(ids))
	}

	seq := dataset.NewBookmarkIterator(r.Context(), ds)
	if err := converter.NewSyncExporter(
		converter.WithSyncJSON(f.Get("with_json").(*forms.BooleanField).V()),
		converter.WithSyncHTML(f.Get("with_html").(*forms.BooleanField).V()),
		converter.WithSyncMarkdown(f.Get("with_markdown").(*forms.BooleanField).V()),
		converter.WithSyncResources(f.Get("with_resources").(*forms.BooleanField).V()),
		converter.WithSyncResourcePrefix(f.Get("resource_prefix").String()),
	).IterExport(r.Context(), w, r, seq); err != nil {
		server.Err(w, r, err)
	}
}
