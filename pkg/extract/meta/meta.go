// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package meta provides extract processors to retrieve several meta information
// from a page (meta tags, favicon, pictures...).
package meta

import (
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
	"github.com/araddon/dateparse"
	"github.com/go-shiori/dom"
	"github.com/gobwas/glob"

	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/microdata"
)

var rxOpenGraphType = regexp.MustCompile(`^([^:]*:)?(.+?)(\..*|$)`)

// ExtractMeta is a processor that extracts metadata from the
// document and set the Drop values accordingly.
func ExtractMeta(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Dom == nil || m.Position() > 0 {
		return next
	}

	m.Log().Debug("loading metadata")

	// Set raw meta
	d := m.Extractor.Drop()
	maps.Copy(d.Meta, ParseMeta(m.Dom))
	maps.Copy(d.Properties, ParseProps(m.Dom))

	// Extract JSON-LD and microdata from page
	var md *mdProp
	if x, err := microdata.ParseNode(m.Dom, m.Extractor.URL.String()); err == nil {
		md = &mdProp{x}
	} else {
		m.Log().Error("parse microdata", slog.Any("err", err))
	}

	if md != nil {
		// Add the raw JSON-LD + microdata to properties
		d.Properties["json-ld"] = md.Raw()

		// fetch relevant information
		if headline, ok := md.getProp("Article.name", "*.headline", "{Movie,VideObject}.name").(string); ok {
			d.Title = headline
		}
		if description, ok := md.getProp("{Article,NewsArticle,WebPage}.description", "*.description").(string); ok {
			d.Description = description
		}
		if image, ok := md.getProp("*.{image,image.url,thumbnailUrl}").(string); ok {
			d.Meta.Add("x.picture_url", image)
		}

		if lang, ok := md.getProp("*.{inLanguage,inLanguage.alternateName}").(string); ok {
			d.Lang = lang[0:2]
		}
		if published, ok := md.getProp("*.datePublished").(string); ok {
			if t, err := dateparse.ParseAny(published); err == nil {
				d.Date = t
			}
		}
		if publisher, ok := md.getProp("{*.publisher,*.publisher.name,{Blog,WebSite}.name}").(string); ok {
			d.Site = publisher
		}

		// note: this will stop at the first matches
		// (if we have 3 entries in *.author.name, it won't check Person.name)
		for x := range md.iterProps("*.author.name", "Person.name", "*.comment.author.alternateName") {
			if x, ok := x.(string); ok {
				d.AddAuthors(x)
			}
		}
	}

	if d.Title == "" {
		d.Title = d.Meta.LookupGet(
			"graph.title",
			"twitter.title",
			"schema.headline",
			"html.title",
		)
	}

	if d.Description == "" {
		d.Description = d.Meta.LookupGet(
			"graph.description",
			"twitter.description",
			"html.description",
		)
	}

	// Keep a short description (60 words)
	if parts := strings.Fields(d.Description); len(parts) > 60 {
		d.Description = strings.Join(parts[:60], " ") + "..."
	}

	if len(d.Authors) == 0 {
		d.AddAuthors(d.Meta.Lookup(
			"schema.author",
			"dc.creator",
			"html.author",
			"html.byl",
			"fediverse.creator",
		)...)
	}

	if site := d.Meta.LookupGet(
		"graph.site_name",
		"schema.publisher",
	); site != "" && (d.Site == "" || d.Site == d.URL.Hostname()) {
		d.Site = site
	}

	if d.Lang == "" {
		if lang := d.Meta.LookupGet(
			"html.lang",
			"html.language",
		); len(lang) >= 2 {
			d.Lang = lang[0:2]
		}
	}

	d.TextDirection = d.Meta.LookupGet("html.dir")

	m.Log().Debug("metadata loaded", slog.Int("count", len(d.Meta)))
	return next
}

// SetDropProperties will set some Drop properties bases on the retrieved
// metadata. It must be run after ExtractMeta and ExtractOembed.
func SetDropProperties(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Position() > 0 {
		return next
	}

	d := m.Extractor.Drop()

	if htmlDate := d.Meta.LookupGet("html.date"); d.Date.IsZero() && htmlDate != "" {
		// Set publication date
		d.Date, _ = dateparse.ParseLocal(htmlDate)
	}

	// We might have a picture here
	if d.Meta.LookupGet("dc.type") == "image" {
		d.DocumentType = "photo"
	}

	// Set document type from opengraph value
	if d.DocumentType == "" {
		ogt := d.Meta.LookupGet("graph.type")
		if ogt != "" {
			d.DocumentType = rxOpenGraphType.ReplaceAllString(ogt, "$2")
		}
	}

	// If no authors, try to get them from oembed
	if len(d.Authors) == 0 {
		d.AddAuthors(d.Meta.Lookup("oembed.author_name")...)
	}

	// Same for website name
	if d.Site == "" || d.Site == d.URL.Hostname() {
		if site := d.Meta.LookupGet("oembed.provider_name"); site != "" {
			d.Site = site
		}
	}

	// If we have a picture type, we force the type and set a new meta
	// for the picture url
	otype := d.Meta.LookupGet("oembed.type")
	if d.DocumentType == "photo" || otype == "photo" {
		d.DocumentType = "photo"

		if otype == "photo" {
			d.Meta.Add("x.picture_url", d.Meta.LookupGet("oembed.url"))
		}
	}

	if otype == "video" {
		d.DocumentType = otype
	}

	// Document type is only a predefined set and nothing more
	switch d.DocumentType {
	case "article", "photo", "video":
		// Valid values
	default:
		d.DocumentType = "article"
	}

	m.Log().Info("document type", slog.String("type", d.DocumentType))
	return next
}

func extMeta(k, v, sep string) func(*html.Node) (string, string) {
	return func(n *html.Node) (string, string) {
		_, k, _ := strings.Cut(strings.TrimSpace(dom.GetAttribute(n, k)), sep)
		v := strings.TrimSpace(dom.GetAttribute(n, v))

		// Some attributes may contain HTML, we don't want that
		a, _ := html.Parse(strings.NewReader(v))
		return k, dom.TextContent(a)
	}
}

var specList = []struct {
	name     string
	selector string
	fn       func(*html.Node) (string, string)
}{
	{"html", "//title", func(n *html.Node) (string, string) {
		return "title", dom.TextContent(n)
	}},
	{"html", "/html[@lang]/@lang", func(n *html.Node) (string, string) {
		return "lang", dom.TextContent(n)
	}},
	{"html", "/*[self::html or self::body][@dir]/@dir", func(n *html.Node) (string, string) {
		if c := strings.ToLower(strings.TrimSpace(dom.TextContent(n))); c == "rtl" || c == "ltr" {
			return "dir", c
		}
		return "dir", ""
	}},

	// Common HTML meta tags
	{"html", `//meta[@content][
		@name='author' or
		@name='byl' or
		@name='copyright' or
		@name='date' or
		@name='description' or
		@name='keywords' or
		@name='language' or
		@name='subtitle'
	]`, extMeta("name", "content", "")},

	// Dublin Core
	{"dc", `//meta[@content][
		starts-with(@name, 'DC.') or
		starts-with(@name, 'dc.')
	]`, extMeta("name", "content", ".")},

	// Facebook opengraph
	{
		"graph", "//meta[@content][starts-with(@property, 'og:')]",
		extMeta("property", "content", ":"),
	},
	{
		"graph", "//meta[@content][starts-with(@name, 'og:')]",
		extMeta("name", "content", ":"),
	},

	// Twitter cards
	{
		"twitter", "//meta[@content][starts-with(@name, 'twitter:')]",
		extMeta("name", "content", ":"),
	},

	// Fediverse meta tags
	{
		"fediverse", "//meta[@content][starts-with(@name, 'fediverse:')]",
		extMeta("name", "content", ":"),
	},

	// Header links (excluding icons and stylesheets)
	{"link", `//link[@href][@rel][
		not(contains(translate(@rel, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), 'icon')) and
		not(contains(@rel, 'stylesheet'))
	]`, extMeta("rel", "href", "")},
}

// ParseMeta parses page metadata.
func ParseMeta(doc *html.Node) extract.DropMeta {
	res := extract.DropMeta{}

	for _, x := range specList {
		nodes, _ := htmlquery.QueryAll(doc, x.selector)

		for _, node := range nodes {
			name, value := x.fn(node)
			name = strings.TrimSpace(name)
			value = strings.TrimSpace(value)
			if name == "" || value == "" {
				continue
			}

			name = fmt.Sprintf("%s.%s", x.name, name)
			res.Add(name, value)
		}
	}

	return res
}

var mdGlobCache sync.Map

type mdProp struct {
	*microdata.Microdata
}

// iterProps returns an iterator over [microdata.Node] of type [microdata.Property].
// It uses a list of glob matchers. The loop stops after the first matcher that yields
// results.
// That way, if you need to yield all the result for Article.name and WebSite.name, you
// call iterProps("{Article,WebSite}.name").
// If you want to stop on the first match, you then call iterProps("Article.name", "WebSite.name").
func (md *mdProp) iterProps(matchers ...string) iter.Seq[any] {
	patterns := []glob.Glob{}
	for _, x := range matchers {
		if g, ok := mdGlobCache.Load(x); ok {
			patterns = append(patterns, g.(glob.Glob))
		} else {
			g := glob.MustCompile(x, '.')
			patterns = append(patterns, g)
			mdGlobCache.Store(x, g)
		}
	}

	return func(yield func(any) bool) {
		for _, g := range patterns {
			match := false
			for node := range md.All(func(n *microdata.Node) bool { return n.Type == microdata.Property }) {
				if g.Match(node.Path) {
					if !yield(node.Data) {
						return
					}
					match = true
				}
			}
			if match {
				return
			}
		}
	}
}

// getProp returns the first property found using [mdProp.iterProps].
func (md *mdProp) getProp(names ...string) any {
	for res := range md.iterProps(names...) {
		return res
	}
	return nil
}
