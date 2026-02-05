// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-shiori/dom"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"

	"codeberg.org/readeck/readeck/pkg/bleach"
)

var (
	rxAuthor      = regexp.MustCompile(`^(?i)by(\s*:)?\s+`)
	rxSpaces      = regexp.MustCompile(`\s+`)
	rxTitleSpaces = regexp.MustCompile(`[_-]+`)
	rxSrcsetURL   = regexp.MustCompile(`(?i)(\S+)(\s+[\d.]+[xw])?(\s*(?:,|$))`)

	mediaTypes = []string{"photo", "video", "audio", "music"}
)

// Drop is the result of a content extraction of one resource.
type Drop struct {
	URL          *url.URL
	Domain       string
	ContentType  string
	Charset      string
	DocumentType string

	Title         string
	Description   string
	Authors       []string
	Site          string
	Lang          string
	TextDirection string
	Date          time.Time

	Header     http.Header
	Meta       DropMeta
	Properties DropProperties
	Body       []byte `json:"-"`

	Pictures map[string]*Picture
}

// NewDrop returns a Drop instance.
func NewDrop(src *url.URL) *Drop {
	d := &Drop{
		Meta:       DropMeta{},
		Properties: DropProperties{},
		Authors:    []string{},
		Body:       []byte{},
		Pictures:   map[string]*Picture{},
	}
	d.SetURL(src)
	return d
}

// SetURL sets the Drop's URL and Domain properties in their unicode versions.
func (d *Drop) SetURL(src *url.URL) {
	if src == nil {
		d.URL = nil
		d.Domain = ""
		d.Site = ""
		return
	}

	uri := new(url.URL)
	*uri = *src

	// Remove port when it's not needed
	// Note: only numeric ports are valid in [url.URL].
	port := uri.Port()
	if uri.Scheme == "http" && port == "80" || uri.Scheme == "https" && port == "443" {
		port = ""
		// we want to keep the brackets on ipv6 here
		uri.Host = uri.Host[:strings.LastIndexByte(uri.Host, ':')]
	}

	hostname := uri.Hostname()

	if ip, err := netip.ParseAddr(hostname); err == nil {
		// Hostname is an IP address. Shorten the address and use it as the domain.
		s := ip.String()
		if ip.Is6() {
			uri.Host = "[" + s + "]"
		} else {
			uri.Host = s
		}
		if port != "" {
			uri.Host += ":" + port
		}

		d.Domain = s
	} else {
		// Always encode the URL to unicode
		if host, err := idna.ToUnicode(uri.Host); err == nil {
			uri.Host = host
		}
		d.Domain, _ = publicsuffix.EffectiveTLDPlusOne(uri.Hostname())
	}

	if d.Domain == "" {
		d.Domain = hostname
	}

	d.URL = uri
}

// Load loads the remote URL and retrieve data.
func (d *Drop) Load(client *http.Client) error {
	if d.URL == nil {
		return errors.New("No document URL")
	}

	if len(d.Body) > 0 {
		// If we have a body already, we don't load anything and
		// just go with it.
		d.Site = d.URL.Hostname()
		d.ContentType = "text/html"
		d.Charset = "utf-8"
		return nil
	}

	if client == nil {
		client = http.DefaultClient
		defer client.CloseIdleConnections()
	}

	var err error
	var rsp *http.Response

	ctx := WithRequestType(context.Background(), PageRequest)
	if rsp, err = Fetch(ctx, client, d.URL.String()); err != nil {
		return err
	}
	defer rsp.Body.Close() //nolint:errcheck

	// Save headers
	d.Header = rsp.Header

	// Set final URL in case it was redirected
	d.SetURL(rsp.Request.URL)

	// Set mime type
	d.ContentType, _, _ = mime.ParseMediaType(rsp.Header.Get("content-type"))

	// Set site
	d.Site = d.URL.Hostname()

	if rsp.StatusCode/100 != 2 {
		return fmt.Errorf("Invalid status code (%d)", rsp.StatusCode)
	}

	switch {
	case d.IsHTML():
		return d.loadBody(rsp)
	case d.ContentType == "text/plain":
		return d.loadTextPlain(rsp)
	case strings.HasPrefix(d.ContentType, "image/"):
		return d.loadImage(rsp)
	default:
		return fmt.Errorf("unsupported content-type: %q", rsp.Header.Get("content-type"))
	}
}

// IsHTML returns true when the MIME type of the resource is one of the known HTML types, and also
// when it's "application/xml" since that might be an XHTML document.
func (d *Drop) IsHTML() bool {
	switch d.ContentType {
	case "text/html", "application/xhtml+xml", "application/xml":
		return true
	default:
		return false
	}
}

// IsMedia returns true when the document type is a media type.
func (d *Drop) IsMedia() bool {
	return slices.Contains(mediaTypes, d.DocumentType)
}

// UnescapedURL returns the Drop's URL unescaped, for storage.
func (d *Drop) UnescapedURL() string {
	var (
		u   string
		err error
	)
	if u, err = url.PathUnescape(d.URL.String()); err != nil {
		return d.URL.String()
	}

	return u
}

// AddAuthors add authors to the author list, ignoring potential
// duplicates.
func (d *Drop) AddAuthors(values ...string) {
	keys := map[string]string{}
	for _, v := range d.Authors {
		keys[strings.ToLower(v)] = v
	}
	for _, v := range values {
		v = strings.TrimSpace(v)
		v = rxSpaces.ReplaceAllLiteralString(v, " ")
		v = rxAuthor.ReplaceAllString(v, "")
		if _, ok := keys[strings.ToLower(v)]; !ok {
			keys[strings.ToLower(v)] = v
		}
	}
	res := make([]string, len(keys))
	i := 0
	for _, v := range keys {
		res[i] = v
		i++
	}

	sort.Strings(res)
	d.Authors = res
}

// loadBody loads the document body and try to convert
// it to UTF-8 when encoding is different.
func (d *Drop) loadBody(rsp *http.Response) error {
	r, encName, err := HTMLReader(rsp.Body, rsp.Header.Get("content-type"))
	if err != nil {
		return fmt.Errorf("HTMLReader error: %w", err)
	}

	var body []byte
	if body, err = io.ReadAll(r); err != nil {
		return err
	}

	// Eventually set the original charset and UTF8 body
	d.Charset = encName
	d.Body = []byte(bleach.SanitizeString(string(body)))

	return nil
}

// loadTextPlain loads a plain text content and wraps it in an HTML document with
// a title.
func (d *Drop) loadTextPlain(rsp *http.Response) error {
	err := d.loadBody(rsp)
	if err != nil {
		return err
	}

	title := strings.TrimSuffix(path.Base(d.URL.Path), path.Ext(d.URL.Path))
	title = rxTitleSpaces.ReplaceAllLiteralString(title, " ")
	d.Body = []byte(
		fmt.Sprintf("<html><head><title>%s</title><body><pre>%s",
			title,
			string(d.Body),
		),
	)
	d.ContentType = "text/html"

	return nil
}

// loadImage sets the type to "photo", try to get a title and set the picture URL
// to the drop's URL.
func (d *Drop) loadImage(_ *http.Response) error {
	d.Meta.Add("x.picture_url", d.URL.String())
	d.Title = strings.TrimSuffix(path.Base(d.URL.Path), path.Ext(d.URL.Path))
	d.Title = rxTitleSpaces.ReplaceAllLiteralString(d.Title, " ")
	d.DocumentType = "photo"

	return nil
}

// fixRelativeURIs normalizes every href, src, srcset and poster
// URI values.
// It uses <base href> when present.
func (d *Drop) fixRelativeURIs(m *ProcessMessage) {
	top := m.Dom
	if top == nil {
		return
	}

	attrs := []string{"href", "src", "poster"}
	baseURL := &url.URL{}
	*baseURL = *d.URL

	m.Log().Debug("fix relative links", slog.String("base", baseURL.String()))

	// <base href> exists, we resolve its URL and set the new baseURL.
	if baseMeta := dom.QuerySelector(top, "base[href]"); baseMeta != nil {
		b := dom.GetAttribute(baseMeta, "href")
		if b != "" {
			if buri, err := url.Parse(b); err == nil {
				baseURL = baseURL.ResolveReference(buri)
				m.Log().Debug("found base tag", slog.String("url", baseURL.String()))
			}
		}
	}

	// walk through anything with href, src, poster attribute.
	for _, attr := range attrs {
		dom.ForEachNode(dom.QuerySelectorAll(top, "["+attr+"]"), func(n *html.Node, _ int) {
			newURI := toAbsoluteURI(dom.GetAttribute(n, attr), baseURL)
			dom.SetAttribute(n, attr, newURI)
		})
	}

	// srcset handler
	dom.ForEachNode(dom.QuerySelectorAll(top, "[srcset]"), func(n *html.Node, _ int) {
		srcset := dom.GetAttribute(n, "srcset")
		if srcset == "" {
			return
		}
		newSrcSet := rxSrcsetURL.ReplaceAllStringFunc(srcset, func(s string) string {
			p := rxSrcsetURL.FindStringSubmatch(s)
			return toAbsoluteURI(p[1], baseURL) + p[2] + p[3]
		})
		dom.SetAttribute(n, "srcset", newSrcSet)
	})

	// make fragments to the same document relative
	dom.ForEachNode(dom.QuerySelectorAll(top, "a[href]"), func(n *html.Node, _ int) {
		attr := dom.GetAttribute(n, "href")
		if attr == "" {
			return
		}
		if strings.HasPrefix(attr, "#") {
			return
		}
		uri, err := url.Parse(dom.GetAttribute(n, "href"))
		if err != nil {
			return
		}
		if fragment := uri.Fragment; fragment != "" {
			if uri.RawFragment != "" {
				fragment = uri.RawFragment
			}
			uri.Fragment = ""
			tmp := new(url.URL)
			*tmp = *baseURL
			tmp.Fragment = ""
			if uri.String() == tmp.String() {
				dom.SetAttribute(n, "href", "#"+fragment)
			}
		}
	})
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

// scanForCharset scans HTML tags from r until it finds the name of the character set declared in
// either the "charset" attribute or by a "http-equiv" tag.
func scanForCharset(r io.Reader) string {
	z := html.NewTokenizer(r)
	for {
		switch z.Next() {
		case html.ErrorToken:
			return ""
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.DataAtom != atom.Meta {
				continue
			}
			isContentType := false
			var metaContent string
			for _, attr := range t.Attr {
				switch attr.Key {
				case "charset":
					return attr.Val
				case "content":
					metaContent = attr.Val
				case "http-equiv":
					isContentType = strings.EqualFold(attr.Val, "content-type")
				}
			}
			if !isContentType || metaContent == "" {
				continue
			}
			if _, params, err := mime.ParseMediaType(metaContent); err == nil {
				if cs, ok := params["charset"]; ok {
					return cs
				}
			}
		}
	}
}

func scanXML(r io.Reader, isTranscoded bool) (encoding.Encoding, string, bool) {
	xd := xml.NewDecoder(r)

	var isHTML bool
	var enc encoding.Encoding
	var encName string

	// This callback will activate when a processing instruction with "encoding=..." is found.
	xd.CharsetReader = func(charsetName string, input io.Reader) (io.Reader, error) {
		if isTranscoded {
			return input, nil
		}
		enc, encName = charset.Lookup(charsetName)
		return transform.NewReader(input, enc.NewDecoder()), nil
	}

	// Read XML tokens until the first element.
	for {
		t, err := xd.Token()
		if err != nil {
			break
		}
		if el, ok := t.(xml.StartElement); ok {
			isHTML = el.Name.Local == "html" && el.Name.Space == "http://www.w3.org/1999/xhtml"
			break
		}
	}

	return enc, encName, isHTML
}

// HTMLReader wraps r in a reader that automatically transcodes a HTML document from non-UTF-8
// encoding to UTF-8. The original encoding is either determined from contentType value, or by
// scanning the initial 1–3kB from the document to look for things like BOM or for the meta charset
// declaration.
func HTMLReader(r io.Reader, contentType string) (io.Reader, string, error) {
	enc := encoding.Nop
	var encName string
	certain := false

	mimeType, params, err := mime.ParseMediaType(contentType)
	if err == nil {
		if cs, ok := params["charset"]; ok {
			if e, name := charset.Lookup(cs); e != nil {
				enc = e
				encName = name
				certain = true
			}
		}
	}

	var buf bytes.Buffer
	if _, err := io.CopyN(&buf, r, 1024); err != nil && err != io.EOF {
		return nil, "", err
	}

	switch mimeType {
	case "application/xml", "application/xhtml+xml":
		var xmlReader io.Reader = bytes.NewReader(buf.Bytes())
		if certain && enc != encoding.Nop {
			xmlReader = transform.NewReader(xmlReader, enc.NewDecoder())
		}
		e, name, isHTML := scanXML(xmlReader, certain)
		if !isHTML && mimeType == "application/xml" {
			return nil, "", fmt.Errorf("html tag not found in %s document", mimeType)
		}
		if e != nil && !certain {
			enc = e
			encName = name
			certain = true
		}
	}

	if !certain {
		// Determine encoding (fast way)
		enc, encName, certain = charset.DetermineEncoding(buf.Bytes(), contentType)
	}

	// When encoding is not 100% certain, we resort to find the charset
	// parsing part of the received HTML. More than recommended
	// by the HTMLWG, since 1024 bytes is often not enough.
	if !certain {
		if _, err := io.CopyN(&buf, r, 2048); err != nil && err != io.EOF {
			return nil, "", err
		}
		if cs := scanForCharset(bytes.NewReader(buf.Bytes())); cs != "" {
			if e, name := charset.Lookup(cs); e != nil {
				enc = e
				encName = name
			}
		}
	}

	newReader := io.MultiReader(&buf, r)
	if enc != encoding.Nop {
		newReader = transform.NewReader(newReader, enc.NewDecoder())
	}
	return newReader, encName, nil
}

// DropProperties contains the raw properties of an extracted page.
type DropProperties map[string]any

// DropMeta is a map of list of strings that contains the
// collected metadata.
type DropMeta map[string][]string

// Add adds a value to the raw metadata list.
func (m DropMeta) Add(name, value string) {
	_, ok := m[name]
	if ok {
		m[name] = append(m[name], value)
	} else {
		m[name] = []string{value}
	}
}

// Lookup returns all the found values for the
// provided metadata names.
func (m DropMeta) Lookup(names ...string) []string {
	for _, x := range names {
		v, ok := m[x]
		if ok {
			return v
		}
	}

	return []string{}
}

// LookupGet returns the first value found for the
// provided metadata names.
func (m DropMeta) LookupGet(names ...string) string {
	r := m.Lookup(names...)
	if len(r) > 0 {
		return r[0]
	}
	return ""
}
