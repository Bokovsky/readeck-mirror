// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package annotate provides an annotation framework for HTML content.
package annotate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

const defaultTagName = "x-annotation"

type boundaryType int

const (
	boundaryStart boundaryType = iota
	boundaryEnd
)

type nodeList []*html.Node

// Annotation holds raw information about an annotation. It contains only selectors and offset used
// to find the relevant text nodes when looking up for a range.
type Annotation struct {
	root          *html.Node
	tagName       string
	startSelector string
	startOffset   int
	endSelector   string
	endOffset     int
}

// AnnotationRange holds the necessary DOM information to find and wrap an full annotation on
// a node.
type AnnotationRange struct {
	root           *html.Node
	textNodes      nodeList
	ancestor       *html.Node
	startContainer *html.Node
	startOffset    int
	endContainer   *html.Node
	endOffset      int
}

// WrapCallback is a function called on each annotation wrapping node.
// As an annotation can be covered by several wrapping nodes, an index gives the
// current wrapping position.
type WrapCallback func(node *html.Node, index, ln int)

type annotationError struct {
	msg string
}

func newError(s string, a ...any) *annotationError {
	return &annotationError{msg: fmt.Sprintf(s, a...)}
}

func (e *annotationError) Error() string {
	return e.msg
}

// ErrAnotate represents an error during the annotation creation.
var ErrAnotate *annotationError

// AddAnnotation is the main function that will add an annotation to a given root node.
// It lets one set an annotation tag name and performs the overlap validation.
// The root node is modified in place.
func AddAnnotation(
	root *html.Node, tagName string,
	startSelector string, startOffset int,
	endSelector string, endOffset int,
	options ...WrapCallback,
) error {
	a := NewAnnotation(root, startSelector, startOffset, endSelector, endOffset)
	r, err := a.ToRange(func(ar *AnnotationRange) error {
		for _, n := range ar.textNodes {
			if n.Parent.Data == tagName {
				return newError("overlapping annotation")
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	r.Wrap(append(options, func(n *html.Node, _, _ int) {
		// Allways set the tag name, regardless of previously applied modifiers
		n.Data = tagName
	})...)
	return nil
}

// NewAnnotation creates a new Annotation instance.
func NewAnnotation(root *html.Node, startSelector string, startOffset int, endSelector string, endOffset int) *Annotation {
	return &Annotation{
		root:          root,
		tagName:       defaultTagName,
		startSelector: startSelector,
		startOffset:   startOffset,
		endSelector:   endSelector,
		endOffset:     endOffset,
	}
}

// ToRange returns AnnotationRange instance from the current Annotation.
// This method finds the text nodes, common ancestor and validates that the
// range can be wraped later.
func (a *Annotation) ToRange(validators ...func(*AnnotationRange) error) (r *AnnotationRange, err error) {
	if a.root == nil {
		return nil, errors.New("root node is not defined")
	}

	r = &AnnotationRange{root: a.root}

	// Get range boundaries
	if r.startContainer, r.startOffset, err = getTextNodeBoundary(boundaryStart, a.root, a.startSelector, a.startOffset); err != nil {
		return nil, err
	}

	if r.endContainer, r.endOffset, err = getTextNodeBoundary(boundaryEnd, a.root, a.endSelector, a.endOffset); err != nil {
		return nil, err
	}

	// Find common ancestor node
	startParents := getParents(r.startContainer)
	endParents := getParents(r.endContainer)
	for i, node := range startParents {
		if i >= len(endParents) || endParents[i] != node {
			break
		}
		r.ancestor = node
	}

	// collect all text nodes included in range
	started := false
	done := false
	walkTextNodes(r.ancestor, func(n *html.Node) {
		if done {
			return
		}
		if n == r.startContainer {
			started = true
		}
		if started {
			r.textNodes = append(r.textNodes, n)
		}
		if n == r.endContainer {
			done = true
		}
	})

	// an empty text node list can be because the start element is after the end element
	// (it won't work)
	if len(r.textNodes) == 0 {
		return nil, newError("no text nodes in range")
	}

	// when only one node, check boundaries are ok
	if len(r.textNodes) == 1 && r.startOffset > r.endOffset {
		return nil, newError("invalid range")
	}

	for _, v := range validators {
		if err = v(r); err != nil {
			return nil, err
		}
	}

	return
}

// Wrap insert annotation elements around the range text nodes.
func (r *AnnotationRange) Wrap(options ...WrapCallback) {
	nbNodes := len(r.textNodes)
	for i, node := range r.textNodes {
		ln := len([]rune(node.Data))
		if i == 0 && r.startOffset == ln {
			// Discard empty start node
			continue
		}

		// Start offset
		s := 0
		if i == 0 {
			s = r.startOffset
		}

		// End offset
		e := ln
		if i+1 == len(r.textNodes) {
			e = r.endOffset
		}

		// Both start and end offset can be on the same node, it
		// does not matter.
		wrapTextNode(node, s, e, func(n *html.Node) {
			for _, f := range options {
				f(n, i, nbNodes)
			}
		})
	}
}

// getTextNodeBoundary returns a range (text node and offset), given a specific selector and an offset.
// The offset parameter is from the very beginning of the selector.
func getTextNodeBoundary(bt boundaryType, root *html.Node, selector string, index int) (*html.Node, int, error) {
	e, err := htmlquery.Query(root, "./"+selector)
	if err != nil {
		return nil, 0, err
	}

	if e == nil {
		return nil, 0, newError(`element "%s" not found`, selector)
	}
	var textNode *html.Node
	offset := 0
	consummed := 0

	nodes := getAllTextNodes(e)
	for i, n := range nodes {
		if textNode != nil {
			break
		}
		runesLength := len([]rune(n.Data))
		if index <= consummed+runesLength {
			offset = index - consummed
			textNode = n
		}
		consummed += runesLength

		if index == consummed {
			// When we reach the end of a node, we can actually move the cursor to the
			// next one.
			if bt == boundaryStart && i+1 < len(nodes) {
				textNode = nil
				offset = 0
			}
		}
	}

	if textNode == nil {
		return nil, 0, newError(`index "%d" is out of range`, index)
	}

	return textNode, offset, nil
}

// getSelector returns a CSS selector for a text node at the given offset.
func getSelector(root *html.Node, node *html.Node, offset int) (string, int, error) {
	if node.Type != html.TextNode {
		return "", 0, newError("node is not a text node")
	}

	p := node.Parent
	names := []string{}

	// Get selector
	for p.Parent != nil && p != root {
		i := 1
		s := p
		for dom.PreviousElementSibling(s) != nil {
			s = dom.PreviousElementSibling(s)
			if dom.TagName(s) == dom.TagName(p) {
				i++
			}
		}
		names = append(names, "")
		copy(names[1:], names)
		names[0] = fmt.Sprintf("%s[%d]", dom.TagName(p), i)
		p = p.Parent
	}

	// Get offset
	done := false
	newOffset := 0
	walkTextNodes(node.Parent, func(n *html.Node) {
		if done {
			return
		}
		if n == node {
			done = true
		}
		if !done {
			newOffset += len([]rune(n.Data))
		} else {
			newOffset += offset
		}
	})

	return strings.Join(names, "/"), newOffset, nil
}

// getParents returns the parents of a node, from top to bottom.
func getParents(node *html.Node) nodeList {
	p := node.Parent
	res := nodeList{}
	for p != nil {
		res = append(res, nil)
		copy(res[1:], res)
		res[0] = p
		p = p.Parent
	}
	return res
}

// walkTextNodes calls a callback function on each text node
// found in a given node and its descendants.
func walkTextNodes(node *html.Node, callback func(*html.Node)) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			callback(child)
		}
		walkTextNodes(child, callback)
	}
}

// getAllTextNodes returns a list of all text nodes found in the given
// node, recursively.
func getAllTextNodes(node *html.Node) []*html.Node {
	res := []*html.Node{}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			res = append(res, child)
		case html.ElementNode:
			res = append(res, getAllTextNodes(child)...)
		}
	}

	return res
}

// wrapTextNode replace the given text node with a wrap text node
// and surrounding text nodes when the offsets have content.
// It can take any func(*html.Node) to modify the resulting wrapping node
// (change the tag name, add attributes...)
func wrapTextNode(node *html.Node, startOffset, endOffset int, options ...func(*html.Node)) {
	if node == nil || node.Parent == nil || node.Type != html.TextNode {
		return
	}
	runes := []rune(node.Data)

	if startOffset > endOffset ||
		startOffset < 0 ||
		endOffset < 0 ||
		startOffset > len(runes) ||
		endOffset > len(runes) {
		return
	}

	var ts *html.Node
	var te *html.Node
	res := dom.CreateElement(defaultTagName)

	if startOffset > 0 && startOffset <= len(runes) {
		ts = dom.CreateTextNode(string(runes[0:startOffset]))
	}
	if endOffset >= 0 && endOffset <= len(runes) {
		te = dom.CreateTextNode(string(runes[endOffset:]))
	}

	tx := dom.CreateTextNode(string(runes[startOffset:endOffset]))
	dom.AppendChild(res, tx)
	dom.ReplaceChild(node.Parent, res, node)

	if ts != nil {
		res.Parent.InsertBefore(ts, res)
	}
	if te != nil {
		res.Parent.InsertBefore(te, res.NextSibling)
	}

	for _, option := range options {
		option(res)
	}
}
