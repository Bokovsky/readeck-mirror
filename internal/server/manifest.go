// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"fmt"
	"net/http"

	"codeberg.org/readeck/readeck/internal/server/urls"
	"github.com/go-chi/chi/v5"
)

type webManifest struct {
	Name                      string              `json:"name"`
	ShortName                 string              `json:"short_name"`
	Description               string              `json:"description"`
	ID                        string              `json:"id"`
	StartURL                  string              `json:"start_url"`
	Scope                     string              `json:"scope"`
	ScopeExtensions           []map[string]string `json:"scope_extensions"`
	Categories                []string            `json:"categories"`
	Dir                       string              `json:"dir"`
	Lang                      string              `json:"lang"`
	Orientation               string              `json:"orientation"`
	PreferRelatedApplications bool                `json:"prefer_related_applications"`
	RelatedApplications       []map[string]string `json:"related_applications"`
	LaunchHandler             map[string]string   `json:"launch_handler"`
	ThemeColor                string              `json:"theme_color"`
	BackgroundColor           string              `json:"background_color"`
	Display                   string              `json:"display"`
	DisplayOverride           []string            `json:"display_override"`
	Icons                     []webManifestIcon   `json:"icons"`
}

type webManifestIcon struct {
	SRC     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

func (s *Server) manifestRoutes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		topURL := urls.AbsoluteURL(r, "/")

		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		s.Render(w, r, http.StatusOK, webManifest{
			Name:                      fmt.Sprintf("Readeck (%s)", r.URL.Hostname()),
			ShortName:                 "Readeck",
			Description:               "Save interesting articles, long read, pictures, videos. Read or revisit them later.",
			ID:                        topURL.String(),
			StartURL:                  topURL.String(),
			Scope:                     topURL.Path,
			ScopeExtensions:           []map[string]string{},
			Categories:                []string{"education", "news", "productivity"},
			Dir:                       "auto",
			Lang:                      "en",
			Orientation:               "natural",
			PreferRelatedApplications: false,
			RelatedApplications:       []map[string]string{},
			LaunchHandler:             map[string]string{"client_mode": "navigate-existing"},
			ThemeColor:                "#064c5c",
			BackgroundColor:           "#161311",
			Display:                   "standalone",
			DisplayOverride:           []string{"standalone"},
			Icons: []webManifestIcon{
				{
					SRC:     urls.PathOnly(urls.AssetURL(r, "img/fi/android-chrome-192x192.png")),
					Sizes:   "192x192",
					Type:    "image/png",
					Purpose: "any",
				},
				{
					SRC:     urls.PathOnly(urls.AssetURL(r, "img/fi/android-chrome-512x512.png")),
					Sizes:   "512x512",
					Type:    "image/png",
					Purpose: "any",
				},
				{
					SRC:     urls.PathOnly(urls.AssetURL(r, "img/fi/favicon.svg")),
					Sizes:   "any",
					Type:    "image/svg+xml",
					Purpose: "any",
				},
				{
					SRC:     urls.PathOnly(urls.AssetURL(r, "img/logo-maskable.svg")),
					Sizes:   "any",
					Type:    "image/svg+xml",
					Purpose: "maskable",
				},
			},
		})
	})

	return r
}
