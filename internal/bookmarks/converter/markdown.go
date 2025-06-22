// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package converter

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/gabriel-vasile/mimetype"
	"golang.org/x/net/html"
	"golang.org/x/net/idna"
	"gopkg.in/yaml.v3"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/http/accept"
	"codeberg.org/readeck/readeck/pkg/utils"
)

var html2md = converter.NewConverter(
	converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(),
		&html2mdAnnotationPlugin{},
	),
)

// MarkdownExporter is an content exporter that produces markdown.
type MarkdownExporter struct {
	HTMLConverter
}

type mdMeta struct {
	Title     string   `yaml:"title,omitempty"`
	Saved     string   `yaml:"saved,omitempty"`
	Published string   `yaml:"published,omitempty"`
	Website   string   `yaml:"website,omitempty"`
	Source    string   `yaml:"source,omitempty"`
	Authors   []string `yaml:"authors,omitempty"`
	Labels    []string `yaml:"labels,omitempty"`
}

type ctxExportTypeKey struct{}

// NewMarkdownExporter returns a new [MarkdownExporter] instance.
func NewMarkdownExporter() MarkdownExporter {
	return MarkdownExporter{
		HTMLConverter: HTMLConverter{},
	}
}

// Export implement [Exporter].
// It can write text only articles (the default) separated by an horizontal rule.
// If the request contains "Accept: multipart/alternative", it returns a multipart response
// that contains images for the exported bookmarks.
func (e MarkdownExporter) Export(ctx context.Context, w io.Writer, r *http.Request, bookmarkList []*bookmarks.Bookmark) error {
	ctx = WithAnnotationTag(ctx, "rd-annotation", nil)
	ctx = server.WithRequest(ctx, r)

	accepted := accept.NegotiateContentType(r.Header, []string{"text/markdown", "application/zip", "multipart/alternative"}, "text/markdown")
	switch accepted {
	case "application/zip":
		return e.exportZip(ctx, w, bookmarkList)
	case "multipart/alternative":
		return e.exportMultipart(ctx, w, bookmarkList)
	default:
		return e.exportTextOnly(ctx, w, bookmarkList)
	}
}

func (e MarkdownExporter) exportTextOnly(ctx context.Context, w io.Writer, bookmarkList []*bookmarks.Bookmark) error {
	if w, ok := w.(http.ResponseWriter); ok {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	}

	for i, b := range bookmarkList {
		c := WithURLReplacer(ctx, func(b *bookmarks.Bookmark) func(name string) string {
			return func(name string) string {
				return urls.AbsoluteURL(server.GetRequest(ctx), "/bm", b.FilePath, name).String()
			}
		})
		if i > 0 {
			fmt.Fprint(w, "\n------------------------------------------------------------\nn") //nolint:errcheck
		}
		if err := e.writeArticle(c, w, b, len(bookmarkList) == 1); err != nil {
			slog.Error("export", slog.Any("err", err))
		}
	}
	return nil
}

func (e MarkdownExporter) exportMultipart(ctx context.Context, w io.Writer, bookmarkList []*bookmarks.Bookmark) error {
	mp := multipart.NewWriter(w)
	defer mp.Close() //nolint:errcheck
	if w, ok := w.(http.ResponseWriter); ok {
		w.Header().Set("Content-Type", `multipart/alternative; boundary="`+mp.Boundary()+`"`)
	}

	ctx = WithURLReplacer(ctx, func(_ *bookmarks.Bookmark) func(name string) string {
		return path.Base
	})
	ctx = context.WithValue(ctx, ctxExportTypeKey{}, "multipart")

	for _, b := range bookmarkList {
		if err := func() error {
			slug := utils.Slug(b.Title)
			part, err := mp.CreatePart(textproto.MIMEHeader{
				"BaseName":            []string{slug},
				"Bookmark-Id":         []string{b.UID},
				"Content-Type":        []string{"text/markdown; charset=utf-8"},
				"Content-Disposition": []string{`attachment; filename="` + slug + `.md"`},
				"Date":                []string{b.Created.Format(time.RFC3339)},
			})
			if err != nil {
				return err
			}
			if err := e.writeArticle(ctx, part, b, true); err != nil {
				return err
			}

			bc, err := b.OpenContainer()
			if err != nil {
				return err
			}
			defer bc.Close()

			// Fetch the image
			if img, ok := b.Files["image"]; ok {
				if z, ok := bc.Lookup(img.Name); ok {
					z.Name = e.getImageURL(ctx, b, z.Name)
					if err := e.writeResource(mp, z, b); err != nil {
						return err
					}
				}
			}

			// Fetch all resources
			for _, x := range bc.ListResources() {
				if err := e.writeResource(mp, x, b); err != nil {
					return err
				}
			}

			return nil
		}(); err != nil {
			slog.Error("export", slog.Any("err", err))
			continue
		}
	}

	return nil
}

func (e MarkdownExporter) exportZip(ctx context.Context, w io.Writer, bookmarkList []*bookmarks.Bookmark) error {
	zw := zip.NewWriter(w)
	defer zw.Close() //nolint:errcheck

	basePath := time.Now().Format(time.DateOnly) + "-readeck-bookmarks"

	if w, ok := w.(http.ResponseWriter); ok {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(
			`attachment; filename="%s.zip"`,
			basePath,
		))
	}

	ctx = WithURLReplacer(ctx, func(_ *bookmarks.Bookmark) func(name string) string {
		return path.Base
	})
	ctx = context.WithValue(ctx, ctxExportTypeKey{}, "multipart")

	if _, err := zw.Create(basePath + "/"); err != nil {
		return err
	}

	copyFromZip := func(src *zip.File, destName string) error {
		src.Name = destName
		fd, err := zw.CreateRaw(&src.FileHeader)
		if err != nil {
			return err
		}
		r, err := src.OpenRaw()
		if err != nil {
			return err
		}
		_, err = io.Copy(fd, r)
		return err
	}

	for _, b := range bookmarkList {
		d, _ := idna.ToASCII(b.Site)
		root := fmt.Sprintf("%s/%s-%s-%s",
			basePath,
			b.Created.Format(time.DateOnly),
			strings.ReplaceAll(d, ".", "-"),
			b.UID,
		)
		if err := func() error {
			if _, err := zw.Create(root + "/"); err != nil {
				return err
			}

			// Create index.md
			fd, err := zw.Create(root + "/index.md")
			if err != nil {
				return err
			}
			if err := e.writeArticle(ctx, fd, b, true); err != nil {
				return err
			}

			bc, err := b.OpenContainer()
			if err != nil {
				return err
			}
			defer bc.Close()

			// Copy the image
			if img, ok := b.Files["image"]; ok {
				if z, ok := bc.Lookup(img.Name); ok {
					if err := copyFromZip(z, root+"/"+e.getImageURL(ctx, b, z.Name)); err != nil {
						return err
					}
				}
			}

			// Copy resources
			for _, x := range bc.ListResources() {
				if err := copyFromZip(x, root+"/"+path.Base(x.Name)); err != nil {
					return err
				}
			}

			return nil
		}(); err != nil {
			slog.Error("export", slog.Any("err", err))
			continue
		}
	}

	return nil
}

func (e MarkdownExporter) getImageURL(ctx context.Context, b *bookmarks.Bookmark, name string) string {
	if s, _ := ctx.Value(ctxExportTypeKey{}).(string); s == "multipart" {
		return b.UID + "-" + path.Base(name)
	}
	return urls.AbsoluteURL(server.GetRequest(ctx), "/bm", b.FilePath, "img", path.Base(name)).String()
}

func (e MarkdownExporter) writeArticle(ctx context.Context, w io.Writer, b *bookmarks.Bookmark, withMeta bool) error {
	reader, err := e.GetArticle(ctx, b)
	if err != nil {
		return err
	}

	intro := new(bytes.Buffer)
	if withMeta {
		fmt.Fprintln(intro, "---")
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
		if err := enc.Encode(meta); err != nil {
			return err
		}
		fmt.Fprint(intro, "---\n\n")
	}

	fmt.Fprintf(intro, "# %s\n\n", b.Title)

	if img, ok := b.Files["image"]; ok {
		fmt.Fprintf(intro, "![](%s)\n\n", e.getImageURL(ctx, b, img.Name))
	}

	if b.DocumentType == "video" {
		fmt.Fprintf(intro, "[Video on %s](%s)\n\n", b.SiteName, b.URL)
	}

	md, err := html2md.ConvertReader(reader)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, io.MultiReader(intro, bytes.NewReader(md)))
	return err
}

func (e MarkdownExporter) writeResource(mp *multipart.Writer, resource *zip.File, b *bookmarks.Bookmark) error {
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

	part, err := mp.CreatePart(textproto.MIMEHeader{
		"Bookmark-Id":         []string{b.UID},
		"Filename":            []string{path.Base(resource.Name)},
		"Content-Disposition": []string{`attachment; filename="` + path.Base(resource.Name) + `"`},
		"Content-Type":        []string{mtype.String()},
	})
	if err != nil {
		return err
	}

	_, err = io.Copy(part, io.MultiReader(buf, r))

	return err
}

// html2mdAnnotationPlugin is an html-to-markdown plugin that converts rd-annotation tags
// to "=={content}==" form, that's compatible with at least Obsidian.
type html2mdAnnotationPlugin struct{}

func (s *html2mdAnnotationPlugin) Name() string {
	return "annotation"
}

func (s *html2mdAnnotationPlugin) Init(conv *converter.Converter) error {
	conv.Register.RendererFor("rd-annotation", converter.TagTypeInline, s.render, converter.PriorityStandard)
	return nil
}

func (s *html2mdAnnotationPlugin) render(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	buf := new(bytes.Buffer)
	ctx.RenderChildNodes(ctx, buf, n)
	content := buf.String()

	if strings.TrimSpace(content) == "" {
		w.WriteString(content) // nolint:errcheck
		return converter.RenderSuccess
	}
	w.WriteString("==" + content + "==") // nolint:errcheck

	return converter.RenderSuccess
}
