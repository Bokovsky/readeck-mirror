// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bleach_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/go-shiori/dom"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/pkg/bleach"
)

func TestClean(t *testing.T) {
	tests := []struct {
		fn       func(*html.Node)
		fragment string
		expected string
	}{
		{
			bleach.DefaultPolicy.Clean,
			`<p id="id-p" class="test" data-test="1" onClick="bar" rel="link" sizes="x1">foo</p>`,
			`<body><p id="id-p">foo</p></body>`,
		},
		{
			func(n *html.Node) {
				bleach.DefaultPolicy.Clean(n)
			},
			`<div><iframe src="http://example.net/" /></div>`,
			`<body><div></div></body>`,
		},
		{
			func(n *html.Node) {
				bleach.DefaultPolicy.Clean(n)
			},
			`<div><noscript>test</noscript></div>`,
			`<body><div><div>test</div></div></body>`,
		},
		{
			func(n *html.Node) {
				bleach.DefaultPolicy.Clean(n)
			},
			`<div><custom><script>alert("test")</script></custom></div>`,
			`<body><div><div></div></div></body>`,
		},
		{
			func(n *html.Node) {
				bleach.DefaultPolicy.Clean(n)
			},
			`<div><iframe src="http://example.net/" /><link><script></div>`,
			`<body><div></div></body>`,
		},
		{
			bleach.DefaultPolicy.RemoveEmptyNodes,
			`<p>test</p><span></span><br /><p>test 2</p>`,
			`<body><p>test</p><br/><p>test 2</p></body>`,
		},
		{
			bleach.DefaultPolicy.RemoveEmptyNodes,
			`<p>test</p><div><span></span></div><br /><p>test 2</p>`,
			`<body><p>test</p><br/><p>test 2</p></body>`,
		},
		{
			bleach.DefaultPolicy.RemoveEmptyNodes,
			`<p>test</p><div><span> </span></div><br /><p>test 😺</p>`,
			`<body><p>test</p><div><span> </span></div><br/><p>test 😺</p></body>`,
		},
		{
			bleach.DefaultPolicy.RemoveEmptyNodes,
			`<video controls><source src="foo"></video>`,
			`<body><video controls=""><source src="foo"/></video></body>`,
		},
		{
			bleach.DefaultPolicy.RemoveEmptyNodes,
			`<p><a name="foo"></a></p><p>test</p>`,
			`<body><p><a name="foo"></a></p><p>test</p></body>`,
		},
		{
			bleach.DefaultPolicy.RemoveEmptyNodes,
			`<pre> test </pre>`,
			`<body><pre> test </pre></body>`,
		},
		{
			bleach.DefaultPolicy.RemoveEmptyNodes,
			`<pre>
				test:
						<code>abc</code>
			</pre>`,
			`<body><pre>				test:
						<code>abc</code>
			</pre></body>`,
		},
		{
			bleach.DefaultPolicy.RemoveEmptyNodes,
			`<table><tr><td></td><td>test</td></tr>
			<tr><td>A</td><td>B</td></tr></table>`,
			`<body><table><tbody><tr><td></td><td>test</td></tr>
			<tr><td>A</td><td>B</td></tr></tbody></table></body>`,
		},
		{
			bleach.DefaultPolicy.RemoveEmptyNodes,
			`<math><mtable><mrow><mtd></mtd><mtd>2</mtd></mrow><mrow></mrow></mtable><mspace></mspace></math>`,
			`<body><math><mtable><mrow><mtd></mtd><mtd>2</mtd></mrow><mrow></mrow></mtable><mspace></mspace></math></body>`,
		},
		{
			bleach.DefaultPolicy.SetLinkRel,
			`<p><a href="foo">link</a></p>`,
			`<body><p><a href="foo" rel="nofollow noopener noreferrer">link</a></p></body>`,
		},
		{
			bleach.DefaultPolicy.SetLinkRel,
			`<p><a name="foo"></a></p>`,
			`<body><p><a name="foo"></a></p></body>`,
		},
		{
			func(n *html.Node) {
				bleach.DefaultPolicy.Clean(n)
				bleach.DefaultPolicy.RemoveEmptyNodes(n)
				bleach.DefaultPolicy.SetLinkRel(n)
			},
			`<p id="id-p" class="test" data-test="1" onClick="bar" rel="link" sizes="x1"><a name="foo"></a></p><p><a href="foo">link</a><span></span><hr></p>`,
			`<body><p id="id-p"><a name="foo"></a></p><p><a href="foo" rel="nofollow noopener noreferrer">link</a></p><hr/></body>`,
		},
		{
			func(n *html.Node) {
				bleach.DefaultPolicy.Clean(n)
				bleach.DefaultPolicy.RemoveEmptyNodes(n)
				bleach.DefaultPolicy.SetLinkRel(n)
			},
			`<div>
				<audio></audio>
				<applet></applet>
				<base></base>
				<button></button>
				<canvas></canvas>
				<dialog></dialog>
				<embed></embed>
				<fieldset></fieldset>
				<form></form>
				<frame></frame>
				<frameset></frameset>
				<iframe></iframe>
				<input></input>
				<link></link>
				<meta></meta>
				<param></param>
				<portal></portal>
				<script></script>
				<select></select>
				<slot></slot>
				<source></source>
				<style></style>
				<template></template>
				<textarea></textarea>
				<track></track>
				<video></video>
			</div>`,
			`<body></body>`,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			r := strings.NewReader(test.fragment)
			node, err := html.Parse(r)
			require.NoError(t, err)

			test.fn(node)
			require.Equal(t, test.expected, dom.OuterHTML(dom.QuerySelector(node, "body")))
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{string([]rune{0, 1, 31, 32, 33, 65}), " !A"},
		{string([]rune{147, 65, 12616}), "Aㅈ"},
		{string([]rune{145, 128571, 155}), "😻"},
		{"test\n\tabc", "test\n\tabc"},
		{"test\r\n\tabc", "test\r\n\tabc"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			require.Equal(t, test.expected, bleach.SanitizeString(test.input))
		})
	}
}
