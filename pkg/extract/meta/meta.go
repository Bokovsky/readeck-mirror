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
	"slices"
	"strings"
	"time"

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

	// Extract article information from any article-like items encoded as JSON-LD.
	if jsonLD, ok := d.Properties["json-ld"].([]any); ok {
		for article := range eachSchemaDotOrgNode(jsonLD) {
			if !article.isArticle() {
				continue
			}
			if headline, ok := article.textProperty("headline"); ok {
				d.Title = headline
			}
			if description, ok := article.textProperty("description"); ok {
				d.Description = description
			}
			if imageURL, ok := article.textProperty("image", "url"); ok {
				d.Meta.Add("x.picture_url", imageURL)
			}
			if language, _ := article.textProperty("inLanguage", "alternateName"); len(language) >= 2 {
				d.Lang = language[0:2]
			}
			if datePublished, ok := article.textProperty("datePublished"); ok {
				if t, err := time.Parse(time.RFC3339, datePublished); err == nil && !t.IsZero() {
					d.Date = t
				}
			}
			for publisher := range article.nestedNodes("publisher") {
				if publisherName, _ := publisher.textProperty("name"); publisherName != "" {
					d.Site = publisherName
					break
				}
			}
			for author := range article.nestedNodes("author") {
				if authorName, _ := author.textProperty("name"); authorName != "" {
					d.AddAuthors(authorName)
				}
			}
			break
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

	// Schema.org microdata
	{
		"schema", "//*[@itemscope]//*[@itemprop]",
		extractSchemaDotOrgMicrodata,
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

// Extract the value of "itemprop" elements that are nested within "itemscope" elements.
//
// https://schema.org/docs/gs.html#microdata_itemscope_itemtype
func extractSchemaDotOrgMicrodata(n *html.Node) (string, string) {
	itemprop := strings.Fields(dom.GetAttribute(n, "itemprop"))
	if slices.Contains(itemprop, "headline") {
		// At this point we could check for `itemtype="http://schema.org/Article"` on the
		// parent `itemscope` element, but so many CreativeWork sub-types (such as
		// BlogPosting) also expose a headline that it would be too noisy to list them all
		// here. Instead, assume that the headline is that of the main article.
		//
		// https://schema.org/CreativeWork
		return "headline", microdataContent(n)
	} else if slices.Contains(itemprop, "name") {
		itemscopeNode := closestParentWithAttribute(n, "itemscope")
		if itemscopeNode == nil {
			return "", ""
		}
		articleNode := closestParentWithAttribute(itemscopeNode, "itemscope")
		if articleNode == nil {
			return "", ""
		}
		if !isArticleTypeURI(dom.GetAttribute(articleNode, "itemtype")) {
			return "", ""
		}
		scopeItemprop := strings.Fields(dom.GetAttribute(itemscopeNode, "itemprop"))
		if slices.Contains(scopeItemprop, "author") {
			// typically itemtype="http://schema.org/Person"
			return "author", microdataContent(n)
		} else if slices.Contains(scopeItemprop, "publisher") {
			// typically itemtype="http://schema.org/Organization"
			return "publisher", microdataContent(n)
		}
	}
	return "", ""
}

func closestParentWithAttribute(n *html.Node, attr string) *html.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if dom.HasAttribute(p, attr) {
			return p
		}
	}
	return nil
}

func microdataContent(n *html.Node) string {
	if n.Data == "meta" {
		return dom.GetAttribute(n, "content")
	}
	return dom.TextContent(n)
}

const schemaDotOrgContext = "https://schema.org"

// Returns whether a string is a schema.org URI.
func isSchemaDotOrg(s string) bool {
	s = strings.TrimSuffix(s, "/")
	return strings.Replace(s, "http:", "https:", 1) == schemaDotOrgContext
}

func isArticleTypeURI(s string) bool {
	idx := strings.LastIndexByte(s, '/')
	if idx < 0 {
		return false
	}
	if !isSchemaDotOrg(s[:idx]) {
		return false
	}
	_, found := schemaDotOrgArticleTypes[s[idx+1:]]
	return found
}

// List taken from https://schema.org/Article#subtypes
// TODO: consider allow-listing more types from https://schema.org/CreativeWork#subtypes
var schemaDotOrgArticleTypes = map[string]struct{}{
	"Article":                  {},
	"AdvertiserContentArticle": {},
	"NewsArticle":              {},
	"AnalysisNewsArticle":      {},
	"AskPublicNewsArticle":     {},
	"BackgroundNewsArticle":    {},
	"OpinionNewsArticle":       {},
	"ReportageNewsArticle":     {},
	"ReviewNewsArticle":        {},
	"Report":                   {},
	"SatiricalArticle":         {},
	"ScholarlyArticle":         {},
	"MedicalScholarlyArticle":  {},
	"SocialMediaPosting":       {},
	"BlogPosting":              {},
	"LiveBlogPosting":          {},
	"DiscussionForumPosting":   {},
	"TechArticle":              {},
	"APIReference":             {},
}

type dataNode map[string]any

func (n dataNode) isArticle() bool {
	nodeType, ok := n["@type"].(string)
	if !ok {
		return false
	}
	_, found := schemaDotOrgArticleTypes[nodeType]
	return found
}

func (n dataNode) nestedNodes(key string) iter.Seq[dataNode] {
	return func(yield func(dataNode) bool) {
		v, ok := n[key]
		if !ok {
			return
		}
		switch typedItem := v.(type) {
		case []any:
			for _, subitem := range typedItem {
				if i, ok := subitem.(map[string]any); ok && !yield(i) {
					return
				}
			}
		case map[string]any:
			if !yield(typedItem) {
				return
			}
		}
	}
}

func (n dataNode) textProperty(key string, subkeys ...string) (string, bool) {
	v, ok := n[key]
	if !ok {
		return "", false
	}
	switch typedItem := v.(type) {
	case string:
		// https://www.w3.org/TR/json-ld11/#restrictions-for-contents-of-json-ld-script-elements
		return html.UnescapeString(typedItem), true
	case map[string]any:
		if len(subkeys) < 1 {
			return "", false
		}
		return dataNode(typedItem).textProperty(subkeys[0], subkeys[:1]...)
	default:
		return "", false
	}
}

// Iterate through all JSON-LD nodes that are directly under `<script>` tags.
func topLevelNodes[V dataNode](items []any) iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, ldItem := range items {
			switch typedItem := ldItem.(type) {
			case []any:
				for _, subitem := range typedItem {
					if i, ok := subitem.(map[string]any); ok && !yield(i) {
						return
					}
				}
			case map[string]any:
				if !yield(typedItem) {
					return
				}
			}
		}
	}
}

// Iterate through all JSON-LD nodes, including ones that are part of the graph.
func eachSchemaDotOrgNode[V dataNode](items []any) iter.Seq[V] {
	return func(yield func(V) bool) {
		for node := range topLevelNodes(items) {
			if c, ok := node.textProperty("@context", "@vocab"); !ok || !isSchemaDotOrg(c) {
				continue
			}
			if nodes, ok := node["@graph"].([]any); ok {
				for _, nn := range nodes {
					if graphNode, ok := nn.(map[string]any); ok && !yield(V(graphNode)) {
						return
					}
				}
			} else if !yield(V(node)) {
				return
			}
		}
	}
}
