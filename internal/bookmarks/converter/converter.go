// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package converter provides bookmark export/converter tooling.
package converter

import (
	"context"
	"io"
	"net/http"

	"codeberg.org/readeck/readeck/internal/bookmarks"
)

// Exporter describes a bookmarks exporter.
type Exporter interface {
	Export(ctx context.Context, w io.Writer, r *http.Request, bookmarks []*bookmarks.Bookmark) error
}
