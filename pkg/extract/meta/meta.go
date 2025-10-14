// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package meta provides extract processors to retrieve several meta information
// from a page (meta tags, favicon, pictures...).
package meta

import (
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
	"github.com/araddon/dateparse"
	"github.com/go-shiori/dom"

	"codeberg.org/readeck/readeck/pkg/extract"
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

	// Set some values
	d.Title = d.Meta.LookupGet(
		"graph.title",
		"twitter.title",
		"html.title",
		"schema.name",
	)

	d.Description = d.Meta.LookupGet(
		"graph.description",
		"twitter.description",
		"html.description",
		"schema.description",
	)
	// Keep a short description (60 words)
	parts := strings.Split(d.Description, " ")
	if len(parts) > 60 {
		d.Description = strings.Join(parts[:60], " ") + "..."
	}

	d.AddAuthors(d.Meta.Lookup(
		"schema.author",
		"dc.creator",
		"html.author",
		"html.byl",
		"fediverse.creator",
	)...)

	if site := d.Meta.LookupGet(
		"graph.site_name",
		"schema.publisher",
	); site != "" {
		d.Site = site
	}

	if lang := d.Meta.LookupGet(
		"html.lang",
		"html.language",
	); len(lang) >= 2 {
		d.Lang = lang[0:2]
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

	// Set publication date
	d.Date, _ = dateparse.ParseLocal(d.Meta.LookupGet("html.date"))

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

type rawSpec struct {
	name     string
	selector string
	fn       func(*html.Node) (string, string)
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

var specList = []rawSpec{
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

	// Schema.org meta tags
	{
		"schema", "//meta[@content][@itemprop]",
		extMeta("itemprop", "content", ""),
	},

	// Schema.org author in content
	{
		"schema", "//*[contains(concat(' ',normalize-space(@itemprop),' '),' author ')]//*[contains(concat(' ',normalize-space(@itemprop),' '),' name ')]",
		func(n *html.Node) (string, string) {
			return "author", dom.TextContent(n)
		},
	},

	// Schema.org publisher in content
	{
		"schema", "//*[@itemscope][contains(concat(' ',normalize-space(@itemprop),' '),' publisher ')]//*[contains(concat(' ',normalize-space(@itemprop),' '),' name ')]",
		func(n *html.Node) (string, string) {
			return "publisher", dom.TextContent(n)
		},
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
