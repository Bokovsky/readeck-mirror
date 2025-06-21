// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package converter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"codeberg.org/readeck/readeck/assets"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/epub"
	"codeberg.org/readeck/readeck/pkg/utils"
)

var uuidURL = uuid.Must(uuid.Parse("6ba7b811-9dad-11d1-80b4-00c04fd430c8"))

// EPUBExporter is a content exporter that produces EPUB files.
type EPUBExporter struct {
	HTMLConverter
	Collection *bookmarks.Collection
}

// NewEPUBExporter returns a new [EPUBExporter] instance.
func NewEPUBExporter() EPUBExporter {
	return EPUBExporter{
		HTMLConverter: HTMLConverter{},
	}
}

// Export implements [Exporter].
// It writes an EPUB file on the provided [io.Writer].
func (e EPUBExporter) Export(ctx context.Context, w io.Writer, r *http.Request, bookmarkList []*bookmarks.Bookmark) error {
	// Define a title, date, siteName and filename
	title := "Readeck Bookmarks"
	date := time.Now()
	siteName := "Readeck"
	if e.Collection != nil {
		title = e.Collection.Name
		siteName = "Readeck Collection"
	} else if len(bookmarkList) == 1 {
		title = bookmarkList[0].Title
		date = bookmarkList[0].Created
		siteName = bookmarkList[0].SiteName
	}

	id := ""
	for _, x := range bookmarkList {
		id += x.UID
	}

	if w, ok := w.(http.ResponseWriter); ok {
		w.Header().Set("Content-Type", "application/epub+zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(
			`attachment; filename="%s-%s.epub"`,
			date.Format(time.DateOnly),
			utils.Slug(strings.TrimSuffix(utils.ShortText(title, 40), "...")),
		))
	}

	m, err := newEpubMaker(w, uuid.NewSHA1(uuidURL, []byte(id)))
	if err != nil {
		return err
	}

	defer func() {
		if err == nil {
			m.SetTitle(title)
			m.SetCreator(siteName)
			err = m.WritePackage()
		}
		m.Close() //nolint:errcheck
	}()

	ctx = WithURLReplacer(ctx, func(_ *bookmarks.Bookmark) func(name string) string {
		return func(name string) string {
			return "./Images/" + path.Base(name)
		}
	})
	for _, b := range bookmarkList {
		if err = m.addBookmark(ctx, r, e, b); err != nil {
			return err
		}
	}

	return nil
}

// epubMaker is a wrapper around epub.Writer with extra methods to
// create an epub file from one or many bookmarks.
type epubMaker struct {
	*epub.Writer
}

// newEpubMaker creates a new EpubMaker instance.
func newEpubMaker(w io.Writer, id uuid.UUID) (*epubMaker, error) {
	m := &epubMaker{epub.New(w)}
	if err := m.Bootstrap(); err != nil {
		return nil, err
	}

	m.SetID(id.URN())
	m.SetTitle("Readeck ebook")
	m.SetLanguage("en")

	if err := m.addStylesheet(); err != nil {
		return nil, err
	}
	return m, nil
}

// addStylesheet adds the stylesheet to the epub file.
func (m *epubMaker) addStylesheet() error {
	f, err := assets.StaticFilesFS().Open("epub.css")
	if err != nil {
		return err
	}
	defer f.Close()
	return m.AddFile("stylesheet", "styles/stylesheet.css", "text/css", f)
}

// addBookmark adds a bookmark, with all its resources, to the epub file.
func (m *epubMaker) addBookmark(ctx context.Context, r *http.Request, e EPUBExporter, b *bookmarks.Bookmark) (err error) {
	var c *bookmarks.BookmarkContainer
	if c, err = b.OpenContainer(); err != nil {
		return
	}
	defer c.Close()

	// Add all the resource files to the book. They are only images for now.
	for _, x := range c.ListResources() {
		err = func() error {
			fp, err := x.Open()
			if err != nil {
				return err
			}
			defer fp.Close() //nolint:errcheck
			return m.AddImage(
				"res-"+strings.TrimSuffix(path.Base(x.Name), path.Ext(x.Name)),
				path.Join("Images", path.Base(x.Name)),
				fp,
			)
		}()
		if err != nil {
			return
		}
	}

	// Build the other resource list
	resources := bookmarks.BookmarkFiles{}
	for k, v := range b.Files {
		if k == "icon" || k == "image" && (b.DocumentType == "photo" || b.DocumentType == "video") {
			resources[k] = &bookmarks.BookmarkFile{
				Name: path.Join("Images", fmt.Sprintf("%s-%s%s", k, b.UID, path.Ext(v.Name))),
				Type: v.Type,
				Size: v.Size,
			}
		}
	}

	// Add all fixed resources (image, icon) to the container
	for k, v := range resources {
		err = func() error {
			fp, err := c.Open(b.Files[k].Name) // original path
			if err != nil {
				return err
			}
			defer fp.Close() //nolint:errcheck

			return m.AddImage(
				fmt.Sprintf("%s-%s", k, b.UID),
				v.Name,
				fp,
			)
		}()
		if err != nil {
			return
		}
	}

	buf := new(bytes.Buffer)
	html, err := e.GetArticle(ctx, b)
	if err != nil {
		return err
	}
	tpl, err := server.GetTemplate("epub/bookmark.jet.html")
	if err != nil {
		return err
	}
	tc := map[string]any{
		"HTML":      html,
		"Item":      b,
		"ItemURL":   urls.AbsoluteURL(r, "/bookmarks", b.UID),
		"Resources": resources,
	}
	if err := tpl.Execute(buf, server.TemplateVars(r), tc); err != nil {
		return err
	}

	return m.AddChapter(
		"page-"+b.UID,
		b.Title,
		b.UID+".html",
		buf,
	)
}
