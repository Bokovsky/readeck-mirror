// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents

import (
	"bytes"
	"log/slog"
	"net/url"

	"codeberg.org/readeck/readeck/pkg/extract"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// ExtractInlineSVGs is a processor that converts inline SVG to cached resources.
// Each SVG node is saved in the resource cache with a known URL, then the node is replaced
// by an img tag linking to this resource.
func ExtractInlineSVGs(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Dom == nil {
		return next
	}

	m.Log().Debug("extract inline SVG images")

	dom.ForEachNode(dom.QuerySelectorAll(m.Dom, "svg"), func(n *html.Node, _ int) {
		// Extract the node content to a buffer, as a standalone SVG file.
		buf := new(bytes.Buffer)
		buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		buf.WriteString("\n")

		// Ensure that the element is recognized as SVG by XML-compatible parsers
		if dom.GetAttribute(n, "xmlns") == "" {
			dom.SetAttribute(n, "xmlns", "http://www.w3.org/2000/svg")
		}

		err := html.Render(buf, n)
		if err != nil {
			m.Log().Error("", slog.Any("err", err))
			return
		}

		// Create an image with data URI
		src := new(bytes.Buffer)
		src.WriteString("data:" + "image/svg+xml,")
		src.WriteString(url.PathEscape(buf.String()))

		// Replace the SVG node by an image.
		imgNode := dom.CreateElement("img")
		dom.SetAttribute(imgNode, "src", src.String())

		dom.ReplaceChild(n.Parent, imgNode, n)
	})

	return next
}
