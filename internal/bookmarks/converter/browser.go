// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package converter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/internal/server"
)

// BrowserExporter is an [IterExporter] that produces a browser bookmark file.
type BrowserExporter struct{}

// IterExport implements [IterExporter].
func (e BrowserExporter) IterExport(_ context.Context, w io.Writer, r *http.Request, bookmarkSeq *dataset.BookmarkIterator) error {
	if w, ok := w.(http.ResponseWriter); ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(
			`attachment; filename="readeck-bookmarks-%s.html"`,
			time.Now().UTC().Format(time.DateOnly),
		))
	}

	uncategorized := []*dataset.Bookmark{}
	favorite := []*dataset.Bookmark{}
	archived := []*dataset.Bookmark{}

	for b, err := range bookmarkSeq.Items {
		if err != nil {
			return err
		}
		switch {
		case b.IsMarked:
			favorite = append(favorite, b)
		case b.IsArchived:
			archived = append(archived, b)
		default:
			uncategorized = append(uncategorized, b)
		}
	}

	tpl, err := server.GetTemplate("bookmarks/export/browser.jet.html")
	if err != nil {
		return err
	}

	tc := map[string]any{
		"Uncategorized": uncategorized,
		"Favorite":      favorite,
		"Archived":      archived,
		"labelsToAttr": func(s []string) string {
			res := make([]string, len(s))
			for i, x := range s {
				res[i] = strings.ReplaceAll(html.EscapeString(x), ",", "&#44;")
			}

			return strings.Join(res, ",")
		},
	}

	return tpl.Execute(w, server.TemplateVars(r), tc)
}
