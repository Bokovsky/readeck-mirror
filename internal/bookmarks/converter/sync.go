// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package converter

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"gopkg.in/yaml.v3"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
)

// SyncOption is a [SyncExporter] option.
type SyncOption func(*SyncExporter)

// SyncExporter is a content exporter for content sync.
// It exports every item from the select dataset as a multipart/alternative
// response into a writer.
type SyncExporter struct {
	dataset.HTMLConverter
	withJSON       bool
	withHTML       bool
	withMarkdown   bool
	withResources  bool
	resourcePrefix string
}

// WithSyncJSON enables or disables JSON information export.
func WithSyncJSON(v bool) SyncOption {
	return func(e *SyncExporter) {
		e.withJSON = v
	}
}

// WithSyncHTML enables or disables HTML (articles) export.
func WithSyncHTML(v bool) SyncOption {
	return func(e *SyncExporter) {
		e.withHTML = v
	}
}

// WithSyncMarkdown enables or disables markdown export.
func WithSyncMarkdown(v bool) SyncOption {
	return func(e *SyncExporter) {
		e.withMarkdown = v
	}
}

// WithSyncResources enables or disables exporting images.
func WithSyncResources(v bool) SyncOption {
	return func(e *SyncExporter) {
		e.withResources = v
	}
}

// WithSyncResourcePrefix set the resource prefix put in front
// of every bookmark's resource. It also triggers a replacement
// in the bookmark's article when there is one.
// "%" is placeholder for the bookmark's ID.
func WithSyncResourcePrefix(v string) SyncOption {
	return func(e *SyncExporter) {
		e.resourcePrefix = v
	}
}

// NewSyncExporter returns a new [SyncExporter].
func NewSyncExporter(options ...SyncOption) SyncExporter {
	e := SyncExporter{
		HTMLConverter: dataset.HTMLConverter{},
	}

	for _, fn := range options {
		fn(&e)
	}
	return e
}

// IterExport implements [IterExporter].
// It writes parts to a multipart/mixed response.
func (e SyncExporter) IterExport(ctx context.Context, w io.Writer, _ *http.Request, bookmarkSeq *dataset.BookmarkIterator) error {
	mp := multipart.NewWriter(w)
	defer mp.Close() //nolint:errcheck
	if w, ok := w.(http.ResponseWriter); ok {
		w.Header().Set("Content-Type", `multipart/mixed; boundary="`+mp.Boundary()+`"`)
	}

	ctx = dataset.WithAnnotationTag(ctx, "rd-annotation", nil)

	for b, err := range bookmarkSeq.Items {
		if err != nil {
			return err
		}

		if err = b.SetEmbed(); err != nil {
			return err
		}

		ctx := dataset.WithURLReplacer(ctx, e.urlReplacer(b))
		if e.withJSON {
			if err = e.writeJSON(ctx, mp, b); err != nil {
				return err
			}
		}
		if e.withHTML && b.HasArticle {
			if err = e.writeHTML(ctx, mp, b); err != nil {
				return err
			}
		}
		if e.withMarkdown {
			if err = e.writeMarkdown(ctx, mp, b); err != nil {
				return err
			}
		}
		if e.withResources {
			if err = e.writeResources(ctx, mp, b); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e SyncExporter) urlReplacer(b *dataset.Bookmark) dataset.URLReplacerFunc {
	if e.resourcePrefix == "" {
		return func(_ *bookmarks.Bookmark) func(name string) string {
			return b.MediaURL
		}
	}

	return func(b *bookmarks.Bookmark) func(name string) string {
		p := path.Clean(e.resourcePrefix)
		p = strings.ReplaceAll(p, "%", b.UID)

		return func(name string) string {
			return path.Join(p, path.Base(name))
		}
	}
}

func (e SyncExporter) writeJSON(_ context.Context, mp *multipart.Writer, b *dataset.Bookmark) error {
	part, err := mp.CreatePart(textproto.MIMEHeader{
		"Bookmark-Id":         []string{b.UID},
		"Type":                []string{"json"},
		"Content-Type":        []string{"application/json; charset=utf-8"},
		"Location":            []string{b.Href},
		"Filename":            []string{"info.json"},
		"Content-Disposition": []string{`attachment; filename="info.json"`},
		"Date":                []string{b.Created.Format(time.RFC3339)},
		"Last-Modified":       []string{b.Updated.Format(time.RFC3339)},
	})
	if err != nil {
		return err
	}

	return json.NewEncoder(part).Encode(b)
}

func (e SyncExporter) writeHTML(ctx context.Context, mp *multipart.Writer, b *dataset.Bookmark) error {
	part, err := mp.CreatePart(textproto.MIMEHeader{
		"Bookmark-Id":         []string{b.UID},
		"Type":                []string{"html"},
		"Content-Type":        []string{"text/html; charset=utf-8"},
		"Location":            []string{b.Href + "/article"},
		"Filename":            []string{"index.html"},
		"Content-Disposition": []string{`attachment; filename="index.html"`},
		"Date":                []string{b.Created.Format(time.RFC3339)},
		"Last-Modified":       []string{b.Updated.Format(time.RFC3339)},
	})
	if err != nil {
		return err
	}

	reader, err := e.GetArticle(ctx, b.Bookmark)
	if err != nil {
		return err
	}

	_, err = io.Copy(part, reader)
	return err
}

func (e SyncExporter) writeMarkdown(ctx context.Context, mp *multipart.Writer, b *dataset.Bookmark) error {
	part, err := mp.CreatePart(textproto.MIMEHeader{
		"Bookmark-Id":         []string{b.UID},
		"Type":                []string{"markdown"},
		"Content-Type":        []string{"text/markdown; charset=utf-8"},
		"Filename":            []string{"index.md"},
		"Content-Disposition": []string{`attachment; filename="index.md"`},
		"Date":                []string{b.Created.Format(time.RFC3339)},
		"Last-Modified":       []string{b.Updated.Format(time.RFC3339)},
	})
	if err != nil {
		return err
	}

	intro := new(bytes.Buffer)
	intro.WriteString("---\n")
	meta := mdMeta{
		Title:   b.Title,
		Saved:   b.Created.Format(time.DateOnly),
		Website: b.Site,
		Source:  b.URL,
		Authors: b.Authors,
		Labels:  b.Labels,
	}
	if b.Published != nil {
		meta.Published = b.Published.Format(time.DateOnly)
	}
	enc := yaml.NewEncoder(intro)
	enc.SetIndent(0)
	if err = enc.Encode(meta); err != nil {
		return err
	}
	intro.WriteString("---\n\n")

	if img, ok := b.Files["image"]; ok {
		fmt.Fprintf(intro, "![](%s)\n\n", e.urlReplacer(b)(b.Bookmark)(path.Base(img.Name)))
	}

	if b.DocumentType == "video" {
		fmt.Fprintf(intro, "[Video on %s](%s)\n\n", b.SiteName, b.URL)
	}

	reader, err := e.GetArticle(ctx, b.Bookmark)
	if err != nil {
		return err
	}
	md, err := html2md.ConvertReader(reader)
	if err != nil {
		return err
	}

	_, err = io.Copy(part, io.MultiReader(intro, bytes.NewReader(md), bytes.NewReader([]byte("\n"))))
	return err
}

func (e SyncExporter) writeResources(ctx context.Context, mp *multipart.Writer, b *dataset.Bookmark) error {
	bc, err := b.OpenContainer()
	if err != nil {
		return err
	}
	defer bc.Close()

	// Fetch the images
	nosize := [2]int{}
	for group, f := range b.Files {
		if f.Size == nosize { // discard non image files
			continue
		}
		if z, ok := bc.Lookup(f.Name); ok {
			if err := e.writeResource(ctx, group, mp, z, b); err != nil {
				return err
			}
		}
	}

	// Fetch all resources
	for _, x := range bc.ListResources() {
		if err := e.writeResource(ctx, "embedded", mp, x, b); err != nil {
			return err
		}
	}

	return nil
}

func (e SyncExporter) writeResource(_ context.Context, group string, mp *multipart.Writer, resource *zip.File, b *dataset.Bookmark) error {
	r, err := resource.Open()
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck

	buf := new(bytes.Buffer)
	mtype, err := mimetype.DetectReader(io.TeeReader(r, buf))
	if err != nil {
		return err
	}

	src := e.urlReplacer(b)(b.Bookmark)(resource.Name)

	part, err := mp.CreatePart(textproto.MIMEHeader{
		"Bookmark-Id":         []string{b.UID},
		"Type":                []string{"resource"},
		"Path":                []string{src},
		"Filename":            []string{path.Base(src)},
		"Content-Disposition": []string{`attachment; filename="` + path.Base(src) + `"`},
		"Content-Type":        []string{mtype.String()},
		"Location":            []string{b.MediaURL(resource.Name)},
		"Group":               []string{group},
		"Content-Length":      []string{strconv.FormatUint(resource.UncompressedSize64, 10)},
	})
	if err != nil {
		return err
	}

	_, err = io.Copy(part, io.MultiReader(buf, r))
	return err
}
