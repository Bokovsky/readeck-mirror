// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package importer

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/bus"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/superbus"
)

var (
	// ImportBookmarksTask is the bookmark import task.
	ImportBookmarksTask superbus.Task
	// ImportExtractTask is the bookmark extraction task.
	ImportExtractTask superbus.Task
)

// ImportParams contains the ImportBookmarksTask parameters.
type ImportParams struct {
	Source          string `json:"source"`
	Data            []byte `json:"data"`
	UserID          int    `json:"user_id"`
	RequestID       string `json:"request_id"`
	AllowDuplicates bool   `json:"allow_duplicates"`
	Label           string `json:"label"`
	Archive         bool   `json:"archive"`
	MarkRead        bool   `json:"mark_read"`
}

func init() {
	bus.OnReady(func() {
		ImportBookmarksTask = bus.Tasks().NewTask(
			"bookmarks.import",
			superbus.WithUnmarshall(func(data []byte) any {
				var res ImportParams
				err := json.Unmarshal(data, &res)
				if err != nil {
					panic(err)
				}
				return res
			}),
			superbus.WithTaskHandler(importBookmarksHandler),
		)
		ImportExtractTask = bus.Tasks().NewTask(
			"bookmarks.import_extract",
			superbus.WithUnmarshall(func(data []byte) any {
				var res tasks.ExtractParams
				err := json.Unmarshal(data, &res)
				if err != nil {
					panic(err)
				}
				return res
			}),
			superbus.WithTaskHandler(importExtractHandler),
		)
	})
}

func importBookmarksHandler(data any) {
	params := data.(ImportParams)

	adapter := LoadAdapter(params.Source)
	if adapter == nil {
		panic(fmt.Errorf(`adapter "%s" not found`, params.Source))
	}

	worker, ok := adapter.(ImportWorker)
	if !ok {
		panic(fmt.Errorf(`loader "%s" does not implement worker`, params.Source))
	}

	var err error
	if err = worker.LoadData(params.Data); err != nil {
		panic(err)
	}

	imp := importer{
		worker:          worker,
		requestID:       params.RequestID,
		allowDuplicates: params.AllowDuplicates,
		label:           params.Label,
		archive:         params.Archive,
		markRead:        params.MarkRead,
	}

	if imp.user, err = users.Users.GetOne(goqu.C("id").Eq(params.UserID)); err != nil {
		panic(err)
	}

	imp.log = slog.With(
		slog.Int("user", imp.user.ID),
		slog.String("@id", imp.requestID),
	)

	trackID := GetTrackID(imp.requestID)
	imp.Import(func(ids []int) {
		_ = setStoreProgressList(trackID, ids)
	})
}

func importExtractHandler(data any) {
	params := data.(tasks.ExtractParams)
	trackID := GetTrackID(params.RequestID)

	logger := slog.With(
		slog.String("@id", params.RequestID),
		slog.Int("bookmark_id", params.BookmarkID),
	)

	defer func() {
		p, err := NewImportProgress(trackID)
		if err != nil {
			logger.Error("fetching progress", slog.Any("err", err))
		}

		if p.Status == 1 {
			if err = clearStoreProgressList(trackID); err != nil {
				logger.Error("clearing progress", slog.Any("err", err))
			}
			logger.Info("import finished")
		}
	}()

	tasks.ExtractPage(params)
}

func getStoreProgressList(trackID string) (ids []int) {
	ids = []int{}
	data := bus.Store().Get("bookmark_import_" + trackID)

	if data == "" {
		return
	}
	_ = json.Unmarshal([]byte(data), &ids)
	return
}

func setStoreProgressList(trackID string, ids []int) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return bus.Store().Set("bookmark_import_"+trackID, string(data), 0)
}

func clearStoreProgressList(trackID string) error {
	return bus.Store().Del("bookmark_import_" + trackID)
}

// GetTrackID returns a tracking ID based on the request ID.
func GetTrackID(requestID string) string {
	return base58.EncodeUUID(
		uuid.NewSHA1(uuid.NameSpaceOID, []byte(requestID)),
	)
}

// ImportProgress contains the import progress information.
type ImportProgress struct {
	Total  int64 `json:"total"`
	Done   int64 `json:"done"`
	Status int   `json:"status"`
}

// NewImportProgress returns an ImportProgress instance based on a
// trackID. It counts bookmarks with a state not StateLoading.
func NewImportProgress(trackID string) (p ImportProgress, err error) {
	ids := getStoreProgressList(trackID)
	p.Total = int64(len(ids))
	if p.Total == 0 {
		p.Status = 1
		return
	}

	p.Done, err = db.Q().Select(goqu.C("id")).
		From(db.TableBookmark).
		Where(
			goqu.C("id").In(ids),
			goqu.C("state").Neq(bookmarks.StateLoading),
		).
		Count()
	if err != nil {
		return
	}

	if p.Done == p.Total {
		p.Status = 1
	}

	return
}
