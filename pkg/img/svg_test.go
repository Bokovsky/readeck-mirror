// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package img_test

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/img"
)

var rxSpaces = regexp.MustCompile(`>(\s+)<`)

func assertXMLEqual(t *testing.T, expected, actual string) {
	actual = rxSpaces.ReplaceAllString(actual, "><")
	expected = rxSpaces.ReplaceAllString(expected, "><")

	eNode, err := xmlquery.Parse(strings.NewReader(expected))
	if err != nil {
		t.Error(err)
		t.FailNow()
		return
	}
	aNode, err := xmlquery.Parse(strings.NewReader(actual))
	if err != nil {
		t.Error(err)
		t.FailNow()
		return
	}

	require.Equal(t, eNode.OutputXML(true), aNode.OutputXML(true))
}

func TestSvgImage(t *testing.T) {
	t.Run("load image", func(t *testing.T) {
		tests := []struct {
			svg string
			err string
		}{
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1">
				</svg>
				`,
				"",
			},
			{
				"<?xml",
				"XML syntax error on line 1: unexpected EOF",
			},
			{
				`<?xml version="1.0" standalone="yes"?>
				<!DOCTYPE test [ <!ENTITY xxe SYSTEM "file:///etc/hostname" > ]>
				<svg width="128px" height="128px" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1">
				  <text font-size="16" x="0" y="16">&xxe;</text>
				</svg>`,
				"XML syntax error on line 4: invalid character entity &xxe;",
			},
		}

		for i, test := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				assert := require.New(t)
				im, err := img.NewSvgImage(strings.NewReader(test.svg))
				if test.err != "" {
					assert.EqualError(err, test.err)
					return
				}

				assert.NoError(err)
				assert.Equal("svg", im.Format())
				assert.Equal("image/svg+xml", im.ContentType())
				assert.NoError(im.Close())
				assert.NoError(im.SetFormat("jpeg"))
				assert.NoError(im.SetCompression(img.CompressionBest))
			})
		}
	})

	t.Run("image size", func(t *testing.T) {
		tests := []struct {
			svg string
			w   int
			h   int
		}{
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1">
				</svg>
				`,
				300, 150,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 width="1280" height="720">
				</svg>
				`,
				1280, 720,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 width="1280px" height="720px">
				</svg>
				`,
				1280, 720,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 width="100%" height="50%">
				</svg>
				`,
				100, 50,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 width="100%" height="50">
				</svg>
				`,
				100, 50,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
					width="2.5ex" height="2em">
				</svg>
				`,
				20, 32,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
					width="2rem" height="1.5rex">
				</svg>
				`,
				32, 12,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 viewBox="0 0 200 300">
				</svg>
				`,
				200, 300,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 width="400" viewBox="0 0 200 300">
				</svg>
				`,
				400, 600,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 height="1200" viewBox="0 0 200 300">
				</svg>
				`,
				800, 1200,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 viewBox="0 0 200.512 300.123">
				</svg>
				`,
				200, 300,
			},
		}

		for i, test := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				assert := require.New(t)
				im, err := img.NewSvgImage(strings.NewReader(test.svg))
				assert.NoError(err)
				assert.Equal(test.w, int(im.Width()))
				assert.Equal(test.h, int(im.Height()))
			})
		}
	})

	t.Run("resize", func(t *testing.T) {
		tests := []struct {
			svg      string
			w        int
			h        int
			expected string
		}{
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1">
				</svg>`,
				500, 400,
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 width="500" height="400">
				</svg>`,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 width="1280" height="720">
				</svg>`,
				500, 400,
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 width="500" height="400">
				</svg>`,
			},
			{
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				 viewBox="0 0 100 200">
				</svg>`,
				500, 400,
				`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
				 <svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1"
				  viewBox="0 0 100 200" width="500" height="400">
				</svg>`,
			},
		}

		for i, test := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				assert := require.New(t)
				im, err := img.NewSvgImage(strings.NewReader(test.svg))
				assert.NoError(err)

				assert.NoError(im.Resize(uint(test.w), uint(test.h)))
				b := &bytes.Buffer{}

				ct, err := im.Encode(b)
				assert.NoError(err)
				assert.Equal("image/svg+xml", ct)
				assertXMLEqual(t, test.expected, b.String())
			})
		}
	})
}

func TestClean(t *testing.T) {
	tests := []struct {
		svg      string
		expected string
	}{
		{
			`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
			<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1">
			  <script type="text/ecmascript">
			    <![CDATA[
			      alert('Hax!');
			    ]]>
			  </script>
			</svg>`,
			`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
			<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" version="1.1">
			</svg>`,
		},
		{
			`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
			<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1">
			  <defs id="defs4">
			    <circle id="my_circle" cx="100" cy="50" r="40" fill="red"/>
			  </defs>
			  <g>
			    <use href="#my_circle" x="20" y="20"/>
				<use xlink:href="#my_circle" x="50" y="50"/>
				<use href="http://example.net/test.svg#abc" x="20" y="20"/>
				<use xlink:href="http://example.net/test.svg#abc" x="20" y="20"/>
			  </g>
			</svg>`,
			`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
			<svg xmlns:svg="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"  version="1.1">
			  <defs id="defs4">
			    <circle id="my_circle" cx="100" cy="50" r="40" fill="red"/>
			  </defs>
			  <g>
			    <use href="#my_circle" x="20" y="20"/>
				<use xlink:href="#my_circle" x="50" y="50"/>
			  </g>
			</svg>`,
		},
		{
			`
			<?xml version="1.0" encoding="UTF-8" standalone="no"?>
			<!-- Created with Inkscape (http://www.inkscape.org/) -->
			<svg xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:cc="http://creativecommons.org/ns#" xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns="http://www.w3.org/2000/svg" version="1.1"
			 width="512" height="512" id="svg2">
			<title>Mostly harmless</title>
			<metadata id="metadata7">Some metadata</metadata>

			<script type="text/ecmascript">
			<![CDATA[
			alert('Hax!');
			]]>
			</script>
			<style type="text/css">
			<![CDATA[ svg{display:none} ]]>
			</style>

			<defs id="defs4">
				<circle id="my_circle" cx="100" cy="50" r="40" fill="red"/>
			</defs>

			<g id="layer1">
			<a xlink:href="www.hax.ru">
				<use xlink:href="#my_circle" x="20" y="20"/>
				<use xlink:href="#my_circle" x="100" y="50"/>
				<use xlink:href="http://example.net/test.svg#abc"/>
				<use href="http://example.net/test.svg#abc"/>
			</a>
			</g>
			<text>
				<tspan>It was the best of times</tspan>
				<tspan dx="-140" dy="15">It was the worst of times.</tspan>
			</text>
			</svg>
			`,
			`
			<?xml version="1.0" encoding="UTF-8" standalone="no"?>
			<!-- Created with Inkscape (http://www.inkscape.org/) -->
			<svg xmlns:cc="http://creativecommons.org/ns#" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"  xmlns="http://www.w3.org/2000/svg" version="1.1"
			 width="512" height="512" id="svg2">
			<title>Mostly harmless</title>
			<metadata id="metadata7">Some metadata</metadata>

			<style type="text/css">
			<![CDATA[ svg{display:none} ]]>
			</style>

			<defs id="defs4">
				<circle id="my_circle" cx="100" cy="50" r="40" fill="red"/>
			</defs>

			<g id="layer1">
			<a xlink:href="www.hax.ru">
				<use xlink:href="#my_circle" x="20" y="20"/>
				<use xlink:href="#my_circle" x="100" y="50"/>
			</a>
			</g>
			<text>
				<tspan>It was the best of times</tspan>
				<tspan dx="-140" dy="15">It was the worst of times.</tspan>
			</text>
			</svg>
			`,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert := require.New(t)
			im, err := img.NewSvgImage(strings.NewReader(test.svg))
			assert.NoError(err)
			assert.NoError(im.Clean())

			b := &bytes.Buffer{}
			ct, err := im.Encode(b)
			assert.NoError(err)
			assert.Equal("image/svg+xml", ct)
			assertXMLEqual(t, test.expected, b.String())
		})
	}
}
