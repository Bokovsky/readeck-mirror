// SPDX-FileCopyrightText: © 2026 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package tinymeta implements reading metadata from HTML documents without having to parse the
// entire DOM in memory.
package tinymeta

import (
	"bytes"
	"io"
	"iter"

	"golang.org/x/net/html"
)

// Scan returns an iterator that scans HTML tokens from r and yields key-value pairs representing
// common document metadata found in head, such as "title" or "og:description". Scanning will
// automatically stop as soon as the first body element is encountered.
func Scan(r io.Reader) iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		z := html.NewTokenizer(r)
		inTitle := false
		var titleText bytes.Buffer

		for {
			if z.Next() == html.ErrorToken {
				return
			}

			tt := z.Token()

			switch tt.Type {
			case html.TextToken:
				if inTitle {
					titleText.WriteString(tt.Data)
				}

			case html.EndTagToken:
				if tt.Data == "title" {
					inTitle = false
					if !yield("title", titleText.String()) {
						return
					}
				} else if inTitle {
					titleText.WriteString(tt.String())
				}

			case html.StartTagToken, html.SelfClosingTagToken:
				if inTitle {
					titleText.WriteString(tt.String())
					continue
				}

				switch tt.Data {
				case "title":
					inTitle = true
				case "meta":
					var metaName string
					var metaContent string
					for _, attr := range tt.Attr {
						switch attr.Key {
						case "name", "property":
							metaName = attr.Val
						case "content":
							metaContent = attr.Val
						}
					}
					if metaName != "" && metaContent != "" && !yield(metaName, metaContent) {
						return
					}
				case "html", "head", "base", "link", "style", "script", "noscript", "template":
					// Ignore other content in head while scanning for meta tags.
				default:
					// We've reached non-head content.
					return
				}
			}
		}
	}
}
