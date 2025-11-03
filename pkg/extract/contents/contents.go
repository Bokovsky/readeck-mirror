// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package contents provide extraction processes for content processing
// (readability) and plain text conversion.
package contents

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"

	"golang.org/x/net/html"

	"github.com/go-shiori/dom"

	"codeberg.org/readeck/go-readability/v2"
	readabilityRender "codeberg.org/readeck/go-readability/v2/render"
	"codeberg.org/readeck/readeck/pkg/extract"
)

type ctxKeyReadabilityEnabled struct{}

type readabilitySlogHandler struct {
	slog.Handler
}

func (h *readabilitySlogHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level == slog.LevelDebug {
		r.Level = slog.LevelDebug - 10
	}
	return h.Handler.Handle(ctx, r)
}

func (h *readabilitySlogHandler) Enabled(ctx context.Context, l slog.Level) bool {
	if l == slog.LevelDebug {
		return h.Handler.Enabled(ctx, slog.LevelDebug-10)
	}
	return h.Handler.Enabled(ctx, l)
}

func (h *readabilitySlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &readabilitySlogHandler{h.Handler.WithAttrs(attrs)}
}

func (h *readabilitySlogHandler) WithGroup(name string) slog.Handler {
	return &readabilitySlogHandler{h.Handler.WithGroup(name)}
}

// IsReadabilityEnabled returns true when readability is enabled
// in the extractor context.
func IsReadabilityEnabled(e *extract.Extractor) (enabled bool, forced bool) {
	if v, ok := e.Context.Value(ctxKeyReadabilityEnabled{}).(bool); ok {
		return v, true
	}
	// Default to true when the context value doest not exist
	return true, false
}

// EnableReadability enables or disable readability in the extractor
// context.
func EnableReadability(e *extract.Extractor, v bool) {
	e.Context = context.WithValue(e.Context, ctxKeyReadabilityEnabled{}, v)
}

// Readability is a processor that executes readability on the drop content.
func Readability(options ...func(*readability.Parser)) extract.Processor {
	return func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
		if m.Step() != extract.StepDom {
			return next
		}

		readabilityEnabled, readabilityForced := IsReadabilityEnabled(m.Extractor)

		// Immediate stop on a media where readability is not explicitly set.
		if m.Extractor.Drop().IsMedia() && !readabilityForced {
			m.ResetContent()
			return next
		}

		// Note: we perform some pre- and post-processing even when readability is disabled.
		fixNoscriptImages(m.Dom)
		convertPictureNodes(m.Dom, m)

		var doc *html.Node
		var body *html.Node

		if readabilityEnabled {
			prepareTitles(m.Dom)
			parser := readability.NewParser()
			parser.Logger = slog.New(&readabilitySlogHandler{m.Log().WithGroup("go-readability").Handler()})

			for _, f := range options {
				f(&parser)
			}

			article, err := parser.ParseAndMutate(m.Dom, m.Extractor.Drop().URL)
			if err != nil {
				m.Log().Error("readability error", slog.Any("err", err))
				m.ResetContent()
				return next
			}

			if article.Node == nil {
				m.Log().Error("could not extract content")
				m.ResetContent()
				return next
			}

			m.Log().Debug("readability on contents")

			doc = &html.Node{Type: html.DocumentNode}
			body = dom.CreateElement("body")
			doc.AppendChild(body)
			dom.AppendChild(body, article.Node)
		} else {
			m.Log().Info("readability is disabled by flag")
			doc = m.Dom
			body = dom.QuerySelector(doc, "body")
		}

		// final cleanup
		removeEmbeds(body)

		// Simplify the top hierarchy
		switch node := findFirstContentNode(body); {
		case node == body:
			break // just carry on
		case node != body.FirstChild:
			dom.ReplaceChild(body, node, body.FirstChild)
		}

		// Ensure we always start with a <section>
		encloseArticle(body)

		if readabilityEnabled {
			// Restore attributes stored by prepareTitles
			restoreDataAttributes(body)
		}

		// Replaces id attributes in content
		setIDs(body)

		m.Dom = doc

		return next
	}
}

// Text is a processor that sets the pure text content of the final HTML.
func Text(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepPostProcess {
		return next
	}

	if len(m.Extractor.HTML) == 0 {
		return next
	}
	if !m.Extractor.Drop().IsHTML() {
		return next
	}

	m.Log().Debug("get text content")

	doc, _ := html.Parse(bytes.NewReader(m.Extractor.HTML))
	m.Extractor.Text = readabilityRender.InnerText(doc)
	return next
}

// Traverses down the DOM to find the first element node that contains multiple elements or a
// non-whitespace text node. Only ever skip elements that are DIV, SECTION, MAIN, or ARTICLE.
func findFirstContentNode(parent *html.Node) *html.Node {
	var elementChild *html.Node
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode && strings.TrimSpace(child.Data) != "" {
			return parent
		} else if child.Type == html.ElementNode {
			if elementChild == nil {
				elementChild = child
			} else {
				// parent has multiple element children
				return parent
			}
		}
	}
	if elementChild == nil {
		return parent
	}
	switch elementChild.Data {
	case "div", "section", "main", "article":
		// Nothing should be lost semantically should we unwrap these container elements.
		return findFirstContentNode(elementChild)
	default:
		return elementChild
	}
}

// prepareTitles moves the "id" and "class" attributes from h* and h* a tags
// into their data--readeck counterparts. This is to avoid readability discarding
// valid titles with, for example id="examples".
func prepareTitles(top *html.Node) {
	dom.ForEachNode(
		dom.QuerySelectorAll(top, "h1, h2, h3, h4, h5, h6, h1 a, h2 a, h3 a, h4 a, h5 a, h6 a"),
		func(n *html.Node, _ int) {
			for _, x := range []string{"id", "class"} {
				if !dom.HasAttribute(n, x) {
					continue
				}
				dom.SetAttribute(n, "data--readeck-"+x, dom.GetAttribute(n, x))
				dom.RemoveAttribute(n, x)
			}
		},
	)
}

// restoreDataAttributes restore attributes previously stored as "data--readeck-*".
func restoreDataAttributes(top *html.Node) {
	dom.ForEachNode(dom.QuerySelectorAll(top, "[data--readeck-id]"), func(n *html.Node, _ int) {
		dom.SetAttribute(n, "id", dom.GetAttribute(n, "data--readeck-id"))
		dom.RemoveAttribute(n, "data--readeck-id")
	})
}

func setIDs(top *html.Node) {
	// Set a random prefix for the whole document
	chars := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	rand.Shuffle(len(chars), func(i, j int) {
		chars[i], chars[j] = chars[j], chars[i]
	})
	prefix := fmt.Sprintf("%s.%s", chars[0:2], chars[3:7])

	// Update all nodes with an id attribute
	for _, node := range dom.QuerySelectorAll(top, "[id]") {
		if value := dom.GetAttribute(node, "id"); value != "" {
			dom.SetAttribute(node, "id", fmt.Sprintf("%s.%s", prefix, value))
		}
	}

	// Update all a[name], because we'll update the href="#..." later
	for _, node := range dom.QuerySelectorAll(top, "a[name]") {
		if value := dom.GetAttribute(node, "name"); value != "" {
			dom.SetAttribute(node, "name", fmt.Sprintf("%s.%s", prefix, value))
		}
	}

	// Update all nodes with an href attribute starting with "#"
	for _, node := range dom.QuerySelectorAll(top, "[href^='#']") {
		if value := strings.TrimPrefix(dom.GetAttribute(node, "href"), "#"); value != "" {
			dom.SetAttribute(node, "href", fmt.Sprintf("#%s.%s", prefix, value))
		}
	}
}

func encloseArticle(top *html.Node) {
	children := dom.ChildNodes(top)

	if len(children) == 1 {
		node := children[0]
		switch node.Type {
		case html.TextNode:
			section := dom.CreateElement("section")
			dom.AppendChild(node.Parent, section)
			dom.AppendChild(section, node)
		case html.ElementNode:
			if node.Data == "div" {
				node.Data = "section"
			} else {
				section := dom.CreateElement("section")
				dom.AppendChild(node.Parent, section)
				dom.AppendChild(section, node)
			}
		}
	} else {
		section := dom.CreateElement("section")
		dom.AppendChild(top, section)
		for _, x := range children {
			dom.AppendChild(section, x)
		}
	}
}

func removeEmbeds(top *html.Node) {
	dom.RemoveNodes(dom.GetAllNodesWithTag(top, "object", "embed", "iframe", "video", "audio"), nil)
}

// fixNoscriptImages processes <noscript> tags by replacing them with the <img> tag contain within,
// but only when the <noscript> tag doesn't immediately follow another <img> tag. go-readability
// also does this, but since convertPictureNodes runs before readability, we need this explicit step
// to process <noscript> tags within <picture> elements.
func fixNoscriptImages(top *html.Node) {
	noscripts := dom.GetElementsByTagName(top, "noscript")
	dom.ForEachNode(noscripts, func(noscript *html.Node, _ int) {
		noscriptContent := dom.TextContent(noscript)
		tmpDoc, err := html.Parse(strings.NewReader(noscriptContent))
		if err != nil {
			return
		}

		tmpBody := dom.GetElementsByTagName(tmpDoc, "body")[0]
		if !isSingleImage(tmpBody) {
			return
		}

		// Sometimes, the image is *after* the noscript tag.
		// Let's move it before so the next step can detect it.
		nextElement := dom.NextElementSibling(noscript)
		if nextElement != nil && isSingleImage(nextElement) {
			if noscript.Parent != nil {
				noscript.Parent.InsertBefore(dom.Clone(nextElement, true), noscript)
				noscript.Parent.RemoveChild(nextElement)
			}
		}

		prevElement := dom.PreviousElementSibling(noscript)
		if prevElement == nil || !isSingleImage(prevElement) {
			dom.ReplaceChild(noscript.Parent, dom.FirstElementChild(tmpBody), noscript)
		}
	})
}

func isSingleImage(node *html.Node) bool {
	if dom.TagName(node) == "img" {
		return true
	}
	children := dom.Children(node)
	textContent := dom.TextContent(node)
	if len(children) != 1 || strings.TrimSpace(textContent) != "" {
		return false
	}

	return isSingleImage(children[0])
}

func convertPictureNodes(top *html.Node, _ *extract.ProcessMessage) {
	dom.ForEachNode(dom.GetElementsByTagName(top, "picture"), func(node *html.Node, _ int) {
		// A picture tag contains zero or more <source> elements
		// and an <img> element. We take all the srcset values from
		// each <source>, add them to the <img> srcset and then replace
		// the picture element with the img.

		// First get or create an img element
		imgs := dom.GetElementsByTagName(node, "img")
		var img *html.Node
		if len(imgs) == 0 {
			img = dom.CreateElement("img")
		} else {
			img = imgs[0]
		}

		// Collect all the srcset attributes
		set := []string{}
		sources := dom.GetElementsByTagName(node, "source")
		for _, n := range sources {
			if dom.HasAttribute(n, "srcset") {
				set = append(set, dom.GetAttribute(n, "srcset"))
			}
		}

		// Include the <img> attributes if present
		if dom.HasAttribute(img, "srcset") {
			set = append(set, dom.GetAttribute(img, "srcset"))
		}
		if dom.HasAttribute(img, "src") {
			set = append(set, dom.GetAttribute(img, "src"))
		}

		// Now mix them all together and replace the picture
		// element.
		dom.SetAttribute(img, "srcset", strings.Join(set, ", "))
		dom.ReplaceChild(node.Parent, img, node)
	})

	// We should keep images when they're in a figure tag.
	// Removing the classes and ids on the figure and its children avoids redability
	// discarding the whole thing.
	dom.ForEachNode(dom.GetElementsByTagName(top, "figure"), func(node *html.Node, _ int) {
		if len(dom.QuerySelectorAll(node, "img")) == 0 {
			return
		}

		dom.ForEachNode(dom.QuerySelectorAll(node, "*"), func(n *html.Node, _ int) {
			dom.SetAttribute(n, "class", "")
			dom.SetAttribute(n, "id", "")
		})
		dom.SetAttribute(node, "class", "")
		dom.SetAttribute(node, "id", "")
	})
}
