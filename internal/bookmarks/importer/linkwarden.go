// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package importer

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/forms"
	"github.com/araddon/dateparse"
)

type linkwardenAdapter struct {
	idx   int
	idMap map[int]int
	Items []*linkwardenItem `json:"items"`
}

type linkwardenFile struct {
	Collections []struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Links []struct {
			ID      int    `json:"id"`
			URL     string `json:"url"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			Created string `json:"createdAt"`
			Tags    []struct {
				Name string `json:"name"`
			} `json:"tags"`
		}
	} `json:"collections"`
	Pinned []struct {
		ID int `json:"id"`
	} `json:"pinnedLinks"`
}
type linkwardenItem struct {
	Link     string        `json:"url"`
	Title    string        `json:"title"`
	Labels   types.Strings `json:"labels"`
	Created  time.Time     `json:"created"`
	IsMarked bool          `json:"is_marked"`
}

func (bi *linkwardenItem) URL() string {
	return bi.Link
}

func (bi *linkwardenItem) Meta() (*BookmarkMeta, error) {
	return &BookmarkMeta{
		Title:    bi.Title,
		Created:  bi.Created,
		IsMarked: bi.IsMarked,
		Labels:   bi.Labels,
	}, nil
}

func (adapter *linkwardenAdapter) Name(tr forms.Translator) string {
	return tr.Gettext("Linkwarden Export File")
}

func (adapter *linkwardenAdapter) Form() forms.Binder {
	return forms.Must(
		context.Background(),
		forms.NewFileField("data", forms.Required),
	)
}

func (adapter *linkwardenAdapter) Params(form forms.Binder) ([]byte, error) {
	if !form.IsValid() {
		return nil, nil
	}

	reader, err := form.Get("data").(*forms.FileField).V().Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close() //nolint:errcheck

	data := &linkwardenFile{}
	dec := json.NewDecoder(reader)
	if err = dec.Decode(data); err != nil {
		form.AddErrors("data", errInvalidFile)
		return nil, nil
	}

	if err = adapter.loadFile(data); err != nil {
		form.AddErrors("data", errInvalidFile)
		return nil, nil
	}

	return json.Marshal(adapter)
}

func (adapter *linkwardenAdapter) LoadData(data []byte) error {
	return json.Unmarshal(data, adapter)
}

func (adapter *linkwardenAdapter) Next() (BookmarkImporter, error) {
	if adapter.idx+1 > len(adapter.Items) {
		return nil, io.EOF
	}

	adapter.idx++
	return adapter.Items[adapter.idx-1], nil
}

func (adapter *linkwardenAdapter) loadFile(file *linkwardenFile) error {
	adapter.idMap = map[int]int{}
	adapter.Items = []*linkwardenItem{}

	for _, c := range file.Collections {
		for _, l := range c.Links {
			if l.Type != "url" && l.Type != "image" {
				continue
			}

			if _, ok := adapter.idMap[l.ID]; ok {
				continue
			}

			item := &linkwardenItem{
				Link:    l.URL,
				Title:   html.UnescapeString(l.Name),
				Created: time.Now().UTC(),
			}

			if d, err := dateparse.ParseAny(l.Created); err == nil {
				item.Created = d.UTC()
			}

			for _, t := range l.Tags {
				item.Labels = append(item.Labels, t.Name)
			}

			adapter.Items = append(adapter.Items, item)
			adapter.idMap[l.ID] = len(adapter.Items) - 1
		}
	}

	// Set favorites
	for _, l := range file.Pinned {
		if i, ok := adapter.idMap[l.ID]; ok {
			adapter.Items[i].IsMarked = true
		}
	}

	return nil
}
