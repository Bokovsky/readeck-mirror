// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents

import (
	"iter"
	"strings"

	"golang.org/x/net/html"

	"github.com/go-shiori/dom"
)

var inlineTags = map[string]struct{}{
	"a":        {},
	"abbr":     {},
	"acronym":  {},
	"button":   {},
	"br":       {},
	"big":      {},
	"bdo":      {},
	"b":        {},
	"cite":     {},
	"code":     {},
	"dfn":      {},
	"i":        {},
	"em":       {},
	"img":      {},
	"input":    {},
	"kbd":      {},
	"label":    {},
	"map":      {},
	"object":   {},
	"output":   {},
	"tt":       {},
	"time":     {},
	"samp":     {},
	"script":   {},
	"select":   {},
	"small":    {},
	"span":     {},
	"strong":   {},
	"sub":      {},
	"sup":      {},
	"textarea": {},
}

// getSiblings is a recursive iterator that yields a node's siblings
// and then its parent's, etc.
func getSiblings(node *html.Node, f func(*html.Node) *html.Node) iter.Seq[*html.Node] {
	return func(yield func(*html.Node) bool) {
		n := f(node)
		for n != nil {
			if !yield(n) {
				return
			}
			n = f(n)
		}

		if node.Parent != nil {
			getSiblings(node.Parent, f)(yield)
		}
	}
}

// hasInlineSiblings iterates over a node's siblings and returns true when
// it's a text node with text or an inline element with text.
func hasInlineSiblings(node *html.Node, f func(*html.Node) *html.Node) bool {
	for n := range getSiblings(node, f) {
		if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
			return true
		}
		if n.Type == html.ElementNode {
			if _, ok := inlineTags[n.Data]; !ok {
				return false
			}
			if strings.TrimSpace(dom.TextContent(n)) != "" {
				return true
			}
		}
	}
	return false
}

// IsIcon returns true when an image has a ratio >= 0.9,
// its biggest dimension is greater or equal than "maxSize"
// and it doesn't have any text or inline sibling on both sides.
func IsIcon(node *html.Node, w, h, maxSize int) bool {
	if w <= 0 || h <= 0 {
		return false
	}

	if max(w, h) > maxSize || float64(min(w, h))/float64(max(w, h)) < 0.9 {
		return false
	}

	if hasInlineSiblings(node, func(n *html.Node) *html.Node { return n.PrevSibling }) &&
		hasInlineSiblings(node, func(n *html.Node) *html.Node { return n.NextSibling }) {
		return false
	}

	return true
}
