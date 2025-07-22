// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package cookbook

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/textproto"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	bookmark_tasks "codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	"codeberg.org/readeck/readeck/pkg/extract/contentscripts"
	"codeberg.org/readeck/readeck/pkg/extract/meta"
	"codeberg.org/readeck/readeck/pkg/forms"
	"codeberg.org/readeck/readeck/pkg/http/accept"
)

// cookbookAPI is the base cookbook api router.
type cookbookAPI struct {
	chi.Router
	srv *server.Server
}

// newCookbookAPI returns a CookbokAPI with all the routes
// set up.
func newCookbookAPI(s *server.Server) *cookbookAPI {
	r := server.AuthenticatedRouter()

	api := &cookbookAPI{Router: r, srv: s}
	r.With(server.WithPermission("api:cookbook", "read")).Group(func(r chi.Router) {
		r.Get("/urls", api.urlList)
		r.Get("/extract", api.extract)
		r.Post("/extract", api.extract)
	})

	return api
}

type extractForm struct {
	*forms.Form
}

func newExtractForm() *extractForm {
	return &extractForm{forms.Must(
		context.Background(),
		forms.NewTextField("url", forms.Required, forms.IsURL("http", "https")),
		forms.NewFileField("data"),
	)}
}

func (f *extractForm) bind(r *http.Request) {
	forms.BindURL(f, r)
	if !f.IsValid() || r.Method == http.MethodGet {
		return
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), "text/") {
		f.Get("data").Set(&bodyOpener{r})
	}
}

type bodyOpener struct {
	r *http.Request
}

func (o *bodyOpener) Open() (io.ReadCloser, error) { return o.r.Body, nil }
func (o *bodyOpener) Filename() string             { return "" }
func (o *bodyOpener) Size() int64                  { return o.r.ContentLength }
func (o *bodyOpener) Header() textproto.MIMEHeader { return textproto.MIMEHeader(o.r.Header) }

func (api *cookbookAPI) getExtractor(uri string, r *http.Request) *extract.Extractor {
	proxyList := make([]extract.ProxyMatcher, len(configs.Config.Extractor.ProxyMatch))
	for i, x := range configs.Config.Extractor.ProxyMatch {
		proxyList[i] = x
	}

	ex, err := extract.New(
		uri,
		extract.SetLogger(slog.Default(), slog.LevelDebug-10,
			slog.String("@id", server.GetReqID(r)),
		),
		extract.SetDeniedIPs(configs.ExtractorDeniedIPs()),
		extract.SetProxyList(proxyList),
	)
	if err != nil {
		panic(err)
	}

	ex.AddProcessors(
		contentscripts.LoadScripts(
			bookmarks.GetContentScripts(server.Log(r))...,
		),
		meta.ExtractMeta,
		meta.ExtractOembed,
		contentscripts.ProcessMeta,
		meta.SetDropProperties,
		bookmark_tasks.OriginalLinkProcessor,
		meta.ExtractFavicon,
		meta.ExtractPicture,
		contentscripts.LoadSiteConfig,
		contentscripts.ReplaceStrings,
		contentscripts.FindContentPage,
		contentscripts.ExtractAuthor,
		contentscripts.ExtractDate,
		contentscripts.FindNextPage,
		contentscripts.ExtractBody,
		contentscripts.StripTags,
		contentscripts.GoToNextPage,
		contentscripts.ProcessDom("initialDocument"),
		contents.StripHeadingAnchors,
		contents.ExtractInlineSVGs,
		contents.Readability(),
		contentscripts.ProcessDom("finalDocument"),
		bookmark_tasks.CleanDomProcessor,
		contents.Text,
		archiveProcessor,
	)

	return ex
}

func (api *cookbookAPI) getExtractResult(ex *extract.Extractor) *extractResult {
	ex.Run()
	runtime.GC()

	drop := ex.Drop()

	res := &extractResult{
		URL:           drop.UnescapedURL(),
		Logs:          ex.Logs,
		Errors:        []string{},
		Meta:          drop.Meta,
		Properties:    drop.Properties,
		Domain:        drop.Domain,
		Title:         drop.Title,
		Authors:       drop.Authors,
		Site:          drop.URL.Hostname(),
		SiteName:      drop.Site,
		Lang:          drop.Lang,
		TextDirection: drop.TextDirection,
		Date:          &drop.Date,
		DocumentType:  drop.DocumentType,
		Description:   drop.Description,
		HTML:          string(ex.HTML),
		Text:          ex.Text,
		Images:        map[string]*extractImg{},
		Links:         []any{},
	}

	if drop.IsMedia() {
		res.Embed = drop.Meta.LookupGet("oembed.html")
	}

	for _, x := range ex.Errors() {
		res.Errors = append(res.Errors, x.Error())
	}
	if res.Date.IsZero() {
		res.Date = nil
	}

	for k, p := range drop.Pictures {
		res.Images[k] = &extractImg{
			Encoded: fmt.Sprintf("data:%s;base64,%s", p.Type, p.Encoded()),
			Size:    p.Size,
		}
	}

	for _, link := range bookmark_tasks.GetExtractedLinks(ex.Context) {
		res.Links = append(res.Links, link)
	}

	return res
}

func (api *cookbookAPI) extract(w http.ResponseWriter, r *http.Request) {
	f := newExtractForm()
	f.bind(r)

	if !f.IsValid() {
		server.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	ex := api.getExtractor(f.Get("url").String(), r)

	if !f.Get("data").IsNil() {
		fd, err := f.Get("data").(*forms.FileField).V().Open()
		if err != nil {
			panic(err)
		}
		defer fd.Close() //nolint:errcheck
		if content, err := io.ReadAll(fd); err == nil {
			ex.AddToCache(f.Get("url").String(), map[string]string{
				"Content-Type": "text/html",
			}, content)
		} else {
			slog.Error("caching body", slog.Any("err", err))
		}
	}

	res := api.getExtractResult(ex)

	// Very rough but good enough for our tests
	accepted := accept.NegotiateContentType(r.Header, []string{"application/json", "text/html"}, "application/json")
	if accepted == "text/html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(ex.HTML)
		return
	}

	server.Render(w, r, 200, res)
}

func (api *cookbookAPI) urlList(w http.ResponseWriter, r *http.Request) {
	urls := map[string][]string{}
	i := 0

	err := fs.WalkDir(contentscripts.SiteConfigFiles, ".", func(p string, d fs.DirEntry, err error) error {
		defer func() {
			i++
		}()

		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(d.Name()) != ".json" {
			return nil
		}

		f, err := contentscripts.SiteConfigFiles.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close() //nolint:errcheck

		cfg, err := contentscripts.NewSiteConfig(f)
		if err != nil {
			slog.Error("error parsing file",
				slog.String("cf", d.Name()),
				slog.Any("err", err),
			)
			return nil
		}

		if cfg != nil && len(cfg.Tests) > 0 {
			name := fmt.Sprintf("%d - %s", i, path.Base(d.Name()))
			urls[name] = make([]string, len(cfg.Tests))
			for i := range cfg.Tests {
				urls[name][i] = cfg.Tests[i].URL
			}
		}

		return nil
	})
	if err != nil {
		server.Err(w, r, err)
		return
	}
	server.Render(w, r, http.StatusOK, urls)
}

type extractImg struct {
	Size    [2]int `json:"size"`
	Encoded string `json:"encoded"`
}

type extractResult struct {
	URL           string                 `json:"url"`
	Logs          []string               `json:"logs"`
	Errors        []string               `json:"errors"`
	Meta          extract.DropMeta       `json:"meta"`
	Properties    extract.DropProperties `json:"properties"`
	Domain        string                 `json:"domain"`
	Title         string                 `json:"title"`
	Authors       []string               `json:"authors"`
	Site          string                 `json:"site"`
	SiteName      string                 `json:"site_name"`
	Lang          string                 `json:"lang"`
	TextDirection string                 `json:"text_direction"`
	Date          *time.Time             `json:"date"`
	DocumentType  string                 `json:"document_type"`
	Description   string                 `json:"description"`
	HTML          string                 `json:"html"`
	Text          string                 `json:"text"`
	Embed         string                 `json:"embed"`
	Images        map[string]*extractImg `json:"images"`
	Links         []any                  `json:"links"`
}
