// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package docs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"runtime/debug"
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
	docHandler := handler.With(server.AuthenticatedRouter(server.WithRedirectLogin).Middlewares()...)
	for tag, section := range manifest.Sections {
		for _, f := range section.Files {
			// Document
			docHandler.With(
				server.WithPermission("docs", "read"),
				handler.withFile(f),
				handler.withSection(tag, section),
			).Get("/"+f.Route, handler.serveDocument)

			// Aliases
			for _, alias := range f.Aliases {
				docHandler.With(
					server.WithPermission("docs", "read"),
				).Get("/"+alias, handler.serveRedirect(routePrefix+"/"+f.Route))
			}
		}
	}

	// Changelog route
	f := manifest.Files["changelog"]
	docHandler.With(
		server.WithPermission("system", "read"),
		handler.withFile(f),
	).Get("/changelog", handler.serveDocument)

	// About page
	docHandler.With(
		server.WithPermission("system", "read"),
	).Get("/about", handler.serveAbout)

	// Main redirection (TODO: do something with user language when we have translations)
	docHandler.With(server.WithPermission("docs", "read")).Get("/", handler.localeRedirect)
	docHandler.With(server.WithPermission("docs", "read")).Get("/{path}", handler.localeRedirect)

	// API documentation
	docHandler.With(
		server.WithPermission("docs", "read"),
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
				server.Status(w, r, http.StatusNotFound)
				return
			}

			ctx := context.WithValue(r.Context(), ctxFileKey{}, f)

			server.WriteEtag(w, r, f)
			server.WithCaching(next).ServeHTTP(w, r.WithContext(ctx))
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

	tag := server.Locale(r).Tag.String()
	if _, ok := manifest.Sections[tag]; !ok {
		tag = "en-US"
	}
	return manifest.Sections[tag], tag
}

func (h *helpHandlers) serveDocument(w http.ResponseWriter, r *http.Request) {
	f, _ := r.Context().Value(ctxFileKey{}).(*File)

	fd, err := Files.Open(f.File)
	if err != nil {
		server.Err(w, r, err)
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

	server.RenderTemplate(w, r, http.StatusOK, "docs/index", ctx)
}

func (h *helpHandlers) serveStatic(w http.ResponseWriter, r *http.Request) {
	f, _ := r.Context().Value(ctxFileKey{}).(*File)
	fd, err := Files.Open(f.File)
	if err != nil {
		server.Err(w, r, err)
		return
	}
	defer fd.Close()

	http.ServeContent(w, r, f.File, time.Time{}, fd)
}

func (h *helpHandlers) localeRedirect(w http.ResponseWriter, r *http.Request) {
	tag := server.Locale(r).Tag.String()
	if _, ok := manifest.Sections[tag]; !ok {
		tag = "en-US"
	}

	server.Redirect(w, r, routePrefix+"/"+tag+"/"+chi.URLParam(r, "path"))
}

func (h *helpHandlers) serveRedirect(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		server.Redirect(w, r, to)
	}
}

func (h *helpHandlers) serveAbout(w http.ResponseWriter, r *http.Request) {
	fp, err := assets.Open("licenses/licenses.toml")
	if err != nil {
		server.Err(w, r, err)
		return
	}

	licenses := map[string][]licenseInfo{}
	dec := json.NewDecoder(toml.New(fp))
	if err = dec.Decode(&licenses); err != nil {
		server.Err(w, r, err)
		return
	}
	slices.SortFunc(licenses["licenses"], func(a, b licenseInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	dbUsageVal, err := db.Driver().DiskUsage()
	if err != nil {
		server.Err(w, r, err)
		return
	}
	diskUsageVal, err := bookmarks.Bookmarks.DiskUsage()
	if err != nil {
		server.Err(w, r, err)
		return
	}

	buildInfo := new(strings.Builder)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, x := range info.Settings {
			fmt.Fprintf(buildInfo, "%s: %s\n", x.Key, strings.ReplaceAll(x.Value, ",", ", "))
		}
	}

	section, tag := h.getSection(r)
	tr := locales.LoadTranslation(tag)
	ctx := server.TC{
		"TOC":         section.TOC,
		"Language":    tag,
		"Version":     configs.Version(),
		"BuildTime":   configs.BuildTime(),
		"BuildInfo":   buildInfo,
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

	server.RenderTemplate(w, r, http.StatusOK, "docs/about", ctx)
}

func (h *helpHandlers) serveAPISchema(w http.ResponseWriter, r *http.Request) {
	fd, err := Files.Open("api.json")
	if err != nil {
		server.Err(w, r, err)
		return
	}
	defer fd.Close()

	var contents strings.Builder
	io.Copy(&contents, fd)
	repl := strings.NewReplacer(
		"__ROOT_URI__", strings.TrimSuffix(urls.AbsoluteURL(r, "/").String(), "/"),
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

	tr := server.Locale(r)
	ctx := server.TC{
		"Schema": urls.AbsoluteURL(r, "/docs/api.json"),
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Documentation"), urls.AbsoluteURL(r, "/docs", tr.Tag.String(), "/").String()},
		{"API"},
	})

	server.RenderTemplate(w, r, http.StatusOK, "docs/api-docs", ctx)
}
