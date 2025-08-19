// SPDX-FileCopyrightText: © 2020 Radhi Fadlillah
// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: MIT

// Package archiver provides functions to archive a full HTML page with its assets.
package archiver

import (
	"bytes"
	"context"
	"image"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/sync/semaphore"
	"golang.org/x/sync/singleflight"

	"github.com/gabriel-vasile/mimetype"
)

// ArchiveFlag is an archiver feature to enable.
type ArchiveFlag uint8

const (
	// EnableCSS enables extraction of CSS files and tags.
	EnableCSS ArchiveFlag = 1 << iota

	// EnableEmbeds enables extraction of Embedes contents.
	EnableEmbeds

	// EnableJS enables extraction of JavaScript contents.
	EnableJS

	// EnableMedia enables extraction of media contents
	// other than image.
	EnableMedia

	// EnableImages enables extraction of images.
	EnableImages

	// EnableFonts enables font extraction.
	EnableFonts

	// EnableDataAttributes enables data attributes in HTML elements.
	EnableDataAttributes

	// EnableBestImage enables an image sorting process to find the
	// best suitable image from srcset and picture>source elements.
	EnableBestImage
)

const levelTrace = slog.LevelDebug - 10

type ctxArchiverKey struct{}

// IsArchiverRequest returns true when an [http.Request] was made using the archiver.
func IsArchiverRequest(req *http.Request) bool {
	res, _ := req.Context().Value(ctxArchiverKey{}).(bool)
	return res
}

// Archiver is the core of the archiver process. It hold the flags [ArchiveFlag] and
// a [Collector] that caches collected content.
type Archiver struct {
	collector Collector
	flags     ArchiveFlag

	fetchGroup     *singleflight.Group
	fetchSemaphore *semaphore.Weighted
	writeLock      *sync.Mutex
}

// Option is a function that can set an [Archiver] options.
type Option func(arc *Archiver)

// WithCollector sets a [Collector] to an [Archiver].
func WithCollector(collector Collector) Option {
	return func(arc *Archiver) {
		arc.collector = collector
	}
}

// WithFlags sets [ArchiveFlag] to an [Archiver].
func WithFlags(flags ArchiveFlag) Option {
	return func(arc *Archiver) {
		arc.flags = flags
	}
}

// WithConcurrency set the maximum concurrent downloads that
// can take place during archiving.
func WithConcurrency(v int64) Option {
	return func(arc *Archiver) {
		arc.fetchSemaphore = semaphore.NewWeighted(v)
	}
}

// New creates a new [Archiver].
func New(options ...Option) *Archiver {
	arc := &Archiver{
		flags:      EnableImages | EnableEmbeds,
		fetchGroup: &singleflight.Group{},
		writeLock:  &sync.Mutex{},
	}

	for _, fn := range options {
		fn(arc)
	}

	if arc.fetchSemaphore == nil {
		arc.fetchSemaphore = semaphore.NewWeighted(6)
	}

	if arc.collector == nil {
		arc.collector = NewFileCollector("", http.DefaultClient)
	}

	return arc
}

func (arc *Archiver) log() *slog.Logger {
	return Logger(arc.collector)
}

// ArchiveDocument runs the archiver on a document [*html.Node] for a given URL.
// If name is empty, the [Collector] will generate one.
func (arc *Archiver) ArchiveDocument(ctx context.Context, doc *html.Node, uri *url.URL, name string) (err error) {
	if res, ok := arc.collector.Get(uri.String()); ok && res.Saved() {
		return nil
	}

	if err = arc.processHTML(ctx, doc, uri); err != nil {
		return
	}

	return arc.saveHTML(ctx, doc, uri.String(), name)
}

// ArchiveReader runs the archiver on an [io.Reader] for a given URL.
// If name is empty, the [Collector] will generate one.
func (arc *Archiver) ArchiveReader(ctx context.Context, r io.Reader, uri *url.URL, name string) error {
	if res, ok := arc.collector.Get(uri.String()); ok && res.Saved() {
		return nil
	}

	doc, err := html.Parse(r)
	if err != nil {
		return err
	}

	return arc.ArchiveDocument(ctx, doc, uri, name)
}

// fetch fetches an URL using the [Collector.fetch] method.
// It takes care of "data:" URLs and will attempt to sniff the content type
// and image dimensions when the received content is an image.
func (arc *Archiver) fetch(ctx context.Context, uri string, headers http.Header) (io.ReadCloser, *Resource, error) {
	type fetchResult struct {
		body     io.ReadCloser
		resource *Resource
	}

	var rsp *http.Response
	var err error

	uri = requestURI(uri)

	// Start resource fetching using a group. Any concurrent call for the same URL
	// will wait for the first one to finish.
	result, err, _ := arc.fetchGroup.Do(uri, func() (interface{}, error) {
		if err = arc.fetchSemaphore.Acquire(ctx, 1); err != nil {
			return nil, err
		}
		defer arc.fetchSemaphore.Release(1)

		if strings.HasPrefix(uri, "data:") {
			// data: URI, build a response out of it
			if rsp, err = loadDataURI(uri); err != nil { //nolint
				return nil, err
			}
		} else {
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
			if err != nil {
				return nil, err
			}
			if headers != nil {
				r.Header = headers
			}

			if referrer := getReferrerContext(ctx); referrer != "" {
				r.Header.Set("Referer", referrer)
			}

			ctx = context.WithValue(ctx, ctxArchiverKey{}, true)
			if rsp, err = arc.collector.Fetch(r.WithContext(ctx)); err != nil { //nolint
				return nil, err
			}
		}

		contentType, _, _ := strings.Cut(rsp.Header.Get("content-type"), ";")
		contentType = strings.TrimSpace(contentType)

		// Here, we're goinf to sniff the content-type and get image
		// dimensions. It consumes a few KB from the body, which can be closed
		// later when that's the only information we need.
		body := rsp.Body

		// Try to detect binary or unspecified types
		if contentType == "" || strings.EqualFold(contentType, "binary/octet-stream") {
			buf := new(bytes.Buffer)
			sniffed, err := mimetype.DetectReader(io.TeeReader(body, buf))
			if err != nil {
				rsp.Body.Close() //nolint:errcheck
				return nil, err
			}
			contentType = sniffed.String()
			body = MultiReadCloser(buf, body)
		}

		// Collect image dimensions
		w, h := 0, 0
		if strings.HasPrefix(contentType, "image/") && contentType != "image/svg+xml" {
			buf := new(bytes.Buffer)
			imageConfig, _, err := image.DecodeConfig(io.TeeReader(body, buf))
			if err == nil {
				w = imageConfig.Width
				h = imageConfig.Height
			}
			body = MultiReadCloser(buf, body)
		}

		return fetchResult{
			body: body,
			resource: &Resource{
				Name:        arc.collector.Name(uri) + GetExtension(contentType),
				url:         uri,
				Size:        rsp.ContentLength,
				ContentType: contentType,
				status:      rsp.StatusCode,
				Width:       w,
				Height:      h,
			},
		}, nil
	})
	arc.fetchGroup.Forget(uri)

	if err != nil {
		return nil, nil, err
	}

	return result.(fetchResult).body, result.(fetchResult).resource, nil
}

// fetchInfo returns a [*Resource] for the given URL. It checks first if the resource
// has already been stored, regardless of its saved status.
func (arc *Archiver) fetchInfo(ctx context.Context, uri string, headers http.Header) (*Resource, error) {
	uri = requestURI(uri)

	if res, ok := arc.collector.Get(uri); ok {
		return res, nil
	}

	arc.log().LogAttrs(ctx, levelTrace, "fetch info", slog.Any("url", URLLogValue(uri)))
	body, res, err := arc.fetch(ctx, uri, headers)
	if body != nil {
		// We don't need the body at this stage
		body.Close() //nolint:errcheck
	}
	if err != nil {
		return nil, err
	}

	arc.collector.Set(uri, res)
	return res, nil
}

// saveResource saves a [io.Reader] into the collector's storage.
func (arc *Archiver) saveResource(ctx context.Context, r io.ReadCloser, res *Resource) (*Resource, error) {
	// Never write more than one file at a time.
	arc.writeLock.Lock()
	defer arc.writeLock.Unlock()

	uri := requestURI(res.URL())
	if res, ok := arc.collector.Get(uri); ok && res.Saved() {
		return res, nil
	}

	// We may convert the result and/or its reader.
	if c, ok := arc.collector.(ConvertCollector); ok {
		var err error
		r, err = c.Convert(ctx, res, r)
		if err != nil {
			return nil, err
		}
		defer r.Close() //nolint:errcheck
	}

	// Create the writer.
	w, err := arc.collector.Create(res)

	defer func() {
		// If our writer is an [io.Closer], we close it
		// when we're done writing.
		if w, ok := w.(io.Closer); ok {
			if err := w.Close(); err != nil {
				arc.log().Error("could not close file",
					slog.String("name", res.Name),
					slog.Any("err", err),
				)
			}
		}
	}()
	if err != nil {
		return nil, err
	}

	if _, err = io.Copy(w, r); err != nil {
		return nil, err
	}

	if c, ok := arc.collector.(PostWriteCollector); ok {
		c.PostWrite(res, w)
	}

	arc.log().LogAttrs(ctx, slog.LevelDebug, "save resource",
		slog.String("name", res.Name),
		slog.Any("url", URLLogValue(res.URL())),
	)

	res.saved = true
	arc.collector.Set(uri, res)
	return res, nil
}
