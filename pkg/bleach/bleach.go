// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package bleach is a simple HTML sanitizer tool.
package bleach

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/go-shiori/dom"
)

// Policy holds the cleaning rules and provides methods to
// perform the DOM cleaning.
type Policy struct {
	blockAttrs []*regexp.Regexp
	elementMap map[string]tagRule
}

// New creates a new cleaning policy.
func New(blockAttrs []*regexp.Regexp, elements map[string]tagRule) Policy {
	return Policy{
		blockAttrs: blockAttrs,
		elementMap: elements,
	}
}

// DefaultPolicy is the default bleach policy.
var DefaultPolicy = New(
	[]*regexp.Regexp{
		// Remove all class and style attributes
		regexp.MustCompile(`^(class|style)$`),
		// Remove all data-* attributes
		regexp.MustCompile(`^data-`),
		// Remove all on* (JS events) attributes
		regexp.MustCompile(`^on[a-z]+`),
		// Remove "rel" and "sizes" attributes
		regexp.MustCompile(`^(rel|sizes)$`),
	},
	elementMap,
)

// SanitizeString replaces any control character in a string by a space.
func SanitizeString(s string) string {
	return ctrlReplacer.Replace(s)
}

// Clean cleans removes unwanted tags and attributes from the document.
func (p Policy) Clean(top *html.Node) {
	p.cleanTags(top)
	p.cleanAttributes(top)
}

// cleanTags cleans up all the [html.Node] children.
// It applies, in one pass, a removal or renaming of elements.
func (p *Policy) cleanTags(top *html.Node) {
	dom.RemoveNodes(dom.QuerySelectorAll(top, "*"), func(node *html.Node) bool {
		if node.Type != html.ElementNode {
			return false
		}
		name := node.Data

		rule, exists := p.elementMap[name]
		if rule&tagRemove > 0 {
			// Remove tag, done
			return true
		}

		// Rename tag when it's unknown or has the [tagRename] flag.
		if !exists || rule&tagRename > 0 {
			if _, ok := blockTags[name]; ok || !exists {
				// a block or unknown tag becomes a div
				node.Data = "div"
				node.DataAtom = atom.Div
			} else {
				// otherwise, a span
				node.Data = "span"
				node.DataAtom = atom.Span
			}
		}

		return false
	})
}

// cleanAttributes discards unwanted attributes from all nodes.
func (p *Policy) cleanAttributes(top *html.Node) {
	for i := len(top.Attr) - 1; i >= 0; i-- {
		k := top.Attr[i].Key
		for _, r := range p.blockAttrs {
			if r.MatchString(k) {
				dom.RemoveAttribute(top, k)
				break
			}
		}
	}

	for child := dom.FirstElementChild(top); child != nil; child = dom.NextElementSibling(child) {
		p.Clean(child)
	}
}

// RemoveEmptyNodes removes the nodes that are empty.
// empty means: no child nodes, no attributes and no text content.
func (p Policy) RemoveEmptyNodes(top *html.Node) {
	dom.RemoveNodes(dom.QuerySelectorAll(top, "*"), func(node *html.Node) bool {
		if node.Type != html.ElementNode {
			return false
		}
		name := node.Data

		// Keep tags that are explicitly allowed to be empty, e.g. <hr>
		if p.elementMap[name]&tagKeepEmpty > 0 {
			return false
		}

		// Keep <a name> tags
		if name == "a" && dom.GetAttribute(node, "name") != "" {
			return false
		}

		// Keep nodes with children
		if len(dom.Children(node)) > 0 {
			return false
		}

		// Keep nodes with any text
		if _, ok := blockTags[dom.TagName(node)]; ok {
			// We can remove block tags with only spaces
			if strings.TrimFunc(dom.TextContent(node), isHTMLSpace) != "" {
				return false
			}
		} else if dom.TextContent(node) != "" {
			// Only remove inline tags when they contain nothing
			return false
		}

		// Remove node unless it's the document body
		return dom.TagName(node) != "body"
	})
}

// SetLinkRel adds a default "rel" attribute on all "a" tags.
func (p Policy) SetLinkRel(top *html.Node) {
	dom.ForEachNode(dom.QuerySelectorAll(top, "a[href]"), func(node *html.Node, _ int) {
		dom.SetAttribute(node, "rel", "nofollow noopener noreferrer")
	})
}

// isHTMLSpace returns true if a rune is a space as defined by the HTML spec.
func isHTMLSpace(r rune) bool {
	if uint32(r) <= unicode.MaxLatin1 {
		switch r {
		case '\t', '\n', '\r', ' ':
			return true
		}
		return false
	}
	return false
}
