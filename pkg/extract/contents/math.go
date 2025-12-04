// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/net/html"

	"github.com/go-shiori/dom"
	"github.com/wyatt915/treeblood"

	"codeberg.org/readeck/readeck/pkg/extract"
)

// ConvertMathBlocks converts MathJax (2.7 and 3+) and katex-html to MathML.
func ConvertMathBlocks(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Dom == nil {
		return next
	}

	numReplaced := 0

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		for child := n.FirstChild; child != nil; {
			nextSibling := child.NextSibling
			if child.Type == html.ElementNode {
				switch child.Data {
				case "mjx-container":
					// MathJax v3+
					if mml, err := convertMathjaxBlock(child); err == nil {
						dom.ReplaceChild(child.Parent, mml, child)
						numReplaced++
					} else {
						m.Log().Error("convert mjx-container", slog.Any("err", err))
					}
				case "span":
					// Ensure that katex-html element is stripped even when Readability is turned
					// off, for example when saving a browser selection.
					var classNames string
					isHidden := false
					for _, attr := range child.Attr {
						switch attr.Key {
						case "class":
							classNames = attr.Val
						case "aria-hidden":
							isHidden = attr.Val == "true"
						}
					}
					// <span class="katex-html" aria-hidden="true">
					if isHidden && classNames == "katex-html" {
						child.Parent.RemoveChild(child)
					}
				case "nobr":
					// MathJax 2.7
					// <span class="MathJax"><nobr aria-hidden="true">
					if isHiddenElement(child) && isMathjaxSpan(child.Parent) {
						child.Parent.RemoveChild(child)
					}
				}
				if child.Parent != nil {
					traverse(child)
				}
			}
			child = nextSibling
		}
	}
	traverse(m.Dom)

	if numReplaced > 0 {
		m.Log().Debug("replaced mjx-container element with MathML", slog.Int("nodes", numReplaced))
	}

	return next
}

func isMathjaxSpan(el *html.Node) bool {
	if el.Data != "span" {
		return false
	}
	for _, attr := range el.Attr {
		if attr.Key == "class" && attr.Val == "MathJax" {
			return true
		}
	}
	return false
}

func convertMathjaxBlock(mjx *html.Node) (*html.Node, error) {
	isBlock := dom.GetAttribute(mjx, "display") == "true"
	for mathEl := range eachElement(mjx) {
		// The first element with the "data-latex" attribute will be either the "mjx-math"
		// element (for CHTML output) or a component of the "svg" element (for SVG output).
		tex := dom.GetAttribute(mathEl, "data-latex")
		if tex == "" {
			continue
		}
		mml, err := treeblood.TexToMML(tex, nil, isBlock, false)
		if err != nil {
			return nil, fmt.Errorf("error converting TeX expression to MathML: %w", err)
		}
		mmlNodes, err := html.ParseFragment(strings.NewReader(mml), nil)
		if err != nil {
			return nil, fmt.Errorf("error parsing MathML: %w", err)
		}
		if len(mmlNodes) == 0 {
			return nil, errors.New("no nodes found in MathML")
		}
		for mmlNode := range eachElementByTag(mmlNodes[0], "math") {
			return mmlNode, nil
		}
	}
	return nil, errors.New("not found")
}
