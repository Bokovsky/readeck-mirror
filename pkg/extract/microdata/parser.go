// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package microdata

import (
	"encoding/json"
	"iter"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type parser struct {
	root            *html.Node
	data            *Microdata
	baseURL         *url.URL
	raw             []map[string]any
	identifiedNodes map[string]*html.Node
}

func newParser(root *html.Node, baseURL string) (*parser, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	return &parser{
		root: root,
		data: &Microdata{
			Nodes: []*Node{},
		},
		baseURL:         u,
		raw:             []map[string]any{},
		identifiedNodes: map[string]*html.Node{},
	}, nil
}

func (p *parser) parse() (*Microdata, error) {
	topLevelNodes := []*html.Node{}
	jsonNodes := []*html.Node{}

	for n := range iterNodes(p.root) {
		// Collect JSON-LD nodes
		if n.FirstChild != nil && n.DataAtom == atom.Script {
			if a, _ := getAttr(n, "type"); a == "application/ld+json" {
				jsonNodes = append(jsonNodes, n.FirstChild)
			}
		}

		// Collect microdata nodes
		if hasAttr(n, "itemscope") && !hasAttr(n, "itemprop") {
			topLevelNodes = append(topLevelNodes, n)
		}

		if id, _ := getAttr(n, "id"); id != "" {
			p.identifiedNodes[id] = n
		}
	}

	for _, n := range topLevelNodes {
		item := map[string]any{}
		p.readSchemaAttr(item, n)
		p.readSchemaNode(item, n, true)
		if len(item) > 0 {
			p.raw = append(p.raw, item)
		}
	}

	for _, n := range jsonNodes {
		v, err := p.decodeJSONLD([]byte(n.Data))
		if err != nil {
			return nil, err
		}
		switch t := v.(type) {
		case []any:
			for _, x := range t {
				if x, ok := x.(map[string]any); ok {
					p.raw = append(p.raw, x)
				}
			}
		case map[string]any:
			p.raw = append(p.raw, t)
		}
	}

	md := &Microdata{
		Nodes: []*Node{},
	}

	// Collect nodes from the raw list
	for _, raw := range p.raw {
		node := &Node{}
		node.raw = raw
		node.load(raw)
		md.Nodes = append(md.Nodes, node)
	}

	return md, nil
}

func (p *parser) readSchemaNode(item map[string]any, n *html.Node, topLevel bool) {
	itemprops, hasProp := getAttr(n, "itemprop")
	_, hasScope := getAttr(n, "itemscope")

	switch {
	case hasScope && hasProp:
		sub := map[string]any{}
		p.readSchemaAttr(sub, n)
		for prop := range strings.FieldsSeq(itemprops) {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				p.readSchemaNode(sub, c, false)
			}

			if sub["@context"] == item["@context"] {
				delete(sub, "@context")
			}

			if len(prop) > 0 {
				addToItem(item, prop, sub)
			}
		}
		return
	case !hasScope && hasProp:
		if s := p.getSchemaValue(n); len(s) > 0 {
			for name := range strings.FieldsSeq(itemprops) {
				if len(name) > 0 {
					addToItem(item, name, s)
				}
			}
		}
		return
	case hasScope && !topLevel:
		return
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.readSchemaNode(item, c, false)
	}
}

func (p *parser) readSchemaAttr(item map[string]any, node *html.Node) {
	if s, ok := getAttr(node, "itemtype"); ok {
		for itemtype := range strings.FieldsSeq(s) {
			if len(itemtype) > 0 {
				addToItem(item, "@type", itemtype)
			}
		}
	}

	if s, ok := getAttr(node, "itemid"); ok {
		if strings.HasPrefix(s, "#") {
			item["@id"] = strings.TrimLeft(s, "#")
		} else if u, err := p.baseURL.Parse(s); err == nil {
			item["@id"] = u.String()
		}
	}

	if s, ok := getAttr(node, "itemref"); ok {
		for itemref := range strings.FieldsSeq(s) {
			if len(itemref) > 0 {
				if n, ok := p.identifiedNodes[itemref]; ok {
					p.readSchemaNode(item, n, false)
				}
			}
		}
	}

	// split type / context
	if t, ok := item["@type"]; ok {
		if t, ok := t.(string); ok {
			if u, err := url.Parse(t); err == nil {
				item["@context"] = u.Scheme + "://" + u.Host
				item["@type"] = strings.Trim(u.Path, "/")
			}
		}
	}
}

func (p *parser) getSchemaValue(node *html.Node) string {
	var propValue string

	switch node.DataAtom {
	case atom.Meta:
		if value, ok := getAttr(node, "content"); ok {
			propValue = value
		}
	case atom.Audio, atom.Embed, atom.Iframe, atom.Source, atom.Track, atom.Video:
		if value, ok := getAttr(node, "src"); ok {
			if u, err := p.baseURL.Parse(value); err == nil {
				propValue = u.String()
			}
		}
	case atom.Img:
		value, ok := getAttr(node, "data-sr")
		if !ok {
			value, ok = getAttr(node, "src")
		}

		if ok {
			if u, err := p.baseURL.Parse(value); err == nil {
				propValue = u.String()
			}
		}
	case atom.A, atom.Area, atom.Link:
		if value, ok := getAttr(node, "href"); ok {
			if u, err := p.baseURL.Parse(value); err == nil {
				propValue = u.String()
			}
		}
	case atom.Data, atom.Meter:
		if value, ok := getAttr(node, "value"); ok {
			propValue = value
		}
	case atom.Time:
		if value, ok := getAttr(node, "datetime"); ok {
			propValue = value
		}
	default:
		// The "content" attribute can be found on other tags besides the meta tag.
		if value, ok := getAttr(node, "content"); ok {
			propValue = value
			break
		}

		buf := new(strings.Builder)
		for n := range iterNodes(node) {
			if n.Type == html.TextNode {
				for s := range strings.FieldsSeq(n.Data) {
					buf.WriteString(s + " ")
				}
			}
		}
		propValue = buf.String()
	}

	return strings.TrimSpace(propValue)
}

func (p *parser) decodeJSONLD(data []byte) (any, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}

	return p.decodeJSONLDValues(v), nil
}

func (p *parser) decodeJSONLDValues(val any) any {
	switch t := val.(type) {
	case map[string]any:
		for k, v := range t {
			t[k] = p.decodeJSONLDValues(v)
		}
	case []any:
		for i, x := range t {
			t[i] = p.decodeJSONLDValues(x)
		}
	case string:
		return html.UnescapeString(t)
	}
	return val
}

func addToItem(item map[string]any, name string, val any) {
	if _, ok := item[name]; ok {
		switch t := item[name].(type) {
		case []any:
			t = append(t, val)
			item[name] = t
		default:
			item[name] = []any{item[name], val}
		}
		return
	}
	item[name] = val
}

func iterNodes(n *html.Node) iter.Seq[*html.Node] {
	return func(yield func(*html.Node) bool) {
		if n != nil {
			if !yield(n) {
				return
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				iterNodes(c)(yield)
			}
		}
	}
}

func getAttr(node *html.Node, name string) (string, bool) {
	for _, attr := range node.Attr {
		if name == attr.Key {
			return attr.Val, true
		}
	}
	return "", false
}

func hasAttr(node *html.Node, name string) bool {
	for _, attr := range node.Attr {
		if name == attr.Key {
			return true
		}
	}
	return false
}
