// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks/importer"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/forms"
)

const maxUploadSize = 4 << 20

func (api *apiRouter) bookmarksImport(w http.ResponseWriter, r *http.Request) {
	source := chi.URLParam(r, "source")

	if r.ContentLength > maxUploadSize {
		api.srv.TextMessage(w, r, http.StatusRequestEntityTooLarge, "Payload Too Large")
		return
	}

	adapter := importer.LoadAdapter(source)
	if adapter == nil {
		api.srv.TextMessage(w, r, http.StatusNotFound, fmt.Sprintf(`Import from "%s" does not exist.`, source))
		return
	}

	f := importer.NewImportForm(
		forms.WithTranslator(context.Background(), api.srv.Locale(r)),
		adapter,
	)
	forms.Bind(f, r)

	// If the form is valid, we can load the adapter parameters.
	var data []byte
	var err error
	if f.IsValid() {
		data, err = adapter.Params(f)
	}

	// At this point, any error is fatal.
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	// The adapter's form might have changed so we can only check the form's validity now.
	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	ignoreDuplicates := f.Get("ignore_duplicates").(forms.TypedField[bool]).V()

	// Create the import task
	trackID := importer.GetTrackID(api.srv.GetReqID(r))
	err = importer.ImportBookmarksTask.Run(trackID, importer.ImportParams{
		Source:          source,
		Data:            data,
		UserID:          auth.GetRequestUser(r).ID,
		RequestID:       api.srv.GetReqID(r),
		AllowDuplicates: !ignoreDuplicates,
		Label:           f.Get("label").String(),
		Archive:         f.Get("archive").(forms.TypedField[bool]).V(),
		MarkRead:        f.Get("mark_read").(forms.TypedField[bool]).V(),
	})
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	w.Header().Add(
		"Location",
		urls.AbsoluteURL(r, "./..", trackID).String(),
	)
	api.srv.TextMessage(w, r, http.StatusAccepted, "import started")
}

func (api *apiRouter) bookmaksImportStatus(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "trackID")
	p, err := importer.NewImportProgress(trackID)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	if api.srv.IsTurboRequest(r) {
		api.srv.RenderTurboStream(w, r,
			"/bookmarks/import/progress", "replace",
			"import-progress-"+trackID, map[string]interface{}{
				"TrackID":  trackID,
				"Running":  importer.ImportBookmarksTask.IsRunning(trackID),
				"Progress": p,
			},
			nil,
		)
		return
	}

	api.srv.Render(w, r, http.StatusOK, map[string]interface{}{
		"scheduled": importer.ImportBookmarksTask.IsRunning(trackID),
		"progress":  p,
	})
}
