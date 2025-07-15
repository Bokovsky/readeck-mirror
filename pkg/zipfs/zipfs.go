// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package zipfs provides tools to serve content directly from a zip file to an HTTP response.
// It also provides the necessary tooling to perform changes on an existing zip file.
package zipfs

import (
	"archive/zip"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"
	"time"
)

// ZipRW is a very simple wrapper around zip.Reader and zip.Writer.
// It can be used as a simple zip file creator or as a tool to copy
// files from one zip file to another.
type ZipRW struct {
	dst     io.Writer
	src     io.ReaderAt
	srcSize int64

	zr *zip.Reader
	zw *zip.Writer

	entries []string
}

// NewZipRW returns a new ZipRW using a destination writer and a source reader.
// Both can be null but, obviously, it won't work very well without at least a writer.
func NewZipRW(dst io.Writer, src io.ReaderAt, srcSize int64) *ZipRW {
	return &ZipRW{
		dst:     dst,
		src:     src,
		srcSize: srcSize,
	}
}

func (z *ZipRW) init() (err error) {
	if z.zw == nil && z.dst != nil {
		z.zw = zip.NewWriter(z.dst)
		z.zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(out, flate.BestSpeed)
		})
	}

	if z.zr == nil && z.src != nil {
		z.zr, err = zip.NewReader(z.src, z.srcSize)
	}

	return
}

func (z *ZipRW) get(name string) (*zip.File, error) {
	for _, x := range z.zr.File {
		if x.Name == name && !x.FileInfo().IsDir() {
			return x, nil
		}
	}
	return nil, os.ErrNotExist
}

func (z *ZipRW) makeDirs(filename string) (err error) {
	parts := []string{}

	for d, _ := path.Split(filename); d != ""; d, _ = path.Split(d) {
		d = strings.TrimSuffix(d, "/")
		parts = append(parts, d)
	}
	slices.Reverse(parts)
	for _, x := range parts {
		if slices.Contains(z.entries, x) {
			return fmt.Errorf(`file "%s" already exists`, x)
		}
		if slices.Contains(z.entries, x+"/") {
			continue
		}

		_, err = z.zw.CreateHeader(&zip.FileHeader{
			Method:   zip.Store,
			Name:     x + "/",
			Modified: time.Now().UTC(),
		})
		if err != nil {
			return
		}
		z.entries = append(z.entries, x+"/")
	}
	return
}

// Close closes all writer resources, including the underlying io.Writer and io.Reader
// when they implement their respective closer interfaces.
func (z *ZipRW) Close() error {
	errs := []error{}
	if z.zw != nil {
		errs = append(errs, z.zw.Close())
	}
	if c, ok := z.dst.(io.WriteCloser); ok {
		errs = append(errs, c.Close())
	}
	if c, ok := z.src.(io.ReadCloser); ok {
		errs = append(errs, c.Close())
	}

	return errors.Join(errs...)
}

// AddDestFile adds a destination file to ZipRW.
func (z *ZipRW) AddDestFile(name string) (err error) {
	if z.dst != nil {
		return errors.New("destination already set")
	}
	if z.dst, err = os.Create(name); err != nil {
		return
	}
	return z.init()
}

// AddSourceFile adds a source file to ZipRW.
func (z *ZipRW) AddSourceFile(name string) (err error) {
	if z.src != nil {
		return errors.New("source already set")
	}

	var r *os.File
	if r, err = os.Open(name); err != nil {
		return
	}
	var fi os.FileInfo
	if fi, err = r.Stat(); err != nil {
		return
	}
	z.src = r
	z.srcSize = fi.Size()
	return z.init()
}

// Dest returns the underlying destination [zip.Writer].
func (z *ZipRW) Dest() *zip.Writer {
	return z.zw
}

// Source returns the underlying source [zip.Reader].
func (z *ZipRW) Source() *zip.Reader {
	return z.zr
}

// SrcFiles returns a list of zip.File from the source.
func (z *ZipRW) SrcFiles() []*zip.File {
	if z.zr == nil {
		return nil
	}
	return slices.DeleteFunc(z.zr.File, func(f *zip.File) bool {
		return f.FileInfo().IsDir()
	})
}

func (z *ZipRW) getWriter(h *zip.FileHeader, raw bool) (io.Writer, error) {
	if h.FileInfo().IsDir() {
		return nil, errors.New("cannot add a directory directly")
	}

	h.Name = path.Clean(h.Name)
	if slices.Contains(z.entries, h.Name) {
		return nil, fmt.Errorf(`file "%s" already exists`, h.Name)
	}

	if err := z.init(); err != nil {
		return nil, err
	}

	if z.zw == nil {
		return nil, errors.New("needs a zip writer to add content")
	}

	if err := z.makeDirs(h.Name); err != nil {
		return nil, err
	}

	if h.Modified.IsZero() {
		h.Modified = time.Now().UTC()
	}

	var w io.Writer
	var err error
	if raw {
		w, err = z.zw.CreateRaw(h)
	} else {
		w, err = z.zw.CreateHeader(h)
	}

	return w, err
}

// GetWriter returns a writer for a zipfile entry.
func (z *ZipRW) GetWriter(h *zip.FileHeader) (io.Writer, error) {
	return z.getWriter(h, false)
}

// GetRawWriter returns a raw writer for a zipfile entry.
func (z *ZipRW) GetRawWriter(h *zip.FileHeader) (io.Writer, error) {
	return z.getWriter(h, true)
}

// Add adds a new file to the zip file.
// It creates the necessary directories if needed.
func (z *ZipRW) Add(h *zip.FileHeader, r io.Reader) error {
	w, err := z.GetWriter(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	if err == nil {
		z.entries = append(z.entries, h.Name)
	}

	return err
}

// Copy copies the content of the file in the source to the destination.
func (z *ZipRW) Copy(name string) error {
	if err := z.init(); err != nil {
		return err
	}
	if z.zr == nil || z.zw == nil {
		return errors.New("needs a zip reader and zip writer to copy")
	}

	name = path.Clean(name)
	if slices.Contains(z.entries, name) {
		return fmt.Errorf(`file "%s" already exists`, name)
	}

	src, err := z.get(name)
	if err != nil {
		return err
	}

	if err := z.makeDirs(name); err != nil {
		return err
	}

	r, err := src.OpenRaw()
	if err != nil {
		return err
	}

	w, err := z.zw.CreateRaw(&src.FileHeader)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	z.entries = append(z.entries, name)
	return err
}
