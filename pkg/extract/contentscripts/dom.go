// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/dop251/goja"
	"github.com/go-shiori/dom"
)

var (
	reflectTypeString    = reflect.TypeOf("")
	reflectTypeNodeProxy = reflect.TypeOf((*domNodeObj)(nil))
)

// domModule is the module to manipulate DOM nodes.
type domModule struct {
	r           *goja.Runtime
	constructor func(goja.FunctionCall) goja.Value
}

type domNodeObj struct {
	node *html.Node
}

// domNodeValue is the only value we expose to Goja.
// It wraps a [goja.Value] that must be a [domNodeObj].
// This double wrapping is the only way to provide custom
// equality targeting directly the embedded [html.Node].
type domNodeValue struct {
	goja.Value
}

func (nv domNodeValue) node() *html.Node {
	return nv.Value.Export().(*domNodeObj).node
}

// String implements [goja.Value]. It returns basic information
// about the wrapped [html.Node].
func (nv domNodeValue) String() string {
	n := nv.node()
	t := ""
	switch n.Type {
	case html.TextNode:
		t = "text"
	case html.DocumentNode:
		t = "document"
	case html.ElementNode:
		t = "element"
	case html.CommentNode:
		t = "comment"
	case html.DoctypeNode:
		t = "doctype"
	}

	return "<Node:" + t + " " + n.Data + ">"
}

// Equals implements [goja.Value] and test equality between
// [nodeObj.node] values.
func (nv domNodeValue) equals(v goja.Value) bool {
	if v, ok := v.Export().(*domNodeObj); ok {
		return v.node == nv.node()
	}
	return false
}

func (nv domNodeValue) SameAs(v goja.Value) bool {
	return nv.equals(v)
}

func (nv domNodeValue) Equals(v goja.Value) bool {
	return nv.equals(v)
}

func (nv domNodeValue) StrictEquals(v goja.Value) bool {
	return nv.equals(v)
}

// newDomModule returns a [domModule] with a constructor ready for use.
func newDomModule(vm *goja.Runtime) *domModule {
	m := &domModule{
		r: vm,
	}

	f := m.r.ToValue(func(call goja.ConstructorCall) *goja.Object {
		node, ok := call.Argument(0).Export().(*domNodeObj)
		if !ok {
			panic(m.r.ToValue("argument is not a node"))
		}

		res := m.r.ToValue(node).(*goja.Object)
		res.SetPrototype(call.This.Prototype())
		return res
	}).(*goja.Object)

	proto := m.createNodePrototype()
	_ = f.Set("prototype", proto)
	_ = proto.DefineDataProperty("constructor", f, goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_FALSE)

	m.constructor = f.Export().(func(goja.FunctionCall) goja.Value)
	return m
}

// newNodeValue returns a wrapped instance of [html.Node] to a
// [goja.Value] that supports comparison at the [html.Node] level.
func (m *domModule) newNodeValue(node *html.Node) goja.Value {
	if node == nil {
		return nil
	}

	// this is where the magic happens; we wrap our constructor
	// result in [domValue] so it become a [goja.Value] with
	// custom methods.
	return domNodeValue{
		m.constructor(goja.FunctionCall{Arguments: []goja.Value{m.r.ToValue(&domNodeObj{node})}}),
	}
}

func (m *domModule) valueToNode(v goja.Value) *domNodeObj {
	if n, ok := v.Export().(*domNodeObj); ok && n != nil {
		return n
	}

	panic(m.r.ToValue(`Value of "this" must be of type Node`))
}

func (m *domModule) defineNodeGetter(p *goja.Object, name string, getter func(*domNodeObj) any) {
	_ = p.DefineAccessorProperty(name,
		m.r.ToValue(func(call goja.FunctionCall) goja.Value {
			return m.r.ToValue(getter(m.valueToNode(call.This)))
		}),
		nil,
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	)
}

func (m *domModule) defineNodeSetter(p *goja.Object, name string, getter func(*domNodeObj) any, setter func(*domNodeObj, goja.Value)) {
	_ = p.DefineAccessorProperty(name,
		m.r.ToValue(func(call goja.FunctionCall) goja.Value {
			return m.r.ToValue(getter(m.valueToNode(call.This)))
		}),
		m.r.ToValue(func(call goja.FunctionCall) goja.Value {
			setter(m.valueToNode(call.This), call.Argument(0))
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	)
}

func (m *domModule) createNodePrototype() *goja.Object { //nolint:gocognit,gocyclo
	p := m.r.NewObject()

	// Constants
	_ = p.DefineDataProperty("TEXT_NODE", m.r.ToValue(html.TextNode), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
	_ = p.DefineDataProperty("DOCUMENT_NODE", m.r.ToValue(html.DocumentNode), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
	_ = p.DefineDataProperty("ELEMENT_NODE", m.r.ToValue(html.ElementNode), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
	_ = p.DefineDataProperty("COMMENT_NODE", m.r.ToValue(html.CommentNode), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
	_ = p.DefineDataProperty("DOCTYPE_NODE", m.r.ToValue(html.DoctypeNode), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)

	/*
	 * Attributes
	 * --------------------------------------------------------------
	 */

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/nodeName
	m.defineNodeGetter(p, "nodeName", func(o *domNodeObj) any {
		switch o.node.Type {
		case html.TextNode:
			return "#text"
		case html.DocumentNode:
			return "#document"
		case html.ElementNode:
			if o.node.Namespace == "" || o.node.Namespace == "html" {
				return strings.ToUpper(o.node.Data)
			}
			return o.node.Data
		case html.CommentNode:
			return "#comment"
		case html.DoctypeNode:
			return o.node.Data
		}

		return ""
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/nodeType
	m.defineNodeGetter(p, "nodeType", func(o *domNodeObj) any {
		return o.node.Type
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/nodeValue
	m.defineNodeGetter(p, "nodeValue", func(o *domNodeObj) any {
		switch o.node.Type {
		case html.TextNode, html.CommentNode:
			return o.node.Data
		}

		return nil
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/attributes
	m.defineNodeGetter(p, "attributes", func(o *domNodeObj) any {
		res := []goja.Value{}
		for _, attr := range o.node.Attr {
			res = append(res, m.r.ToValue(map[string]any{"name": attr.Key, "value": attr.Val}))
		}
		return res
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Document/body
	m.defineNodeGetter(p, "body", func(o *domNodeObj) any {
		if o.node.Type != html.DocumentNode {
			return goja.Undefined()
		}
		if n := dom.GetElementsByTagName(o.node, "body"); len(n) > 0 {
			return m.newNodeValue(n[0])
		}
		return goja.Undefined()
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/childNodes
	m.defineNodeGetter(p, "childNodes", func(o *domNodeObj) any {
		res := []goja.Value{}
		for c := range o.node.ChildNodes() {
			res = append(res, m.newNodeValue(c))
		}
		return res
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/children
	m.defineNodeGetter(p, "children", func(o *domNodeObj) any {
		res := []goja.Value{}
		for c := range o.node.ChildNodes() {
			if c.Type == html.ElementNode {
				res = append(res, m.newNodeValue(c))
			}
		}
		return res
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/firstChild
	m.defineNodeGetter(p, "firstChild", func(o *domNodeObj) any {
		if o.node.FirstChild != nil {
			return m.newNodeValue(o.node.FirstChild)
		}
		return nil
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/firstElementChild
	m.defineNodeGetter(p, "firstElementChild", func(o *domNodeObj) any {
		for c := range o.node.ChildNodes() {
			if c.Type == html.ElementNode {
				return m.newNodeValue(c)
			}
		}
		return nil
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/id
	m.defineNodeSetter(p, "id", func(o *domNodeObj) any {
		return dom.GetAttribute(o.node, "id")
	}, func(np *domNodeObj, v goja.Value) {
		if v.String() == "" {
			dom.RemoveAttribute(np.node, "id")
			return
		}
		dom.SetAttribute(np.node, "id", v.String())
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/innerHTML
	m.defineNodeSetter(p, "innerHTML", func(o *domNodeObj) any {
		return dom.InnerHTML(o.node)
	}, func(o *domNodeObj, v goja.Value) {
		if v.String() == "" {
			return
		}

		context := &html.Node{
			Type:      html.ElementNode,
			Data:      "body",
			DataAtom:  atom.Lookup([]byte("body")),
			Namespace: "",
		}

		nodes, err := html.ParseFragment(strings.NewReader(v.String()), context)
		if err != nil {
			panic(m.r.ToValue(fmt.Sprintf("can't parse HTML (%s)", err.Error())))
		}

		dom.RemoveNodes(dom.ChildNodes(o.node), nil)

		for _, n := range nodes {
			o.node.AppendChild(n)
		}
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/lastChild
	m.defineNodeGetter(p, "lastChild", func(np *domNodeObj) any {
		if np.node.LastChild != nil {
			return m.newNodeValue(np.node.LastChild)
		}
		return nil
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/nextSibling
	m.defineNodeGetter(p, "nextSibling", func(np *domNodeObj) any {
		if np.node.NextSibling != nil {
			return m.newNodeValue(np.node.NextSibling)
		}
		return nil
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/outerHTML
	m.defineNodeSetter(p, "outerHTML", func(np *domNodeObj) any {
		return dom.OuterHTML(np.node)
	}, func(o *domNodeObj, v goja.Value) {
		if v.String() == "" {
			return
		}

		if o.node.Parent == nil {
			panic(m.r.ToValue("can't set outerHTML of node without parent"))
		}

		context := &html.Node{
			Type:      html.ElementNode,
			Data:      "body",
			DataAtom:  atom.Lookup([]byte("body")),
			Namespace: "",
		}

		nodes, err := html.ParseFragment(strings.NewReader(v.String()), context)
		if err != nil {
			panic(m.r.ToValue(fmt.Sprintf("can't parse HTML (%s)", err.Error())))
		}

		for _, n := range nodes {
			o.node.Parent.InsertBefore(n, o.node)
		}
		o.node.Parent.RemoveChild(o.node)
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/parentNode
	m.defineNodeGetter(p, "parentNode", func(np *domNodeObj) any {
		if np.node.Parent != nil {
			return m.newNodeValue(np.node.Parent)
		}
		return nil
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/parentElement
	m.defineNodeGetter(p, "parentElement", func(np *domNodeObj) any {
		if np.node.Parent == nil {
			return nil
		}

		for p := range np.node.Ancestors() {
			if p.Type == html.ElementNode {
				return m.newNodeValue(p)
			}
		}
		return nil
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/previousSibling
	m.defineNodeGetter(p, "previousSibling", func(np *domNodeObj) any {
		if np.node.PrevSibling != nil {
			return m.newNodeValue(np.node.PrevSibling)
		}
		return nil
	})

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/textContent
	m.defineNodeGetter(p, "textContent", func(np *domNodeObj) any {
		return dom.TextContent(np.node)
	})

	/*
	 * Methods
	 * --------------------------------------------------------------
	 */

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/appendChild
	_ = p.Set("appendChild", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		arg, ok := call.Argument(0).Export().(*domNodeObj)
		if !ok {
			panic(m.r.ToValue("argument must be a node"))
		}
		np.node.AppendChild(arg.node)
		return goja.Undefined()
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/append
	_ = p.Set("append", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		for _, arg := range call.Arguments {
			var n *html.Node
			switch arg.ExportType() {
			case reflectTypeString:
				n = dom.CreateTextNode(arg.String())
			case reflectTypeNodeProxy:
				n = arg.Export().(*domNodeObj).node
			}

			if n == nil {
				continue
			}

			np.node.AppendChild(n)
		}
		return goja.Undefined()
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/cloneNode
	_ = p.Set("cloneNode", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		deep, _ := call.Argument(0).Export().(bool)
		return m.newNodeValue(dom.Clone(np.node, deep))
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/contains
	_ = p.Set("contains", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		arg, ok := call.Argument(0).Export().(*domNodeObj)
		if !ok {
			panic(m.r.ToValue("argument must be a node"))
		}

		for p := range arg.node.Ancestors() {
			if p == np.node {
				return m.r.ToValue(true)
			}
		}
		return m.r.ToValue(false)
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Document/createElement
	_ = p.Set("createElement", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		name, ok := call.Argument(0).Export().(string)
		if ok && name != "" {
			np := m.newNodeValue(dom.CreateElement(name))
			return np
		}
		return goja.Undefined()
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Document/createTextNode
	_ = p.Set("createTextNode", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		return m.r.ToValue(m.newNodeValue(dom.CreateTextNode(call.Argument(0).String())))
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/getAttribute
	_ = p.Set("getAttribute", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		arg, ok := call.Argument(0).Export().(string)
		if !ok {
			return goja.Null()
		}

		if dom.HasAttribute(np.node, arg) {
			return m.r.ToValue(dom.GetAttribute(np.node, arg))
		}
		return goja.Null()
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/hasAttribute
	_ = p.Set("hasAttribute", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		arg, ok := call.Argument(0).Export().(string)
		if !ok {
			return m.r.ToValue(false)
		}

		return m.r.ToValue(dom.HasAttribute(np.node, arg))
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/hasAttributes
	_ = p.Set("hasAttributes", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		return m.r.ToValue(len(np.node.Attr) > 0)
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/hasChildNodes
	_ = p.Set("hasChildNodes", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		return m.r.ToValue(np.node.FirstChild != nil)
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/insertBefore
	_ = p.Set("insertBefore", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		newChild, ok := call.Argument(0).Export().(*domNodeObj)
		if !ok {
			panic(m.r.ToValue("argument must be a node"))
		}
		oldChild, ok := call.Argument(1).Export().(*domNodeObj)
		if !ok {
			panic(m.r.ToValue("argument must be a node"))
		}
		np.node.InsertBefore(newChild.node, oldChild.node)
		return m.r.ToValue(newChild)
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/removeChild
	_ = p.Set("removeChild", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		arg, ok := call.Argument(0).Export().(*domNodeObj)
		if !ok {
			panic(m.r.ToValue("argument must be a node"))
		}
		if arg.node.Parent != np.node {
			panic(m.r.ToValue("removeChild called for a non-child Node"))
		}

		np.node.RemoveChild(arg.node)
		return m.r.ToValue(arg)
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/querySelector
	_ = p.Set("querySelector", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		arg, ok := call.Argument(0).Export().(string)
		if !ok {
			panic(m.r.ToValue("selector must be a string"))
		}

		return m.newNodeValue(dom.QuerySelector(np.node, arg))
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/querySelectorAll
	_ = p.Set("querySelectorAll", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		arg, ok := call.Argument(0).Export().(string)
		if !ok {
			panic(m.r.ToValue("selector must be a string"))
		}
		nodes := dom.QuerySelectorAll(np.node, arg)
		res := make([]goja.Value, len(nodes))
		for i, n := range nodes {
			res[i] = m.newNodeValue(n)
		}

		return m.r.ToValue(res)
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Node/replaceChild
	_ = p.Set("replaceChild", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		newChild, ok := call.Argument(0).Export().(*domNodeObj)
		if !ok {
			panic(m.r.ToValue("argument must be a node"))
		}
		oldChild, ok := call.Argument(1).Export().(*domNodeObj)
		if !ok {
			panic(m.r.ToValue("argument must be a node"))
		}

		dom.ReplaceChild(np.node, newChild.node, oldChild.node)
		return m.r.ToValue(oldChild)
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/replaceWith
	_ = p.Set("replaceWith", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		var nc *html.Node
		for i, arg := range call.Arguments {
			var n *html.Node
			switch arg.ExportType() {
			case reflectTypeString:
				n = dom.CreateTextNode(arg.String())
			case reflectTypeNodeProxy:
				n = arg.Export().(*domNodeObj).node
			}

			if n == nil {
				continue
			}

			if i == 0 {
				nc = n
				dom.ReplaceChild(np.node.Parent, n, np.node)
			} else {
				nc.Parent.InsertBefore(n, nc.NextSibling)
			}
		}

		return goja.Undefined()
	}))

	// https://developer.mozilla.org/en-US/docs/Web/API/Element/setAttribute
	_ = p.Set("setAttribute", m.r.ToValue(func(call goja.FunctionCall) goja.Value {
		np := m.valueToNode(call.This)
		name := call.Argument(0).String()
		value := call.Argument(1).String()

		if name != "" && value != "" {
			dom.SetAttribute(np.node, name, value)
		}

		return goja.Undefined()
	}))

	return p
}
