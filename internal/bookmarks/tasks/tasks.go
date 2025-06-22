// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package tasks contains the bookmark and collection related tasks.
package tasks

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"golang.org/x/net/html"
	"golang.org/x/sync/errgroup"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bus"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/archiver"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	"codeberg.org/readeck/readeck/pkg/extract/contentscripts"
	"codeberg.org/readeck/readeck/pkg/extract/meta"
	"codeberg.org/readeck/readeck/pkg/superbus"
	"codeberg.org/readeck/readeck/pkg/utils"
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

var (
	// ExtractPageTask is the bookmark creation task.
	ExtractPageTask superbus.Task
	// DeleteBookmarkTask is the bookmark deletion task.
	DeleteBookmarkTask superbus.Task
	// DeleteCollectionTask is the collection deletion task.
	DeleteCollectionTask superbus.Task
	// DeleteLabelTask is the label deletion task.
	DeleteLabelTask superbus.Task
)

type (
	// MultipartResource contains information loaded from a form/multipart request body.
	MultipartResource struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Data    []byte            `json:"data"`
	}

	// ExtractParams contains the extraction parameters.
	ExtractParams struct {
		BookmarkID int
		RequestID  string
		Resources  []MultipartResource
		FindMain   bool
	}

	// LabelDeleteParams contains the label deletion parameters.
	LabelDeleteParams struct {
		UserID int
		Name   string
	}
)

func init() {
	bus.OnReady(func() {
		ExtractPageTask = bus.Tasks().NewTask(
			"bookmark.create",
			superbus.WithUnmarshall(func(data []byte) interface{} {
				var res ExtractParams
				err := json.Unmarshal(data, &res)
				if err != nil {
					panic(err)
				}
				return res
			}),
			superbus.WithTaskHandler(extractPageHandler),
		)

		DeleteBookmarkTask = bus.Tasks().NewTask(
			"bookmark.delete",
			superbus.WithTaskDelay(20),
			superbus.WithUnmarshall(func(data []byte) interface{} {
				var res int
				err := json.Unmarshal(data, &res)
				if err != nil {
					panic(err)
				}
				return res
			}),
			superbus.WithTaskHandler(deleteBookmarkHandler),
		)

		DeleteCollectionTask = bus.Tasks().NewTask(
			"collection.delete",
			superbus.WithTaskDelay(20),
			superbus.WithUnmarshall(func(data []byte) interface{} {
				var res int
				err := json.Unmarshal(data, &res)
				if err != nil {
					panic(err)
				}
				return res
			}),
			superbus.WithTaskHandler(deleteCollectionHandler),
		)

		DeleteLabelTask = bus.Tasks().NewTask(
			"label.delete",
			superbus.WithTaskDelay(20),
			superbus.WithUnmarshall(func(data []byte) interface{} {
				var res LabelDeleteParams
				err := json.Unmarshal(data, &res)
				if err != nil {
					panic(err)
				}
				return res
			}),
			superbus.WithTaskHandler(deleteLabelHandler),
		)
	})
}

// ExtractPage is the public function that run an extraction synchronously.
// Caution: it will panic and should only be run insisde another task.
func ExtractPage(params ExtractParams) {
	extractPageHandler(params)
}

func deleteBookmarkHandler(data interface{}) {
	id := data.(int)
	logger := slog.With(slog.Int("id", id))

	logger.Debug("deleting bookmark")
	b, err := bookmarks.Bookmarks.GetOne(goqu.C("id").Eq(id))
	if err != nil {
		logger.Error("bookmark retrieve", slog.Any("err", err))
		return
	}

	if err := b.Delete(); err != nil {
		logger.Error("bookmark removal", slog.Any("err", err))
		return
	}

	logger.Info("bookmark removed")
}

func deleteCollectionHandler(data interface{}) {
	id := data.(int)
	logger := slog.With(slog.Int("id", id))

	logger.Debug("deleting collection")

	c, err := bookmarks.Collections.GetOne(goqu.C("id").Eq(id))
	if err != nil {
		logger.Error("collection retrieve", slog.Any("err", err))
		return
	}

	if err := c.Delete(); err != nil {
		logger.Error("collection removal", slog.Any("err", err))
		return
	}

	logger.Info("collection removed")
}

func deleteLabelHandler(data interface{}) {
	params := data.(LabelDeleteParams)
	logger := slog.With(
		slog.Int("user", params.UserID),
		slog.String("label", params.Name),
	)
	logger.Debug("deleting label")

	u, err := users.Users.GetOne(goqu.C("id").Eq(params.UserID))
	if err != nil {
		logger.Error("user retrieve", slog.Any("err", err))
		return
	}

	if _, err = bookmarks.Bookmarks.RenameLabel(u, params.Name, ""); err != nil {
		logger.Error("label remove", slog.Any("err", err))
		return
	}

	logger.Info("label removed")
}

func extractPageHandler(data interface{}) {
	var b *bookmarks.Bookmark
	var err error

	params := data.(ExtractParams)

	var resourceCount int
	saved := false
	logger := slog.With(
		slog.String("@id", params.RequestID),
		slog.Int("bookmark_id", params.BookmarkID),
		slog.Bool("find_main", params.FindMain),
	)
	logger.Debug("starting extraction")
	start := time.Now()

	defer func() {
		if b == nil {
			return
		}

		// Recover from any error that could have arose
		if r := recover(); r != nil {
			logger.Error("error during extraction", slog.Any("recover", r))
			debug.PrintStack()
			b.State = bookmarks.StateError
			b.Errors = append(b.Errors, fmt.Sprintf("%v", r))
			saved = false
		}

		// Never stay hanging
		if b.State == bookmarks.StateLoading {
			b.State = bookmarks.StateLoaded
			saved = false
		}

		// Then save the whole thing
		if !saved {
			if err := b.Save(); err != nil {
				logger.Error("saving bookmark", slog.Any("err", err))
			}
		}

		metricCreation.WithLabelValues(b.StateName()).Inc()
		metricTiming.WithLabelValues(b.StateName()).Observe(time.Since(start).Seconds())
		metricResources.Observe(float64(resourceCount))
		runtime.GC()
	}()

	b, err = bookmarks.Bookmarks.GetOne(goqu.C("id").Eq(params.BookmarkID))
	if err != nil {
		logger.Error("", slog.Any("err", err))
		return
	}

	proxyList := make([]extract.ProxyMatcher, len(configs.Config.Extractor.ProxyMatch))
	for i, x := range configs.Config.Extractor.ProxyMatch {
		proxyList[i] = x
	}

	ex, err := extract.New(
		b.URL,
		extract.SetLogger(slog.Default(),
			slog.String("@id", params.RequestID),
			slog.Int("bookmark_id", b.ID),
		),
		extract.SetDeniedIPs(configs.ExtractorDeniedIPs()),
		extract.SetProxyList(proxyList),
	)
	if err != nil {
		logger.Error("", slog.Any("err", err))
		return
	}

	for _, x := range params.Resources {
		// Inject resource in client's cache
		ex.AddToCache(x.URL, x.Headers, x.Data)
	}

	ex.AddProcessors(
		contentscripts.LoadScripts(
			bookmarks.GetContentScripts(ex.Log())...,
		),
		meta.ExtractMeta,
		meta.ExtractOembed,
		contentscripts.ProcessMeta,
		meta.SetDropProperties,
		OriginalLinkProcessor,
		meta.ExtractFavicon,
		meta.ExtractPicture,
		contentscripts.LoadSiteConfig,
		conditionnalProcessor(params.FindMain, contentscripts.ReplaceStrings),
		// Only when the page is not in cache
		conditionnalProcessor(!ex.IsInCache(b.URL), contentscripts.FindContentPage),
		conditionnalProcessor(!ex.IsInCache(b.URL), contentscripts.FindNextPage),
		contentscripts.ExtractAuthor,
		contentscripts.ExtractDate,
		// Default is true but the request can override this
		conditionnalProcessor(params.FindMain, contentscripts.ExtractBody),
		conditionnalProcessor(params.FindMain, contentscripts.StripTags),
		conditionnalProcessor(params.FindMain, contentscripts.GoToNextPage),
		contents.ExtractInlineSVGs,
		contents.ConvertVideoEmbeds,
		contents.Readability(),
		CleanDomProcessor,
		extractLinksProcessor,
		contents.Text,
		saveBookmark(b, &saved, &resourceCount),
		fetchLinksProcessor(b),
	)

	if !params.FindMain {
		// Disable readability when find_main=0
		contents.EnableReadability(ex, false)
	}

	ex.Run()
}

func conditionnalProcessor(test bool, p extract.Processor) extract.Processor {
	if test {
		return p
	}
	return nilProcessor
}

var nilProcessor = func(_ *extract.ProcessMessage, next extract.Processor) extract.Processor {
	return next
}

// saveBookmark is one last step of the extraction process, it saves the bookmark
// and marks it ready for reading.
// Other steps can still perform tasks later.
func saveBookmark(b *bookmarks.Bookmark, saved *bool, resourceCount *int) extract.Processor {
	return func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
		if m.Step() != extract.StepDone {
			return next
		}

		ex := m.Extractor
		var err error
		drop := ex.Drop()
		if drop == nil {
			return next
		}

		b.Updated = time.Now()
		b.URL = drop.UnescapedURL()
		b.State = bookmarks.StateLoaded
		b.Domain = drop.Domain
		b.Site = drop.URL.Hostname()
		b.SiteName = drop.Site
		b.Authors = types.Strings{}
		b.Lang = drop.Lang
		b.TextDirection = drop.TextDirection
		b.DocumentType = drop.DocumentType
		b.Description = utils.NormalizeSpaces(drop.Description)
		b.Text = utils.NormalizeSpaces(ex.Text)
		b.WordCount = len(strings.Fields(b.Text))

		if b.Title == "" {
			b.Title = drop.Title
		}
		b.Title = utils.NormalizeSpaces(b.Title)

		for _, x := range drop.Authors {
			b.Authors = append(b.Authors, x)
		}

		for _, x := range ex.Errors() {
			b.Errors = append(b.Errors, x.Error())
		}

		if !drop.Date.IsZero() {
			b.Published = &drop.Date
		}

		if drop.IsMedia() {
			b.Embed = drop.Meta.LookupGet("oembed.html")
		}

		if duration, err := strconv.Atoi(drop.Meta.LookupGet("x.duration")); err == nil {
			b.Duration = duration
		}

		b.Links = GetExtractedLinks(ex.Context)

		// Run the archiver
		var arc *archiver.Archiver
		if len(ex.HTML) > 0 && ex.Drop().IsHTML() {
			arc, err = bookmarks.NewArchive(context.TODO(), ex)
			if err != nil {
				m.Log().Error("archiver error", slog.Any("err", err))
			}
		}

		if arc != nil {
			*resourceCount = len(arc.Cache)
		}

		// Create the zip file
		err = createZipFile(b, ex, arc)
		if err != nil {
			// If something goes really wrong, cleanup after ourselves
			b.Errors = append(b.Errors, err.Error())
			b.RemoveFiles()
			b.FilePath = ""
			b.Files = bookmarks.BookmarkFiles{}
		}

		// All good? Save now
		if err := b.Save(); err != nil {
			m.Log().Error("", slog.Any("err", err))
			return next
		}
		*saved = true
		return next
	}
}

// fetchLinksProcessor retrieves the link list (from extractLinksProcessor) and
// process all of them to get some information (content type, title when possible...)
// The link list is then saved into the bookmark.
// This processor MUST run after saveBookmark.
func fetchLinksProcessor(b *bookmarks.Bookmark) extract.Processor {
	return func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
		if m.Step() != extract.StepDone {
			return next
		}

		links, ok := checkExtractLinks(m.Extractor.Context)
		if !ok {
			return next
		}

		g, _ := errgroup.WithContext(context.TODO())
		g.SetLimit(10)
		for i := range links {
			g.Go(func() error {
				m.Log().Debug("extract link", slog.String("url", links[i].URL))
				URL, err := url.Parse(links[i].URL)
				if err != nil {
					return err
				}
				d := extract.NewDrop(URL)
				err = d.Load(m.Extractor.Client())
				if err != nil {
					m.Log().Warn("extract link error",
						slog.String("url", d.URL.String()),
						slog.Any("err", err),
					)
				}

				links[i].ContentType = d.ContentType
				links[i].IsPage = d.IsHTML()

				if !links[i].IsPage {
					return nil
				}

				node, err := html.Parse(bytes.NewReader(d.Body))
				if err != nil {
					m.Log().Warn("extract link error",
						slog.String("url", d.URL.String()),
						slog.Any("err", err),
					)
					return nil
				}
				meta := meta.ParseMeta(node)
				title := meta.LookupGet("graph.title", "tiwtter.title", "html.title")
				m.Log().Debug("link",
					slog.String("url", d.URL.String()),
					slog.String("title", title),
				)

				links[i].Title = title
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			m.Log().Error("extract links", slog.Any("err", err))
		}

		links = slices.CompactFunc(links, func(a, b bookmarks.BookmarkLink) bool {
			return a.URL == b.URL
		})

		if len(links) == 0 {
			return next
		}

		if err := b.Update(map[string]any{"links": links}); err != nil {
			m.Log().Error("", slog.Any("err", err))
		}

		return next
	}
}

func createZipFile(b *bookmarks.Bookmark, ex *extract.Extractor, arc *archiver.Archiver) error {
	// Fail fast
	fileURL, err := b.GetBaseFileURL()
	if err != nil {
		return err
	}
	zipFile := filepath.Join(bookmarks.StoragePath(), fileURL+".zip")

	b.FilePath = fileURL
	b.Files = bookmarks.BookmarkFiles{}

	// Create the zip file
	z := zipfs.NewZipRW(nil, nil, 0)
	defer func() {
		if err := z.Close(); err != nil {
			panic(err)
		}
	}()

	if err = os.MkdirAll(filepath.Dir(zipFile), 0o750); err != nil {
		return err
	}
	if err = z.AddDestFile(zipFile); err != nil {
		return err
	}

	// Add images
	for k, p := range ex.Drop().Pictures {
		name := path.Join("img", p.Name(k))
		if err = z.Add(
			&zip.FileHeader{Name: name},
			bytes.NewReader(p.Bytes()),
		); err != nil {
			return err
		}
		b.Files[k] = &bookmarks.BookmarkFile{Name: name, Type: p.Type, Size: p.Size}
	}

	// Add HTML content
	if arc != nil && len(arc.Result) > 0 {
		if err = z.Add(
			&zip.FileHeader{Name: "index.html", Method: zip.Deflate},
			bytes.NewReader(arc.Result),
		); err != nil {
			return err
		}
		b.Files["article"] = &bookmarks.BookmarkFile{Name: "index.html"}
	}

	// Add assets
	if arc != nil && len(arc.Cache) > 0 {
		for uri, asset := range arc.Cache {
			fname := path.Join(bookmarks.ResourceDirName(), bookmarks.GetURLfilename(uri, asset.ContentType))
			if err = z.Add(
				&zip.FileHeader{Name: fname},
				bytes.NewReader(asset.Data),
			); err != nil {
				return err
			}
		}
	}

	// Add the log
	if err = z.Add(
		&zip.FileHeader{Name: "log", Method: zip.Deflate},
		strings.NewReader(strings.Join(ex.Logs, "\n")),
	); err != nil {
		return err
	}
	b.Files["log"] = &bookmarks.BookmarkFile{Name: "log"}

	// Add the metadata
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	if err = enc.Encode(ex.Drop()); err != nil {
		return err
	}
	if err = z.Add(
		&zip.FileHeader{Name: "props.json", Method: zip.Deflate},
		buf,
	); err != nil {
		return err
	}
	b.Files["props"] = &bookmarks.BookmarkFile{Name: "props.json"}

	return nil
}
