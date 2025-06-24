// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package importer

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"

	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type csvAdapter struct {
	idx   int
	Items []csvBookmarkItem `json:"items"`
}

type csvBookmarkItem struct {
	Link       string        `json:"url"`
	Title      string        `json:"title"`
	Created    time.Time     `json:"created"`
	Labels     types.Strings `json:"labels"`
	IsArchived bool          `json:"is_archived"`
}

func newCSVBookmarkItem(headerMap csvHeaderMap, record []string) (csvBookmarkItem, error) {
	res := csvBookmarkItem{}
	if headerMap.url == -1 || len(record) < headerMap.url {
		return res, errors.New("no URL provided")
	}
	res.Link = record[headerMap.url]

	if headerMap.title != -1 && len(record) > headerMap.title {
		res.Title = strings.TrimSpace(record[headerMap.title])
	}
	if headerMap.created != -1 && len(record) > headerMap.created {
		ts, err := strconv.Atoi(record[headerMap.created])
		if err == nil {
			res.Created = time.Unix(int64(ts), 0)
		} else {
			res.Created, _ = dateparse.ParseAny(record[headerMap.created])
		}
	}
	if headerMap.labels != -1 && len(record) > headerMap.labels {
		labels := record[headerMap.labels]
		if labels != "" && labels != "[]" {
			_ = json.Unmarshal([]byte(labels), &res.Labels)
		}
	}
	if headerMap.state != -1 && len(record) > headerMap.state {
		state := record[headerMap.state]
		if strings.ToLower(state) == "archive" {
			res.IsArchived = true
		}
	}

	return res, nil
}

func (bi *csvBookmarkItem) URL() string {
	return bi.Link
}

func (bi *csvBookmarkItem) Meta() (*BookmarkMeta, error) {
	return &BookmarkMeta{
		Title:      bi.Title,
		Created:    bi.Created,
		Labels:     bi.Labels,
		IsArchived: bi.IsArchived,
	}, nil
}

type csvHeaderMap struct {
	url     int
	title   int
	state   int
	created int
	labels  int
}

func newHeaderMap(record []string) csvHeaderMap {
	res := csvHeaderMap{
		url:     -1,
		title:   -1,
		state:   -1,
		created: -1,
		labels:  -1,
	}
	for i, x := range record {
		switch strings.ToLower(x) {
		case "url":
			res.url = i
		case "title":
			res.title = i
		case "folder", "state":
			res.state = i
		case "created", "timestamp":
			res.created = i
		case "labels", "tags":
			res.labels = i
		}
	}

	return res
}

func (adapter *csvAdapter) Name(tr forms.Translator) string {
	return tr.Gettext("CSV File")
}

func (adapter *csvAdapter) Form() forms.Binder {
	return forms.Must(
		context.Background(),
		forms.NewFileField("data", forms.Required),
	)
}

func (adapter *csvAdapter) Params(form forms.Binder) ([]byte, error) {
	if !form.IsValid() {
		return nil, nil
	}

	reader, err := form.Get("data").(*forms.FileField).V().Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close() //nolint:errcheck

	r := csv.NewReader(reader)
	headers, err := r.Read()
	if err != nil {
		form.AddErrors("data", forms.Gettext("Empty or invalid import file"))
		return nil, nil
	}
	headerMap := newHeaderMap(headers)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			form.AddErrors("data", forms.Gettext("Empty or invalid import file"))
			return nil, nil
		}
		item, err := newCSVBookmarkItem(headerMap, record)
		if err != nil {
			form.AddErrors("data", forms.Gettext("Empty or invalid import file"))
			return nil, nil
		}
		_ = item

		adapter.Items = append(adapter.Items, item)
	}

	if len(adapter.Items) == 0 {
		form.AddErrors("data", forms.Gettext("Empty or invalid import file"))
		return nil, nil
	}

	slices.Reverse(adapter.Items)
	return json.Marshal(adapter)
}

func (adapter *csvAdapter) LoadData(data []byte) error {
	return json.Unmarshal(data, adapter)
}

func (adapter *csvAdapter) Next() (BookmarkImporter, error) {
	if adapter.idx+1 > len(adapter.Items) {
		return nil, io.EOF
	}

	adapter.idx++
	return &adapter.Items[adapter.idx-1], nil
}
