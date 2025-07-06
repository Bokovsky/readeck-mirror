// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/http/request"
	"github.com/stretchr/testify/require"
)

func TestPagination(t *testing.T) {
	bu, _ := url.Parse("https://localhost/")

	getContext := func() (ctx context.Context) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		request.InitBaseURL(bu)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			ctx = r.Context()
		})).ServeHTTP(w, r)
		return
	}

	tests := []struct {
		p       [3]int
		links   []server.PageLink
		headers http.Header
	}{
		{
			p: [3]int{0, 50, 0},
			links: []server.PageLink{
				{Index: 1, URL: "https://localhost/?limit=50&offset=0"},
			},
			headers: http.Header{
				"Current-Page": {"1"},
				"Link":         {`<https://localhost/?limit=50&offset=0>; rel="first"`},
				"Total-Count":  {"0"},
				"Total-Pages":  {"0"},
			},
		},
		{
			p: [3]int{20, 50, 0},
			headers: http.Header{
				"Current-Page": {"1"},
				"Link":         {`<https://localhost/?limit=50&offset=0>; rel="first"`},
				"Total-Count":  {"20"},
				"Total-Pages":  {"1"},
			},
			links: []server.PageLink{
				{Index: 1, URL: "https://localhost/?limit=50&offset=0"},
			},
		},
		{
			p: [3]int{99, 50, 0},
			headers: http.Header{
				"Current-Page": {"1"},
				"Link": {
					`<https://localhost/?limit=50&offset=50>; rel="next"`,
					`<https://localhost/?limit=50&offset=0>; rel="first"`,
					`<https://localhost/?limit=50&offset=50>; rel="last"`,
				},
				"Total-Count": {"99"},
				"Total-Pages": {"2"},
			},
			links: []server.PageLink{
				{Index: 1, URL: "https://localhost/?limit=50&offset=0"},
				{Index: 2, URL: "https://localhost/?limit=50&offset=50"},
			},
		},
		{
			p: [3]int{99, 50, 50},
			headers: http.Header{
				"Current-Page": {"2"},
				"Link": {
					`<https://localhost/?limit=50&offset=0>; rel="previous"`,
					`<https://localhost/?limit=50&offset=0>; rel="first"`,
					`<https://localhost/?limit=50&offset=50>; rel="last"`,
				},
				"Total-Count": {"99"},
				"Total-Pages": {"2"},
			},
			links: []server.PageLink{
				{Index: 1, URL: "https://localhost/?limit=50&offset=0"},
				{Index: 2, URL: "https://localhost/?limit=50&offset=50"},
			},
		},
		{
			p: [3]int{101, 50, 50},
			headers: http.Header{
				"Current-Page": {"2"},
				"Link": {
					`<https://localhost/?limit=50&offset=0>; rel="previous"`,
					`<https://localhost/?limit=50&offset=100>; rel="next"`,
					`<https://localhost/?limit=50&offset=0>; rel="first"`,
					`<https://localhost/?limit=50&offset=100>; rel="last"`,
				},
				"Total-Count": {"101"},
				"Total-Pages": {"3"},
			},
			links: []server.PageLink{
				{Index: 1, URL: "https://localhost/?limit=50&offset=0"},
				{Index: 2, URL: "https://localhost/?limit=50&offset=50"},
				{Index: 3, URL: "https://localhost/?limit=50&offset=100"},
			},
		},
		{
			p: [3]int{2680, 50, 250},
			headers: http.Header{
				"Current-Page": {"6"},
				"Link": {
					`<https://localhost/?limit=50&offset=200>; rel="previous"`,
					`<https://localhost/?limit=50&offset=300>; rel="next"`,
					`<https://localhost/?limit=50&offset=0>; rel="first"`,
					`<https://localhost/?limit=50&offset=2650>; rel="last"`,
				},
				"Total-Count": {"2680"},
				"Total-Pages": {"54"},
			},
			links: []server.PageLink{
				{Index: 1, URL: "https://localhost/?limit=50&offset=0"},
				{Index: 0},
				{Index: 4, URL: "https://localhost/?limit=50&offset=150"},
				{Index: 5, URL: "https://localhost/?limit=50&offset=200"},
				{Index: 6, URL: "https://localhost/?limit=50&offset=250"},
				{Index: 7, URL: "https://localhost/?limit=50&offset=300"},
				{Index: 8, URL: "https://localhost/?limit=50&offset=350"},
				{Index: 0},
				{Index: 54, URL: "https://localhost/?limit=50&offset=2650"},
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := require.New(t)
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			p := server.NewPagination(getContext(), test.p[0], test.p[1], test.p[2])

			t.Logf("CurrentPage:  %d\n", p.CurrentPage)
			t.Logf("First:        %d\n", p.First)
			t.Logf("Last:         %d\n", p.Last)
			t.Logf("Next:         %d\n", p.Next)
			t.Logf("Previous:     %d\n", p.Previous)
			t.Logf("FirstPage:    %s\n", p.FirstPage)
			t.Logf("LastPage:     %s\n", p.LastPage)
			t.Logf("NextPage:     %s\n", p.NextPage)
			t.Logf("PreviousPage: %s\n", p.PreviousPage)

			for _, x := range server.GetPaginationLinks(r, p) {
				t.Logf("Link rel:%s href: %s", x.Rel, x.URL)
			}
			for _, x := range p.PageLinks {
				t.Logf("Link index:%d href: %s", x.Index, x.URL)
			}

			server.SendPaginationHeaders(w, r, p)
			assert.Equal(test.headers, w.Header())
			assert.Equal(test.links, p.PageLinks)
		})
	}
}
