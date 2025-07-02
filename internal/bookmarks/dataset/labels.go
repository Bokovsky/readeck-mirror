// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package dataset

import (
	"context"
	"net/url"
	"strconv"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/db/scanner"
	"codeberg.org/readeck/readeck/internal/server/urls"
)

// LabelList is a list of [*Label].
type LabelList []*Label

// Label contains a label's information.
type Label struct {
	Name          labelString `db:"name"  json:"name"`
	Count         int         `db:"count" json:"count"`
	Href          string      `db:"-"     json:"href"`
	HrefBookmarks string      `db:"-"     json:"href_bookmarks"`
}

// NewLabelList returns a new [LabelList] from a select dataset.
func NewLabelList(ctx context.Context, ds *goqu.SelectDataset) (LabelList, error) {
	res := LabelList{}

	for item, err := range scanner.IterTransform(ctx, ds, NewLabel) {
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}

	return res, nil
}

// NewLabel returns a new [*Label], setting the necessary URLs.
func NewLabel(ctx context.Context, l *Label) *Label {
	l.Href = urls.AbsoluteURLContext(ctx, "/api/bookmarks/labels", l.Name.Path()).String()
	l.HrefBookmarks = urls.AbsoluteURLContext(ctx, "/api/bookmarks").String() + "?" + url.Values{
		"labels": []string{strconv.Quote(string(l.Name))},
	}.Encode()
	return l
}

// labelString is a string with a Path method.
type labelString string

func (s labelString) Path() string {
	return url.QueryEscape(string(s))
}
