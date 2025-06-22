// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package videoplayer provides a route for an HLS embed video player.
package videoplayer

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/http/csp"
)

// SetupRoutes mounts the routes for the videoplayer domain.
func SetupRoutes(s *server.Server) {
	// The /videoplayer route is not authenticated
	r := chi.NewRouter()
	r.Get("/", videoPlayerHandler)

	s.AddRoute("/videoplayer", r)
}

func videoPlayerHandler(w http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("src")
	mediaType := r.URL.Query().Get("type")
	if src == "" {
		server.Status(w, r, http.StatusBadRequest)
		return
	}

	ctx := server.TC{
		"Src":    src,
		"Type":   mediaType,
		"Height": r.URL.Query().Get("h"),
		"Width":  r.URL.Query().Get("w"),
	}

	// Set appropriate CSP values for thie ressource to work
	// as a video play in an iframe.
	policy := server.GetCSPHeader(r)
	policy.Set("connect-src", "*")
	policy.Set("worker-src", "blob:")
	policy.Add("media-src", "blob:", "*")
	policy.Set("frame-ancestors", csp.Self)

	policy.Write(w.Header())
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")

	server.RenderTemplate(w, r, 200, "videoplayer/index", ctx)
}
