// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"encoding/json"
	"strings"

	"codeberg.org/readeck/readeck/pkg/extract"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// ParseProps parses raw properties from the page.
// The results for link and meta are a bit redundant with what we fetch
// with ParseMeta but they contain more information and can be useful
// to content scripts.
func ParseProps(doc *html.Node) extract.DropProperties {
	res := extract.DropProperties{}

	if plist := fetchProps(doc, "script[type='application/json']", jsonNode); plist != nil {
		res["json"] = plist
	}

	// Get meta
	if plist := fetchProps(doc, "head meta", nodeToMap); plist != nil {
		res["meta"] = plist
	}

	// Get links
	if plist := fetchProps(doc, "link", nodeToMap); plist != nil {
		res["link"] = plist
	}

	return res
}

func fetchProps(top *html.Node, selector string, fn func(*html.Node) (any, error)) []any {
	res := []any{}
	for _, node := range dom.QuerySelectorAll(top, selector) {
		if v, err := fn(node); err == nil {
			res = append(res, v)
		}
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

func jsonNode(node *html.Node) (any, error) {
	var res any
	err := json.Unmarshal([]byte(dom.TextContent(node)), &res)
	return res, err
}

func nodeToMap(node *html.Node) (any, error) {
	res := map[string]string{}
	for _, x := range node.Attr {
		res["@"+x.Key] = x.Val
	}
	if content := strings.TrimSpace(dom.TextContent(node)); content != "" {
		res["#text"] = content
	}

	return res, nil
}
