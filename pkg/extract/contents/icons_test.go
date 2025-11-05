// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents

import (
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/go-shiori/dom"
	"github.com/stretchr/testify/require"
)

func TestIsIcon(t *testing.T) {
	tests := []struct {
		html     string
		size     [2]int
		expected bool
	}{
		{
			`<div><p><img src="img.png"></p></div>`,
			[2]int{32, 32},
			true,
		},
		{
			`<div><p><img src="img.png"></p></div>`,
			[2]int{32, 12},
			false,
		},
		{
			`<div><p><img src="img.png"></p></div>`,
			[2]int{120, 120},
			false,
		},
		{
			`<div><p><strong><span><img src="img.png">`,
			[2]int{48, 48},
			true,
		},
		{
			`<div><p><span><strong>test</strong> <img src="img.png">`,
			[2]int{48, 48},
			true,
		},
		{
			`<div><p><span><img src="img.png"> <strong>test</strong>`,
			[2]int{48, 48},
			true,
		},
		{
			`<div><p>test <img src="img.png"> test`,
			[2]int{48, 48},
			false,
		},
		{
			`<div><p><strong>test</strong><img src="img.png"> test`,
			[2]int{48, 48},
			false,
		},
		{
			`<div><p><strong>test</strong><img src="img.png"><em>test</em>`,
			[2]int{48, 48},
			false,
		},
		{
			`<div><p><strong>test</strong><a><img src="img.png"></a></p>`,
			[2]int{48, 48},
			true,
		},
		{
			`<div><p><strong>test</strong> <a><img src="img.png"></a> <strong>test</strong></p>`,
			[2]int{48, 48},
			false,
		},
		{
			`<div><p><strong>test</strong> <img src="img.png"> <em>test</em>`,
			[2]int{48, 48},
			false,
		},
		{
			`<div><p>test</p>  <img src="img.png"> <div>test</div>`,
			[2]int{48, 48},
			true,
		},
		{
			`<div><p>test</p>  <a><img src="img.png"></a> <div>test</div>`,
			[2]int{48, 48},
			true,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := require.New(t)

			doc, err := html.Parse(strings.NewReader(test.html))
			assert.NoError(err)

			node := dom.QuerySelector(doc, "img")
			ok := IsIcon(node, test.size[0], test.size[1], 64)
			assert.Equal(test.expected, ok)
		})
	}
}
