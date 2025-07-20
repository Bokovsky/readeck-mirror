// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package cookbook

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type cookbookViews struct {
	chi.Router
	*cookbookAPI
}

func newCookbookViews(api *cookbookAPI) *cookbookViews {
	r := server.AuthenticatedRouter(server.WithRedirectLogin)
	v := &cookbookViews{r, api}

	r.With(server.WithPermission("cookbook", "read")).Group(func(r chi.Router) {
		r.Get("/", v.namedTemplateView("prose"))
		r.Get("/ui", v.uiView)
		r.Get("/{name}", v.templateView)
		r.Get("/extract", v.extractView)
	})

	return v
}

func (v *cookbookViews) templateView(w http.ResponseWriter, r *http.Request) {
	template := "cookbook/" + chi.URLParam(r, "name")
	_, err := server.GetTemplate(template)
	if err != nil {
		server.Log(r).Error("can't load template", slog.Any("err", err))
		server.Status(w, r, http.StatusNotFound)
		return
	}

	server.RenderTemplate(w, r, 200, template, nil)
}

func (v *cookbookViews) namedTemplateView(name string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		chi.RouteContext(r.Context()).URLParams.Add("name", name)
		v.templateView(w, r)
	}
}

func (v *cookbookViews) uiView(w http.ResponseWriter, r *http.Request) {
	f := newCookbookForm()
	ef := newCookbookForm()
	forms.BindURL(ef, r)

	ctx := server.TC{
		"Form":    f,
		"FormErr": ef,
	}

	server.RenderTemplate(w, r, 200, "cookbook/ui", ctx)
}

func (v *cookbookViews) extractView(w http.ResponseWriter, r *http.Request) {
	f := newExtractForm()
	forms.BindURL(f, r)

	ctx := server.TC{
		"Form":   f,
		"Result": nil,
	}

	if f.IsValid() && f.Get("url").String() != "" {
		ex := v.getExtractor(f.Get("url").String(), r)
		res := v.getExtractResult(ex)
		ctx["Result"] = res
		ctx["HTML"] = strings.NewReader(bookmarks.ExtractHTMLBody(res.HTML))
	}

	server.RenderTemplate(w, r, 200, "cookbook/extract", ctx)
}

func newCookbookForm() *forms.Form {
	return forms.Must(
		context.Background(),
		forms.NewTextField("text", forms.Required, forms.IsEmail),
		forms.NewTextField("select", forms.Default("choice 2"), forms.Choices(
			forms.Choice("Choice 1", "choice 1"),
			forms.Choice("Choice 2", "choice 2"),
			forms.Choice("Choice 3", "choice 3"),
		)),
		forms.NewTextListField("choices", forms.Default([]string{"b"}), forms.Required, forms.Choices(
			forms.Choice("Choice A", "a"),
			forms.Choice("Choice B", "b"),
			forms.Choice("Choice C", "c"),
		)),
	)
}
