// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package converter provides bookmark export/converter tooling.
package converter

import (
	"context"
	"io"
	"net/http"

	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
)

// Exporter describes a bookmarks exporter.
type Exporter interface {
	Export(ctx context.Context, w io.Writer, r *http.Request, bookmarkList *dataset.BookmarkList) error
}

// IterExporter describes a bookmarks exporter that works with an iterator.
type IterExporter interface {
	IterExport(ctx context.Context, w io.Writer, r *http.Request, bookmarkSeq *dataset.BookmarkIterator) error
}
