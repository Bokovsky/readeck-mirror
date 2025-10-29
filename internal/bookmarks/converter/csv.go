// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package converter

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
)

// CSVExporter is an [IterExporter] that produces a CSV file.
type CSVExporter struct{}

// IterExport implements [IterExporter].
func (e CSVExporter) IterExport(_ context.Context, w io.Writer, _ *http.Request, bookmarkSeq *dataset.BookmarkIterator) error {
	if w, ok := w.(http.ResponseWriter); ok {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(
			`attachment; filename="readeck-bookmarks-%s.csv"`,
			time.Now().Format(time.DateOnly),
		))
	}

	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	if err := cw.Write([]string{
		"url", "title", "created", "labels", "is_archived", "is_marked", "type",
	}); err != nil {
		return err
	}

	for b, err := range bookmarkSeq.Items {
		if err != nil {
			return err
		}

		labels, err := json.Marshal(b.Labels)
		if err != nil {
			return err
		}

		if err = cw.Write([]string{
			b.URL,
			b.Title,
			b.Created.UTC().Format(time.RFC3339),
			string(labels),
			strconv.FormatBool(b.IsArchived),
			strconv.FormatBool(b.IsMarked),
			b.Type,
		}); err != nil {
			return err
		}
	}
	cw.Flush()

	return nil
}
