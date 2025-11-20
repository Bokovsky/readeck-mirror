// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package microdata provides a JSON-LD and HTML microdata parser and resolver.
// This is a very naive implementation, it only parses compact JSON-LD and does not
// care about context.
// Its purpose is to provide:
// - a microdata converter to JSON-LD
// - a JSON-LD cleanup pass (badly formatted JSON and/or HTML escaped)
// - a property lookup in the parsed data
package microdata

import (
	"golang.org/x/net/html"
)

// ParseNode parses an [html.Node] and returns a [Microdata] instance.
func ParseNode(root *html.Node, baseURL string) (*Microdata, error) {
	p, err := newParser(root, baseURL)
	if err != nil {
		return nil, err
	}

	return p.parse()
}
