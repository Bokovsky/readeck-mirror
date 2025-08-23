// SPDX-FileCopyrightText: © 2020 Radhi Fadlillah
// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: MIT

package archiver

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"iter"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "image/gif"  // GIF decoder
	_ "image/jpeg" // JPEG decoder
	_ "image/png"  // PNG decoder

	_ "golang.org/x/image/bmp"  // BMP decoder
	_ "golang.org/x/image/tiff" // TIFF decoder
	_ "golang.org/x/image/webp" // WEBP decoder

	"github.com/google/uuid"
)

var nullLogger = slog.New(slog.DiscardHandler)

// compressedTypes are the types that are compressed in a zip file,
// on top of text/*.
var compressedTypes = map[string]struct{}{
	"application/javascript":   {},
	"image/svg+xml":            {},
	"font/ttf":                 {},
	"image/vnd.microsoft.icon": {},
	"image/x-icon":             {},
}

// Resource is a remote resource.
type Resource struct {
	url    string
	status int
	saved  bool

	Name        string
	ContentType string
	Width       int
	Height      int
	Size        int64
	Contents    *bytes.Buffer
}

// Saved returns the resource's saved state.
func (c *Resource) Saved() bool {
	return c.saved
}

// URL returns the resource's URL.
func (c *Resource) URL() string {
	return c.url
}

// Value returns the resource value. It's usually [Resource.Name] but
// it can be [Resource.Contents] when it's not a nil value.
func (c *Resource) Value() string {
	if c.Contents != nil {
		return c.Contents.String()
	}
	return c.Name
}

// Collector describes a resource collector.
// Its role is to provide some methods to retrieve and keep track
// of remote resources.
// A collector is orchestrated by [Archiver.fetch], and [Archiver.saveResource].
type Collector interface {
	sync.Locker
	Get(uri string) (*Resource, bool)
	Set(uri string, res *Resource)
	Name(uri string) string
	Fetch(req *http.Request) (*http.Response, error)
	Create(res *Resource) (io.Writer, error)
	Resources() iter.Seq[*Resource]
}

// ConvertCollector describes a collector providing a method
// to transform a response's body and/or the associated resource.
type ConvertCollector interface {
	Convert(ctx context.Context, res *Resource, r io.ReadCloser) (io.ReadCloser, error)
}

// PostWriteCollector describes a collector providing a method
// to perform an action just after writing a resource's content.
type PostWriteCollector interface {
	PostWrite(res *Resource, w io.Writer)
}

// LoggerCollector describes a logger provider.
type LoggerCollector interface {
	Log() *slog.Logger
}

// ClientOptions is a function to set HTTP client's properties.
type ClientOptions func(c *http.Client)

// WithTimeout set the HTTP client's timeout for downloading resources.
func WithTimeout(timeout time.Duration) func(c *http.Client) {
	return func(c *http.Client) {
		c.Timeout = timeout
	}
}

// DownloadCollector is a [Collector] that takes care of keeping track of fetched
// resources and their cached state.
type DownloadCollector struct {
	sync.RWMutex
	client    *http.Client
	resources map[string]*Resource
}

// NewDownloadCollector returns a [DownloadCollector].
func NewDownloadCollector(client *http.Client, options ...ClientOptions) *DownloadCollector {
	res := &DownloadCollector{
		RWMutex:   sync.RWMutex{},
		resources: make(map[string]*Resource),
	}

	// Copy the client so we can change its properties if needed
	res.client = &http.Client{}
	*res.client = *client

	if res.client == nil {
		res.client = http.DefaultClient
	}

	res.client.Timeout = time.Second * 10
	for _, fn := range options {
		fn(res.client)
	}

	return res
}

// Get returns the [*Resource] associated with a given URL.
func (c *DownloadCollector) Get(uri string) (res *Resource, ok bool) {
	c.RLock()
	defer c.RUnlock()
	res, ok = c.resources[uri]
	return
}

// Set sets a [*Resource] for a given URL.
func (c *DownloadCollector) Set(uri string, res *Resource) {
	c.Lock()
	defer c.Unlock()
	c.resources[uri] = res
}

// Fetch calls the collector's HTTP client and returns an [*http.Response].
func (c *DownloadCollector) Fetch(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// Resources returns an [iter.Seq] of all the collected resources.
func (c *DownloadCollector) Resources() iter.Seq[*Resource] {
	return maps.Values(c.resources)
}

// uuidNamer is a UUID based resource namer.
type uuidNamer struct{}

// Name returns a name for a URL, using UUID's URL namespace.
func (c uuidNamer) Name(uri string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(uri)).String()
}

var stdName = uuidNamer{}

// Logger returns the [Collector]'s logger when it's a [LoggerCollector].
// It returns a null logger otherwise.
func Logger(c Collector) *slog.Logger {
	if c, ok := c.(LoggerCollector); ok {
		return c.Log()
	}
	return nullLogger
}

// FileCollector is a [Collector] that saves resources on a filesystem.
type FileCollector struct {
	*DownloadCollector
	uuidNamer
	root string
}

// NewFileCollector returns a new [*FileCollector].
func NewFileCollector(root string, client *http.Client, options ...ClientOptions) *FileCollector {
	return &FileCollector{
		DownloadCollector: NewDownloadCollector(client, options...),
		uuidNamer:         stdName,
		root:              root,
	}
}

// Create implement [Collector]. It creates a new resource [io.Writer] and returns a reader.
// The new reader can simply be the original one or a buffer created after an custom
// transformation. At this point, the [*Resource] properties can change, including its name
// and it will reflect on the final document.
func (c *FileCollector) Create(res *Resource) (io.Writer, error) {
	dest := filepath.Join(c.root, res.Name)

	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return nil, err
	}

	w, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// ZipCollector is a [Collector] that saves resources in a zip file.
type ZipCollector struct {
	*DownloadCollector
	uuidNamer
	zw          *zip.Writer
	directories map[string]struct{}
}

// NewZipCollector returns a [ZipCollector] instance.
// The [zip.Writer] must be open and it's the caller's responsibility to
// close it when done adding files.
func NewZipCollector(zw *zip.Writer, client *http.Client, options ...ClientOptions) *ZipCollector {
	return &ZipCollector{
		DownloadCollector: NewDownloadCollector(client, options...),
		uuidNamer:         stdName,
		zw:                zw,
		directories:       make(map[string]struct{}),
	}
}

// Create implement [Collector]. The returned [io.Writer] is a zip fileWriter.
// It creates the necessary directory entries.
// See [FileCollector.Create] for more information.
func (c *ZipCollector) Create(res *Resource) (io.Writer, error) {
	// Create missing directory entries
	dir := path.Dir(res.Name)
	if _, ok := c.directories[dir]; dir != "." && !ok {
		if _, err := c.zw.CreateHeader(&zip.FileHeader{
			Name:     dir + "/",
			Modified: time.Now().UTC(),
		}); err != nil {
			return nil, err
		}
		if err := c.zw.Flush(); err != nil {
			return nil, err
		}
		c.directories[dir] = struct{}{}
	}

	// Create the writer.
	header := &zip.FileHeader{
		Name:     path.Clean(res.Name),
		Modified: time.Now().UTC(),
		Method:   zip.Store,
	}

	if _, ok := compressedTypes[res.ContentType]; ok || strings.Split(res.ContentType, "/")[0] == "text" {
		header.Method = zip.Deflate
	}

	w, err := c.zw.CreateHeader(header)
	if err != nil {
		return nil, err
	}

	return w, nil
}

// SingleFileCollector is a [Collector] that produces a single HTML file with
// every resource URL base64 encoded.
// Note that it is very memory inneficient and should only be used
// for testing purposes.
type SingleFileCollector struct {
	*DownloadCollector
	uuidNamer
	w io.Writer
}

// NewSingleFileCollector returns a new [SingleFileCollector].
func NewSingleFileCollector(w io.Writer, client *http.Client, options ...ClientOptions) *SingleFileCollector {
	return &SingleFileCollector{
		DownloadCollector: NewDownloadCollector(client, options...),
		uuidNamer:         stdName,
		w:                 w,
	}
}

// Create implements [Collector]. For resources, that is, not index.html, it returns
// a [bytes.Buffer] that will be filled with the resource's content.
func (c *SingleFileCollector) Create(res *Resource) (io.Writer, error) {
	if res.Name != "index.html" {
		return new(bytes.Buffer), nil
	}

	return c.w, nil
}

// PostWrite implements [PostWriteCollector]. For any resource that's not index.html
// it renames it to a data URL using the previously created buffer.
func (c *SingleFileCollector) PostWrite(res *Resource, w io.Writer) {
	if w, ok := w.(*bytes.Buffer); ok {
		if res.Name != "index.html" {
			res.Contents = new(bytes.Buffer)
			res.Contents.WriteString("data:" + res.ContentType + ";base64,")
			enc := base64.NewEncoder(base64.StdEncoding, res.Contents)
			io.Copy(enc, w) //nolint:errcheck
			enc.Close()     //nolint:errcheck
		}
	}
}
