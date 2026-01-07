// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/pkg/ctxr"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

type (
	ctxTaskParamsKey struct{}
)

var withTaskParams, getTaskParams = ctxr.WithGetter[tasks.ExtractParams](ctxTaskParamsKey{})

func TestBookmarkAPIShare(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := app.Client(WithToken("user"))
	bookmarkID := app.Users["user"].Bookmarks[0].UID

	client.RT(t,
		WithMethod("GET"),
		WithTarget("/api/bookmarks/"+bookmarkID+"/share/link"),
		AssertStatus(201),
	)

	publicPath := client.History[0].Response.Redirect
	require.NotEmpty(t, publicPath, "public path is set")

	client.RT(t,
		WithTarget(publicPath),
		AssertStatus(200),
		AssertContains(`Shared by user`),
	)
}

func TestBookmarkCreate(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := app.Client(WithToken("user"))

	assertTask := func(t *testing.T, rsp *Response) {
		assert := require.New(t)

		assert.Len(Events().Records("task"), 1)
		evt := map[string]interface{}{}
		assert.NoError(json.Unmarshal(Events().Records("task")[0], &evt))
		assert.Equal("bookmark.create", evt["name"])

		params := GetTaskPayload[tasks.ExtractParams](
			t,
			fmt.Sprintf("tasks:bookmark.create:%v", evt["id"]),
			tasks.ExtractPageTask,
		)

		b, err := bookmarks.Bookmarks.GetOne(goqu.C("id").Eq(params.BookmarkID))
		assert.NoError(err)
		assert.Equal("https://example.org/", b.URL)
		assert.Equal(bookmarks.StateLoading, b.State)

		// Store params in context so we can retrieve them in later assertions
		ctx := rsp.Request.Context()
		ctx = withTaskParams(ctx, params)
		rsp.Request = rsp.Request.WithContext(ctx)
	}

	tests := []struct {
		name    string
		options []TestOption
	}{
		{
			"no url",
			[]TestOption{
				WithBody(url.Values{}),
				AssertStatus(422),
			},
		},
		{
			"url only JSON",
			[]TestOption{
				WithBody(map[string]string{"url": "https://example.org/"}),
				AssertStatus(202),
				WithAssert(assertTask),
			},
		},
		{
			"url only as form",
			[]TestOption{
				WithBody(url.Values{"url": {"https://example.org/"}}),
				AssertStatus(202),
				WithAssert(assertTask),
			},
		},
		{
			"url only multipart",
			[]TestOption{
				//nolint:errcheck
				func(rt *RequestTest) {
					buf := new(bytes.Buffer)
					mp := multipart.NewWriter(buf)
					mp.WriteField("url", "https://example.org/")
					mp.Close()

					rt.Header.Add("Content-Type", mp.FormDataContentType())
					rt.Body = buf
				},
				AssertStatus(202),
				WithAssert(assertTask),
			},
		},
		{
			"multipart resources",
			[]TestOption{
				//nolint:errcheck
				func(rt *RequestTest) {
					buf := new(bytes.Buffer)
					mp := multipart.NewWriter(buf)
					mp.WriteField("url", "https://example.org/")

					payload := map[string]any{
						"url": "https://example.org/",
						"headers": map[string]string{
							"content-type": "text/html",
						},
					}

					p, _ := mp.CreateFormFile("resource", "index.html")
					enc := json.NewEncoder(p)
					enc.Encode(payload)
					io.Copy(p, strings.NewReader("<p>test"))

					mp.Close()

					rt.Header.Add("Content-Type", mp.FormDataContentType())
					rt.Body = buf
				},
				AssertStatus(202),
				WithAssert(assertTask),
				WithAssert(func(t *testing.T, rsp *Response) {
					assert := require.New(t)
					params := getTaskParams(rsp.Request.Context())
					assert.Len(params.Resources, 1)
					assert.Equal("https://example.org/", params.Resources[0].URL)
					assert.Equal("text/html", params.Resources[0].Header.Get("content-type"))
				}),
			},
		},
		{
			"multipart resources no payload",
			[]TestOption{
				//nolint:errcheck
				func(rt *RequestTest) {
					buf := new(bytes.Buffer)
					mp := multipart.NewWriter(buf)
					mp.WriteField("url", "https://example.org/")

					p, _ := mp.CreateFormFile("resource", "index.html")
					io.Copy(p, strings.NewReader("<p>test"))

					mp.Close()

					rt.Header.Add("Content-Type", mp.FormDataContentType())
					rt.Body = buf
				},
				AssertStatus(202),
				WithAssert(assertTask),
				WithAssert(func(t *testing.T, rsp *Response) {
					require.Empty(t, getTaskParams(rsp.Request.Context()).Resources)
				}),
			},
		},
		{
			"multipart resources no payload URL",
			[]TestOption{
				//nolint:errcheck
				func(rt *RequestTest) {
					buf := new(bytes.Buffer)
					mp := multipart.NewWriter(buf)
					mp.WriteField("url", "https://example.org/")

					p, _ := mp.CreateFormFile("resource", "index.html")
					fmt.Fprintln(p, "{}")
					io.Copy(p, strings.NewReader("<p>test"))

					mp.Close()

					rt.Header.Add("Content-Type", mp.FormDataContentType())
					rt.Body = buf
				},
				AssertStatus(202),
				WithAssert(assertTask),
				WithAssert(func(t *testing.T, rsp *Response) {
					require.Empty(t, getTaskParams(rsp.Request.Context()).Resources)
				}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer Store().Clear()
			defer Events().Clear()

			rt := RT(
				append([]TestOption{
					WithMethod(http.MethodPost),
					WithTarget("/api/bookmarks"),
				}, test.options...)...,
			)
			client.Assert(t, rt)
		})
	}
}
