// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/forms"
)

// PaginationForm is a default form for pagination.
type PaginationForm struct {
	*forms.Form
}

func newPaginationForm(tr forms.Translator) *PaginationForm {
	return &PaginationForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewIntegerField("limit", forms.Gte(0), forms.Lte(100)),
		forms.NewIntegerField("offset", forms.Gte(0)),
	)}
}

// Limit returns the current limit or zero if none was given.
func (f *PaginationForm) Limit() int {
	return f.Get("limit").(forms.TypedField[int]).V()
}

// Offset returns the current offset or 0 if none was given.
func (f *PaginationForm) Offset() int {
	return f.Get("offset").(forms.TypedField[int]).V()
}

// SetLimit sets the limit's value. It's used to set a default limit before binding the form.
func (f *PaginationForm) SetLimit(v int) {
	f.Get("limit").Set(v)
}

// GetPageParams returns the pagination parameters from the query string.
func GetPageParams(r *http.Request, defaultLimit int) *PaginationForm {
	f := newPaginationForm(Locale(r))
	f.Get("limit").Set(0)
	f.Get("offset").Set(0)
	forms.BindURL(f, r)

	if !f.IsValid() {
		return nil
	}

	if f.Get("limit").(*forms.IntegerField).V() == 0 {
		f.SetLimit(defaultLimit)
	}

	return f
}

// Pagination holds all the information regarding pagination.
type Pagination struct {
	URL          *url.URL
	Limit        int
	Offset       int
	TotalCount   int
	TotalPages   int
	CurrentPage  int
	First        int
	Last         int
	Next         int
	Previous     int
	FirstPage    string
	LastPage     string
	NextPage     string
	PreviousPage string
	PageLinks    []PageLink
}

// PageLink contains a link to a page in a Pagination instance.
type PageLink struct {
	Index int
	URL   string
}

// GetLink returns a new url string with limit and offset values.
func (p Pagination) GetLink(offset int) string {
	u := *p.URL
	q := u.Query()
	q.Set("limit", strconv.Itoa(p.Limit))
	q.Set("offset", strconv.Itoa(offset))
	u.RawQuery = q.Encode()
	return u.String()
}

// GetPageLinks returns the links that can be used in a template.
func (p Pagination) GetPageLinks() []PageLink {
	res := []PageLink{
		{1, p.GetLink(0)},
	}

	prevLinks := []PageLink{}
	for i := p.CurrentPage - 1; i > max(1, p.CurrentPage-3); i-- {
		prevLinks = append([]PageLink{{i, p.GetLink((i - 1) * p.Limit)}}, prevLinks...)
	}
	if len(prevLinks) > 0 && prevLinks[0].Index > 2 {
		res = append(res, PageLink{})
	}
	res = append(res, prevLinks...)

	if p.CurrentPage > 1 {
		res = append(res, PageLink{p.CurrentPage, p.GetLink((p.CurrentPage - 1) * p.Limit)})
	}

	for i := p.CurrentPage + 1; i < min(p.TotalPages, p.CurrentPage+3); i++ {
		res = append(res, PageLink{i, p.GetLink((i - 1) * p.Limit)})
	}

	if len(res) > 0 && res[len(res)-1].Index < p.TotalPages-1 {
		res = append(res, PageLink{})
	}

	if p.CurrentPage < p.TotalPages {
		res = append(res, PageLink{p.TotalPages, p.GetLink(p.Last)})
	}

	return res
}

// NewPagination creates a new Pagination instance base on the current request.
func NewPagination(r *http.Request, count, limit, offset int) Pagination {
	p := Pagination{
		URL:         urls.AbsoluteURL(r),
		Limit:       limit,
		Offset:      offset,
		TotalCount:  count,
		TotalPages:  int(math.Ceil(float64(count) / float64(limit))),
		CurrentPage: int(math.Floor(float64(offset)/float64(limit))) + 1,
		First:       0,
	}
	p.Last = (p.TotalPages - 1) * p.Limit

	if n := p.Offset + p.Limit; n <= p.Last {
		p.Next = p.Offset + p.Limit
		p.NextPage = p.GetLink(p.Next)
	}
	if n := p.Offset - p.Limit; n >= 0 {
		p.Previous = p.Offset - p.Limit
		p.PreviousPage = p.GetLink(p.Previous)
	}

	p.FirstPage = p.GetLink(p.First)
	p.LastPage = p.GetLink(p.Last)
	p.PageLinks = p.GetPageLinks()

	return p
}

// GetPaginationLinks returns a list of Link instances suitable for pagination.
func GetPaginationLinks(r *http.Request, p Pagination) []Link {
	uri := urls.AbsoluteURL(r)
	pages := int(math.Ceil(float64(p.TotalCount) / float64(p.Limit)))
	lastOffset := int(pages-1) * p.Limit
	prevOffset := p.Offset - p.Limit
	nextOffset := p.Offset + p.Limit

	links := []Link{}
	getLink := func(rel string, offset int) Link {
		u := *uri
		q := u.Query()
		q.Set("limit", strconv.Itoa(p.Limit))
		q.Set("offset", strconv.Itoa(offset))
		u.RawQuery = q.Encode()
		return NewLink(u.String()).WithRel(rel)
	}

	if prevOffset >= 0 {
		links = append(links, getLink("previous", prevOffset))
	}
	if nextOffset <= lastOffset {
		links = append(links, getLink("next", nextOffset))
	}
	links = append(links, getLink("first", 0), getLink("last", lastOffset))

	return links
}

// SendPaginationHeaders compute and set the pagination headers.
func SendPaginationHeaders(
	w http.ResponseWriter, r *http.Request,
	p Pagination,
) {
	pages := int(math.Ceil(float64(p.TotalCount) / float64(p.Limit)))
	page := int(math.Floor(float64(p.Offset)/float64(p.Limit))) + 1

	for _, link := range GetPaginationLinks(r, p) {
		link.Write(w)
	}

	w.Header().Set("Total-Count", strconv.Itoa(p.TotalCount))
	w.Header().Set("Total-Pages", strconv.Itoa(pages))
	w.Header().Set("Current-Page", strconv.Itoa(page))
}
