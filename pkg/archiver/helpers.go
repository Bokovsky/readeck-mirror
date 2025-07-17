// SPDX-FileCopyrightText: © 2020 Radhi Fadlillah
// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: MIT

package archiver

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

var errInvalidBase64URI = errors.New("invalid base64 URI")

var rxStyleURL = regexp.MustCompile(`(?i)^url\((.+)\)$`)

var commonTypes = map[string]string{
	"application/javascript":        ".js",
	"application/json":              ".json",
	"application/ogg":               ".ogx",
	"application/pdf":               ".pdf",
	"application/rtf":               ".rtf",
	"application/vnd.ms-fontobject": ".eot",
	"application/wasm":              ".wasm",
	"application/xhtml+xml":         ".xhtml",
	"application/xml":               ".xml",
	"audio/aac":                     ".aac",
	"audio/midi":                    ".midi",
	"audio/x-midi":                  ".midi",
	"audio/mpeg":                    ".mp3",
	"audio/ogg":                     ".oga",
	"audio/opus":                    ".opus",
	"audio/wav":                     ".wav",
	"audio/webm":                    ".weba",
	"font/otf":                      ".otf",
	"font/ttf":                      ".ttf",
	"font/woff":                     ".woff",
	"font/woff2":                    ".woff2",
	"image/avif":                    ".avif",
	"image/bmp":                     ".bmp",
	"image/gif":                     ".gif",
	"image/jpeg":                    ".jpg",
	"image/png":                     ".png",
	"image/svg+xml":                 ".svg",
	"image/tiff":                    ".tiff",
	"image/vnd.microsoft.icon":      ".ico",
	"image/webp":                    ".webp",
	"image/x-icon":                  ".ico",
	"text/calendar":                 ".ics",
	"text/css":                      ".css",
	"text/csv":                      ".csv",
	"text/html":                     ".html",
	"text/javascript":               ".js",
	"text/plain":                    ".txt",
	"text/xml":                      ".xml",
	"video/mp2t":                    ".ts",
	"video/mp4":                     ".mp4",
	"video/mpeg":                    ".mpeg",
	"video/ogg":                     ".ogv",
	"video/webm":                    ".webm",
	"video/x-msvideo":               ".avi",
}

// URLLogValue is a [slog.LogValuer] for URLs.
// It truncates the string when there too long (ie. data: URLs).
type URLLogValue string

// LogValue implements [slog.LogValuer].
func (s URLLogValue) LogValue() slog.Value {
	if len(s) > 256 {
		return slog.StringValue(string(s)[0:40] + "..." + string(s)[len(s)-40:])
	}

	return slog.StringValue(string(s))
}

type nodeLogValue struct {
	node *html.Node
}

// NodeLogValue is an [slog.LogValuer] for an [*html.Node].
// Its LogValue method renders and truncate the node as HTML.
func NodeLogValue(n *html.Node) slog.LogValuer {
	return &nodeLogValue{n}
}

func (n *nodeLogValue) LogValue() slog.Value {
	if n.node.Type == html.TextNode {
		return slog.StringValue(n.node.Data)
	}

	var tagPreview strings.Builder
	tagPreview.WriteString("<")
	tagPreview.WriteString(n.node.Data)

	hasOtherAttributes := false
	for _, attr := range n.node.Attr {
		switch strings.ToLower(attr.Key) {
		case "id", "class", "rel", "itemprop", "name", "type", "role", "for":
			fmt.Fprintf(&tagPreview, ` %s=%q`, attr.Key, attr.Val)
		case "src", "href":
			val := attr.Val
			if strings.HasPrefix(val, "data:") {
				if v, _, ok := strings.Cut(val, ","); ok {
					val = v + ",***"
				}
			} else if strings.HasPrefix(val, "javascript:") {
				val = "javascript:***"
			}
			fmt.Fprintf(&tagPreview, ` %s=%q`, attr.Key, val)
		default:
			hasOtherAttributes = true
		}
	}
	if hasOtherAttributes {
		tagPreview.WriteString(" ...")
	}

	if n.node.FirstChild == nil {
		tagPreview.WriteString("/")
	}
	tagPreview.WriteString(">")
	return slog.StringValue(tagPreview.String())
}

// GetExtension returns an extension for a given mime type. It defaults
// to .bin when none was found.
func GetExtension(mimeType string) string {
	t, _, _ := strings.Cut(mimeType, ";")
	if ext, ok := commonTypes[t]; ok {
		return ext
	}
	if ext, _ := mime.ExtensionsByType(mimeType); len(ext) > 0 {
		return ext[0]
	}
	return ".bin"
}

func commonPrefix(strs ...string) string {
	longestPrefix := ""
	endPrefix := false

	if len(strs) > 0 {
		sort.Strings(strs)
		first := string(strs[0])
		last := string(strs[len(strs)-1])

		for i := 0; i < len(first); i++ {
			if !endPrefix && string(last[i]) == string(first[i]) {
				longestPrefix += string(last[i])
			} else {
				endPrefix = true
			}
		}
	}
	return longestPrefix
}

func requestURI(s string) (uri string) {
	uri, _, _ = strings.Cut(s, "#")
	return
}

func isValidURL(s string) bool {
	_, err := url.ParseRequestURI(s)
	return err == nil
}

func toAbsoluteURI(uri string, base *url.URL) string {
	if uri == "" {
		return uri
	}
	if strings.HasPrefix(uri, "data:") {
		return uri
	}

	tmp, err := url.Parse(uri)
	if err != nil {
		return uri
	}

	if tmp.Scheme != "" {
		return uri
	}

	return base.ResolveReference(tmp).String()
}

// sanitizeStyleURL sanitizes the URL in CSS by removing `url()`,
// quotation marks.
func sanitizeStyleURL(uri string) string {
	cssURL := rxStyleURL.ReplaceAllString(uri, "$1")
	cssURL = strings.TrimSpace(cssURL)

	if strings.HasPrefix(cssURL, `"`) {
		return strings.Trim(cssURL, `"`)
	}

	if strings.HasPrefix(cssURL, `'`) {
		return strings.Trim(cssURL, `'`)
	}

	return strings.TrimSpace(cssURL)
}

// loadDataURI returns an [http.Response] from a "data:" URI.
// If the URI defines a "base64" encoding, it's decoded using
// [base64.StdEncoding]. The result response has always a status 200,
// a content-type header and an [io.NopCloser] body that wraps
// a buffer with the content.
func loadDataURI(uri string) (*http.Response, error) {
	if !strings.HasPrefix(uri, "data:") {
		return nil, errInvalidBase64URI
	}

	prefix, data, found := strings.Cut(uri, ",")
	if !found {
		return nil, errInvalidBase64URI
	}
	prefix = strings.TrimSpace(prefix)
	data = strings.TrimSpace(data)
	contentType := strings.TrimPrefix(prefix, "data:")

	var res *bytes.Buffer
	var err error

	if !strings.HasSuffix(prefix, ";base64") {
		p, _ := url.PathUnescape(data)
		res = bytes.NewBufferString(p)
	} else {
		res = new(bytes.Buffer)
		dec := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(data))
		_, err = io.Copy(res, dec)
		if err != nil {
			return nil, err
		}
	}

	return &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: int64(res.Len()),
		Header: http.Header{
			"Content-Type": {contentType},
		},
		Body:    io.NopCloser(res),
		Request: &http.Request{},
	}, nil
}

type multiReadCloser struct {
	io.Reader
	closers []io.Closer
}

// MultiReadCloser returns an [io.ReadCloser] that's the concatenation
// of multiple [io.Reader]. It stores a reader resulting from [io.MultiReader]
// and a list of [io.Closer] for the provided readers that implement [io.Closer].
func MultiReadCloser(readers ...io.Reader) io.ReadCloser {
	res := &multiReadCloser{
		Reader: io.MultiReader(readers...),
	}

	for _, r := range readers {
		if r, ok := r.(io.Closer); ok {
			res.closers = append(res.closers, r)
		}
	}

	return res
}

func (mrc *multiReadCloser) Close() error {
	errs := []error{}
	for _, c := range mrc.closers {
		errs = append(errs, c.Close())
	}

	return errors.Join(errs...)
}
