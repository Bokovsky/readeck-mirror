// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	_ "embed"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
)

func TestGetters(t *testing.T) {
	tests := []string{
		`doc.nodeName == "#document"`,
		`doc.nodeType == doc.DOCUMENT_NODE`,
		`doc.nodeValue == null`,
		`doc.attributes.length == 0`,
		`doc.body.firstChild.attributes.length == 1`,
		`doc.body !== null`,
		`doc.body.body === undefined`,
		`doc.body.nodeType == doc.ELEMENT_NODE`,
		`doc.childNodes.length == 1`,
		`doc.children.length == 1`,
		`doc.firstChild.nodeName == "HTML"`,
		`doc.body.firstElementChild.nodeName == "P"`,
		`doc.body.firstElementChild == doc.body.firstChild`,
		`doc.body.firstElementChild === doc.body.firstChild`,
		`doc.body.firstChild.firstChild.nodeType == doc.TEXT_NODE`,
		`doc.id == ""`,
		`doc.body.firstElementChild.id == "test-id"`,
		`doc.body.id = "body-id"; doc.body.id == "body-id"`,
		`doc.body.id = ""; doc.body.id == ""`,
		`doc.innerHTML == "<html><head></head><body><p id=\"test-id\">test</p><hr/></body></html>"`,
		`doc.body.firstChild.nextSibling.nodeName == "HR"`,
		`doc.body.firstChild.previousSibling == null`,
		`doc.body.lastChild.nextSibling == null`,
		`doc.body.lastChild.previousSibling.nodeName == "P"`,
		`doc.body.lastChild.firstChild == null`,
		`doc.body.lastChild.firstElementChild == null`,
		`doc.body.firstChild.parentNode.nodeName == "BODY"`,
		`doc.body.firstChild.parentElement.nodeName == "BODY"`,
		`doc.parentNode == null`,
		`doc.parentElement == null`,
		`doc.body.firstChild.innerHTML == "test"`,
		`doc.body.firstChild.outerHTML == "<p id=\"test-id\">test</p>"`,
		`doc.textContent == "test"`,
		`doc.createElement() === undefined`,
		`
			e = doc.createElement("img")
			doc.body.appendChild(e)
			doc.body.lastChild == e
		`,
		`
			e = doc.createElement("img")
			doc.body.append(e)
			doc.body.lastChild == e
		`,
		`
			e = doc.createElement("img")
			doc.body.append(e, "new text", 123)
			doc.body.lastChild.nodeValue == "new text"
		`,
		`
			e = doc.body.firstChild.cloneNode(true)
			e != doc.body.firstChild
		`,
		`doc.contains(doc.body.firstChild)`,
		`!doc.body.firstChild.contains(doc.body)`,
		`e = doc.createTextNode("abc"); e.nodeType == doc.TEXT_NODE`,
		`doc.body.firstChild.getAttribute(1) == null`,
		`doc.body.firstChild.getAttribute("id") == "test-id"`,
		`doc.body.firstChild.getAttribute("class") == null`,
		`!doc.body.firstChild.hasAttribute(1)`,
		`doc.body.firstChild.hasAttribute("id")`,
		`!doc.body.firstChild.hasAttribute("class")`,
		`!doc.body.hasAttributes()`,
		`doc.body.firstChild.hasAttributes()`,
		`doc.body.hasChildNodes()`,
		`!doc.body.lastChild.hasChildNodes()`,
		`
		node = doc.createElement("div")
		a = doc.createElement("a")
		span = doc.createElement("span")

		node.appendChild(a)
		node.insertBefore(span, a)
		node.outerHTML == "<div><span></span><a></a></div>"
		`,
		`
		res = false
		node = doc.createElement("div")
		try {
		  a = doc.createElement("a")
		  node.insertBefore(null, a)
		} catch (e) {
		  res = true
		}
		res
		`,
		`
		res = false
		node = doc.createElement("div")
		try {
		  a = doc.createElement("a")
		  node.insertBefore(a, null)
		} catch (e) {
		  res = true
		}
		res
		`,
		`
		doc.body.innerHTML = "<div>test</div>test"
		doc.body.outerHTML == "<body><div>test</div>test</body>"
		`,
		`
		doc.body.firstChild.outerHTML = "<div class=\"s\">woot</div>test"
		doc.body.outerHTML == "<body><div class=\"s\">woot</div>test<hr/></body>"
		`,
		`
		doc.body.removeChild(doc.body.firstChild)
		doc.body.innerHTML == "<hr/>"
		`,
		`
		res = false
		try {
		  doc.body.removeChild(0)
		} catch (e) {
		  res = true
		}
		res
		`,
		`
		res = false
		try {
		  doc.body.removeChild(doc)
		} catch (e) {
		  res = true
		}
		res
		`,
		`
		doc.body.querySelector("p") == doc.body.firstChild
		`,
		`
		res = false
		try {
		  doc.body.querySelector(123)
		} catch (e) {
		  res = true
		}
		res
		`,
		`
		doc.body.querySelectorAll("*").length == 2
		`,
		`
		res = false
		try {
		  doc.body.querySelectorAll(123)
		} catch (e) {
		  res = true
		}
		res
		`,
		`
		n = doc.createElement("span")
		n.setAttribute("data-test", "test")
		n.outerHTML == "<span data-test=\"test\"></span>"
		`,
		`
		n = doc.createElement("div")
		doc.body.replaceChild(n, doc.body.lastChild)
		doc.body.innerHTML == "<p id=\"test-id\">test</p><div></div>"
		`,
		`
		res = false
		try {
		  doc.body.replaceChild(0, doc.body.lastChild)
		} catch(e) {
		  res = true
		}
		res
		`,
		`
		res = false
		try {
		  doc.body.replaceChild(doc.body.lastChild, 0)
		} catch(e) {
		  res = true
		}
		res
		`,
		`
		n = doc.createElement("div")
		n.innerHTML = "test"
		doc.body.firstChild.replaceWith(n, "more test", 123)
		doc.body.innerHTML == "<div>test</div>more test<hr/>"
		`,
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			runtime := goja.New()
			m := newDomModule(runtime)

			printer := console.StdPrinter{
				StdoutPrint: func(s string) {
					t.Log(s)
				},
				StderrPrint: func(s string) {
					t.Log(s)
				},
			}

			registry := new(require.Registry)
			registry.Enable(runtime)
			registry.RegisterNativeModule(console.ModuleName, console.RequireWithPrinter(printer))
			console.Enable(runtime)

			node, err := html.Parse(strings.NewReader(`<p id="test-id">test</p><hr>`))
			if err != nil {
				t.Fatalf("errors is not nil (%s)", err.Error())
			}

			_ = runtime.Set("doc", m.newNodeValue(node))

			v, err := runtime.RunString(test)
			if err != nil {
				t.Fatalf("errors is not nil (%s)", err.Error())
			}

			if ok, _ := v.Export().(bool); !ok {
				t.Fatalf("result is false: %s :: %#v", test, v.Export())
			}
		})
	}
}
