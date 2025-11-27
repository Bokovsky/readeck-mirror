// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package epub creates EPUB files.
package epub

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
)

const containerXML = `<?xml version="1.0" encoding="UTF-8" ?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

// Writer is an EPUB container.
// It's basically a wrapper around a zip.Writer with some
// additional methods to handle epub specificities.
type Writer struct {
	*zip.Writer
	pkg Package
}

// New creates a new EpubFile instance.
func New(w io.Writer) *Writer {
	c := &Writer{
		Writer: zip.NewWriter(w),
		pkg:    NewPackage(),
	}

	c.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestSpeed)
	})

	return c
}

// Bootstrap adds the necessary files to the EPUB container.
func (c *Writer) Bootstrap() (err error) {
	if err = c.addFile(
		"mimetype", zip.Store, strings.NewReader("application/epub+zip"),
	); err != nil {
		return
	}

	if err = c.addDirectory("META-INF"); err != nil {
		return
	}

	if err = c.addFile(
		"META-INF/container.xml", zip.Deflate, strings.NewReader(containerXML),
	); err != nil {
		return
	}

	if err = c.addDirectory("OEBPS"); err != nil {
		return
	}

	return c.Flush()
}

// SetTitle sets the book's title.
func (c *Writer) SetTitle(value string) {
	c.pkg.Metadata.Title = value
}

// SetLanguage sets the book's main language.
func (c *Writer) SetLanguage(value string) {
	c.pkg.Metadata.Language = value
}

// SetID sets the book's unique ID.
func (c *Writer) SetID(value string) {
	c.pkg.Metadata.Identifier.Value = value
}

// SetCreator sets the book's creator.
func (c *Writer) SetCreator(value string) {
	c.pkg.Metadata.Creator = value
}

// AddChapter adds a new chapter to the book.
func (c *Writer) AddChapter(id, title, name string, r io.Reader) error {
	c.pkg.Manifest.Items = append(c.pkg.Manifest.Items, ManifestItem{
		ID:        id,
		Href:      name,
		MediaType: "application/xhtml+xml",
	})
	c.pkg.Spine.Items = append(c.pkg.Spine.Items, SpineItem{
		IDRef: id,
		Title: title,
		Src:   name,
	})
	return c.addFile(path.Join("OEBPS", name), zip.Deflate, r)
}

// AddImage adds an image to the book with automatic media type detection.
func (c *Writer) AddImage(id, name string, r io.Reader) error {
	buf := new(bytes.Buffer)
	mtype, err := mimetype.DetectReader(io.TeeReader(r, buf))
	if err != nil {
		return err
	}

	c.pkg.Manifest.Items = append(c.pkg.Manifest.Items, ManifestItem{
		ID:        id,
		Href:      name,
		MediaType: mtype.String(),
	})
	return c.addFile(
		path.Join("OEBPS", name),
		zip.Store,
		io.MultiReader(buf, r),
	)
}

// AddFile adds a file to the book.
func (c *Writer) AddFile(id, name, mediaType string, r io.Reader) error {
	c.pkg.Manifest.Items = append(c.pkg.Manifest.Items, ManifestItem{
		ID:        id,
		Href:      name,
		MediaType: mediaType,
	})
	return c.addFile(path.Join("OEBPS", name), zip.Deflate, r)
}

// WritePackage finishes the book creation by adding a content.opf and a toc.ncx files
// based on all the files added earlier.
func (c *Writer) WritePackage() error {
	// Write content.opf
	f, err := c.CreateHeader(&zip.FileHeader{
		Method:   zip.Deflate,
		Name:     "OEBPS/content.opf",
		Modified: time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	c.pkg.Manifest.Items = append(c.pkg.Manifest.Items, ManifestItem{
		ID:        "ncx",
		Href:      "toc.ncx",
		MediaType: "application/x-dtbncx+xml",
	})

	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if _, err = f.Write([]byte(xml.Header)); err != nil {
		return err
	}
	if err = enc.Encode(c.pkg); err != nil {
		return err
	}

	// Write toc.ncx
	f, err = c.CreateHeader(&zip.FileHeader{
		Method:   zip.Deflate,
		Name:     "OEBPS/toc.ncx",
		Modified: time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	toc := TOC{
		XMLns:   nsNCX,
		Version: "2005-1",
		Meta: []TOCMeta{
			{Name: "dtb:uid", Content: c.pkg.Metadata.Identifier.Value},
			{Name: "dtb:depth", Content: "1"},
			{Name: "dtb:totalPageCount", Content: "0"},
			{Name: "dtb:maxPageNumber", Content: "0"},
		},
		Nav: []TOCNav{},
	}

	for _, x := range c.pkg.Spine.Items {
		toc.Nav = append(toc.Nav, TOCNav{
			ID:    x.IDRef,
			Title: x.Title,
			Src:   TOCNavSrc{Src: x.Src},
		})
	}

	enc = xml.NewEncoder(f)
	enc.Indent("", "  ")
	if _, err = fmt.Fprint(f, xml.Header, ncxDoctype); err != nil {
		return err
	}
	if err = enc.Encode(toc); err != nil {
		return err
	}

	return c.Flush()
}

// addDirectory adds a new directory to the zip container.
func (c *Writer) addDirectory(name string) error {
	if !strings.HasSuffix(name, "/") {
		name += "/"
	}

	if _, err := c.CreateHeader(&zip.FileHeader{
		Method:   zip.Deflate,
		Name:     name,
		Modified: time.Now().UTC(),
	}); err != nil {
		return err
	}

	return c.Flush()
}

// addFile adds a new file to the container.
func (c *Writer) addFile(name string, method uint16, r io.Reader) error {
	f, err := c.CreateHeader(&zip.FileHeader{
		Method:   method,
		Name:     name,
		Modified: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, r); err != nil {
		return err
	}

	return c.Flush()
}
