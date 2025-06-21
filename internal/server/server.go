// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package server is the main Readeck HTTP server.
// It defines common middlewares, guards, permission handlers, etc.
package server

import (
	"log/slog"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/metrics"
	"codeberg.org/readeck/readeck/internal/server/urls"
)

// Server is a wrapper around chi router.
type Server struct {
	*chi.Mux
}

// New create a new server. Routes must be added manually before
// calling ListenAndServe.
func New() *Server {
	s := &Server{
		chi.NewRouter(),
	}

	s.Use(
		middleware.Recoverer,
		s.InitRequest,
		middleware.RequestID,
		Logger(),
		metrics.Middleware,
		s.SetSecurityHeaders,
		s.CompressResponse,
		s.WithCacheControl,
		s.CannonicalPaths,
		auth.Init(
			&auth.TokenAuthProvider{},
			&auth.SessionAuthProvider{
				GetSession:          s.GetSession,
				UnauthorizedHandler: s.unauthorizedHandler,
			},
		),
		s.LoadLocale,
		s.ErrorPages,
	)

	return s
}

// Init initializes the server and the template engine.
func (s *Server) Init() {
	// System routes
	s.AddRoute("/api/info", s.infoRoutes())
	s.AddRoute("/api/sys", s.sysRoutes())
	s.AddRoute("/logger", s.loggerRoutes())

	// web manifest
	s.AddRoute("/manifest.webmanifest", s.manifestRoutes())

	// Init templates
	s.initTemplates()
}

// AuthenticatedRouter returns a chi.Router instance
// with middlewares to force authentication.
func (s *Server) AuthenticatedRouter(middlewares ...func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	r.Use(middlewares...)
	r.Use(
		s.WithSession(),
		auth.Required,
		s.LoadLocale,
		s.Csrf,
		// It's already in the main router but this one will be called first and have
		// the current user information
		s.ErrorPages,
	)

	return r
}

// AddRoute adds a new route to the server, prefixed with
// the BasePath.
func (s *Server) AddRoute(pattern string, handler http.Handler) {
	s.Mount(path.Join(urls.Prefix(), pattern), handler)
}

// IsTurboRequest returns true when the request was made with
// an x-turbo header.
func (s *Server) IsTurboRequest(r *http.Request) bool {
	return r.Header.Get("x-turbo") == "1"
}

// Redirect yields a 303 redirection with a location header.
// The given "ref" values are joined togegher with the server's base path
// to provide a full absolute URL.
func (s *Server) Redirect(w http.ResponseWriter, r *http.Request, ref ...string) {
	w.Header().Set("Location", urls.AbsoluteURL(r, ref...).String())
	w.WriteHeader(http.StatusSeeOther)
}

// Log returns a log entry including the request ID.
func (s *Server) Log(r *http.Request) *slog.Logger {
	return slog.With(slog.String("@id", s.GetReqID(r)))
}

// infoRoutes returns the route returning the service information.
func (s *Server) infoRoutes() http.Handler {
	r := chi.NewRouter()

	type versionInfo struct {
		Canonical string `json:"canonical"`
		Release   string `json:"release"`
		Build     string `json:"build"`
	}

	type serviceInfo struct {
		Version versionInfo `json:"version"`
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		canonical := configs.Version()
		release, build, _ := strings.Cut(canonical, "-")

		res := serviceInfo{
			Version: versionInfo{
				Canonical: canonical,
				Release:   release,
				Build:     build,
			},
		}

		s.Render(w, r, 200, res)
	})

	return r
}

// sysRoutes returns the route returning some system
// information.
func (s *Server) sysRoutes() http.Handler {
	r := s.AuthenticatedRouter()
	r.Use(s.WithPermission("system", "read"))

	type memInfo struct {
		Alloc      uint64 `json:"alloc"`
		TotalAlloc uint64 `json:"totalalloc"`
		Sys        uint64 `json:"sys"`
		NumGC      uint32 `json:"numgc"`
	}
	type storageInfo struct {
		Database  uint64 `json:"database"`
		Bookmarks uint64 `json:"bookmarks"`
	}

	type sysInfo struct {
		Version   string      `json:"version"`
		BuildDate time.Time   `json:"build_date"`
		OS        string      `json:"os"`
		Platform  string      `json:"platform"`
		Hostname  string      `json:"hostname"`
		CPUs      int         `json:"cpus"`
		GoVersion string      `json:"go_version"`
		Mem       memInfo     `json:"mem"`
		DiskUsage storageInfo `json:"disk_usage"`
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		host, _ := os.Hostname()

		var err error
		usage := storageInfo{}
		usage.Database, err = db.Driver().DiskUsage()
		if err != nil {
			s.Error(w, r, err)
		}

		usage.Bookmarks, err = bookmarks.Bookmarks.DiskUsage()
		if err != nil {
			s.Error(w, r, err)
		}

		res := sysInfo{
			Version:   configs.Version(),
			BuildDate: configs.BuildTime(),
			OS:        runtime.GOOS,
			Platform:  runtime.GOARCH,
			Hostname:  host,
			CPUs:      runtime.NumCPU(),
			GoVersion: runtime.Version(),
			Mem: memInfo{
				Alloc:      m.Alloc,
				TotalAlloc: m.TotalAlloc,
				Sys:        m.Sys,
				NumGC:      m.NumGC,
			},
			DiskUsage: usage,
		}

		s.Render(w, r, 200, res)
	})

	return r
}

func (s *Server) loggerRoutes() http.Handler {
	r := chi.NewRouter()
	r.Post("/csp-report", s.cspReport)

	return r
}

// GetReqID returns the request ID.
func (s *Server) GetReqID(r *http.Request) string {
	return middleware.GetReqID(r.Context())
}
