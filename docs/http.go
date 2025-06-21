// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package docs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/komkom/toml"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/locales"
	"codeberg.org/readeck/readeck/pkg/http/csp"
)

type (
	ctxFileKey     struct{}
	ctxSectionKey  struct{}
	ctxLanguageKey struct{}
)

type helpHandlers struct {
	chi.Router
	srv *server.Server
}

type licenseInfo struct {
	Name      string
	License   string
	Author    string
	URL       string
	Copyright string
}

const routePrefix = "/docs"

// SetupRoutes mounts the routes for the auth domain.
func SetupRoutes(s *server.Server) {
	handler := &helpHandlers{
		chi.NewRouter(),
		s,
	}

	// File routes
	for _, f := range manifest.Files {
		if f.IsDocument {
			continue
		}
		handler.With(handler.withFile(f)).Get("/"+f.Route, handler.serveStatic)
	}

	// Document routes
	// docHandler serves the document and requires authentication
	docHandler := handler.With(s.AuthenticatedRouter(s.WithRedirectLogin).Middlewares()...)
	for tag, section := range manifest.Sections {
		for _, f := range section.Files {
			// Document
			docHandler.With(
				s.WithPermission("docs", "read"),
				handler.withFile(f),
				handler.withSection(tag, section),
			).Get("/"+f.Route, handler.serveDocument)

			// Aliases
			for _, alias := range f.Aliases {
				docHandler.With(
					s.WithPermission("docs", "read"),
				).Get("/"+alias, handler.serveRedirect(routePrefix+"/"+f.Route))
			}
		}
	}

	// Changelog route
	f := manifest.Files["changelog"]
	docHandler.With(
		s.WithPermission("system", "read"),
		handler.withFile(f),
	).Get("/changelog", handler.serveDocument)

	// About page
	docHandler.With(
		s.WithPermission("system", "read"),
	).Get("/about", handler.serveAbout)

	// Main redirection (TODO: do something with user language when we have translations)
	docHandler.With(s.WithPermission("docs", "read")).Get("/", handler.localeRedirect)
	docHandler.With(s.WithPermission("docs", "read")).Get("/{path}", handler.localeRedirect)

	// API documentation
	docHandler.With(
		s.WithPermission("docs", "read"),
	).Group(func(r chi.Router) {
		r.Get("/api", handler.serveAPIDocs)
		r.Get("/api.json", handler.serveAPISchema)
	})

	s.AddRoute(routePrefix, handler)
}

func (h *helpHandlers) withFile(f *File) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if f == nil {
				h.srv.Status(w, r, http.StatusNotFound)
				return
			}

			ctx := context.WithValue(r.Context(), ctxFileKey{}, f)

			h.srv.WriteEtag(w, r, f)
			h.srv.WithCaching(next).ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (h *helpHandlers) withSection(tag string, section *Section) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxSectionKey{}, section)
			ctx = context.WithValue(ctx, ctxLanguageKey{}, tag)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (h *helpHandlers) getSection(r *http.Request) (*Section, string) {
	if section, ok := r.Context().Value(ctxSectionKey{}).(*Section); ok {
		return section, r.Context().Value(ctxLanguageKey{}).(string)
	}

	tag := h.srv.Locale(r).Tag.String()
	if _, ok := manifest.Sections[tag]; !ok {
		tag = "en-US"
	}
	return manifest.Sections[tag], tag
}

func (h *helpHandlers) serveDocument(w http.ResponseWriter, r *http.Request) {
	f, _ := r.Context().Value(ctxFileKey{}).(*File)

	fd, err := Files.Open(f.File)
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}
	defer fd.Close()

	var contents strings.Builder
	io.Copy(&contents, fd)
	repl := strings.NewReplacer(
		"readeck-instance://", urls.AbsoluteURL(r, "/").String(),
	)
	buf := new(bytes.Buffer)
	repl.WriteString(buf, contents.String())

	section, tag := h.getSection(r)
	tr := locales.LoadTranslation(tag)
	ctx := server.TC{
		"TOC":      section.TOC,
		"Language": tag,
		"Title":    f.Title,
		"HTML":     buf,
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Documentation"), urls.AbsoluteURL(r, "/docs", tag, "/").String()},
		{f.Title},
	})

	h.srv.RenderTemplate(w, r, http.StatusOK, "docs/index", ctx)
}

func (h *helpHandlers) serveStatic(w http.ResponseWriter, r *http.Request) {
	f, _ := r.Context().Value(ctxFileKey{}).(*File)
	fd, err := Files.Open(f.File)
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}
	defer fd.Close()

	http.ServeContent(w, r, f.File, time.Time{}, fd)
}

func (h *helpHandlers) localeRedirect(w http.ResponseWriter, r *http.Request) {
	tag := h.srv.Locale(r).Tag.String()
	if _, ok := manifest.Sections[tag]; !ok {
		tag = "en-US"
	}

	h.srv.Redirect(w, r, routePrefix+"/"+tag+"/"+chi.URLParam(r, "path"))
}

func (h *helpHandlers) serveRedirect(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.srv.Redirect(w, r, to)
	}
}

func (h *helpHandlers) serveAbout(w http.ResponseWriter, r *http.Request) {
	fp, err := assets.Open("licenses/licenses.toml")
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}

	licenses := map[string][]licenseInfo{}
	dec := json.NewDecoder(toml.New(fp))
	if err = dec.Decode(&licenses); err != nil {
		h.srv.Error(w, r, err)
		return
	}
	slices.SortFunc(licenses["licenses"], func(a, b licenseInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	dbUsageVal, err := db.Driver().DiskUsage()
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}
	diskUsageVal, err := bookmarks.Bookmarks.DiskUsage()
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}

	section, tag := h.getSection(r)
	tr := locales.LoadTranslation(tag)
	ctx := server.TC{
		"TOC":         section.TOC,
		"Language":    tag,
		"Version":     configs.Version(),
		"BuildTime":   configs.BuildTime(),
		"Licenses":    licenses["licenses"],
		"OS":          runtime.GOOS,
		"Arch":        runtime.GOARCH,
		"GoVersion":   runtime.Version(),
		"DBConnecter": db.Driver().Name(),
		"DBVersion":   db.Driver().Version(),
		"DBSize":      dbUsageVal,
		"DiskUsage":   diskUsageVal,
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Documentation"), urls.AbsoluteURL(r, "/docs", tag, "/").String()},
		{tr.Gettext("About Readeck")},
	})

	h.srv.RenderTemplate(w, r, http.StatusOK, "docs/about", ctx)
}

func (h *helpHandlers) serveAPISchema(w http.ResponseWriter, r *http.Request) {
	fd, err := Files.Open("api.json")
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}
	defer fd.Close()

	var contents strings.Builder
	io.Copy(&contents, fd)
	repl := strings.NewReplacer(
		"__BASE_URI__", urls.AbsoluteURL(r, "/api").String(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	repl.WriteString(w, contents.String())
}

func (h *helpHandlers) serveAPIDocs(w http.ResponseWriter, r *http.Request) {
	// By including a web component full of inline styles, we need
	// to relax the style-src policy.
	policy := server.GetCSPHeader(r).Clone()
	policy.Set("style-src", csp.ReportSample, csp.Self, csp.UnsafeInline)
	policy.Write(w.Header())

	tr := h.srv.Locale(r)
	ctx := server.TC{
		"Schema": urls.AbsoluteURL(r, "/docs/api.json"),
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Documentation"), urls.AbsoluteURL(r, "/docs", tr.Tag.String(), "/").String()},
		{"API"},
	})

	h.srv.RenderTemplate(w, r, http.StatusOK, "docs/api-docs", ctx)
}
