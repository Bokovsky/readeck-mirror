// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var (
	whitespaceOnly    = regexp.MustCompile("(?m)^[ \t]+$")
	leadingWhitespace = regexp.MustCompile("(?m)(^[ \t]*)(?:[^ \t\n])")
)

func dedent(text string) string {
	var margin string

	text = whitespaceOnly.ReplaceAllString(text, "")
	indents := leadingWhitespace.FindAllStringSubmatch(text, -1)

	for i, indent := range indents {
		switch {
		case i == 0:
			margin = indent[1]
		case strings.HasPrefix(indent[1], margin):
			continue
		case strings.HasPrefix(margin, indent[1]):
			margin = indent[1]
		default:
			margin = ""
			goto END_LOOP
		}
	}
END_LOOP:

	if margin != "" {
		text = regexp.MustCompile("(?m)^"+margin).ReplaceAllString(text, "")
	}
	return text
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		maps     []string
		expected string
	}{
		{
			[]string{
				`
				a: m1a
				b: m1b
				d:
					x: {a: m1dxa, b: m1dxb}
					z: 1234
				`,
				`
				a: m2a
				d:
					x: {a: m2dxa}
					y: m2dy
					z: m2dz
				c: [1, 2]
				`,
			},
			`
			a: m2a
			b: m1b
			d:
				x: {a: m2dxa, b: m1dxb}
				z: m2dz
				y: m2dy
			c: [1, 2]
			`,
		},
		{
			[]string{
				`
				a: m1a
				b: m1b
				`,
				`
				a: m2a
				`,
				`
				b: m3b
				`,
			},
			`
			a: m2a
			b: m3b
			`,
		},
		{
			[]string{
				`
				a: m1a
				b: [a, b, c]
				`,
				`
				a: m2a
				`,
				`
				b: [a, x, y]
				`,
			},
			`
			a: m2a
			b: [a, b, c, a, x, y]
			`,
		},
		{
			[]string{
				`
				responses:
					200:
						title: response A
						description: some text
				`,
				`
				responses:
					200:
						title: response B
				`,
				`
				responses:
					400:
						title: error
				`,
			},
			`
			responses:
				200:
					title: response B
					description: some text
				400:
					title: error
			`,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			mapList := []*yamlMap{}
			for _, m := range test.maps {
				m = dedent(m)
				m = strings.ReplaceAll(m, "\t", "    ")

				doc := new(yaml.Node)
				dec := yaml.NewDecoder(strings.NewReader(m))
				if err := dec.Decode(doc); err != nil {
					t.Fatal(err)
				}
				mapList = append(mapList, newMap(doc.Content[0]))
			}

			final := newMap(nil)
			for _, m := range mapList {
				final.update(m)
			}

			b := new(bytes.Buffer)
			enc := yaml.NewEncoder(b)
			if err := enc.Encode(&yaml.Node{Kind: yaml.MappingNode, Content: final.contentNodes()}); err != nil {
				t.Fatal(err)
			}

			expected := dedent(test.expected)
			expected = strings.ReplaceAll(expected, "\t", "    ")
			expected = strings.TrimSpace(expected)
			result := strings.TrimSpace(b.String())

			if expected != result {
				t.Logf("YAML result differs:\n> Expected:\n%s\n> Got:\n%s\n--\n", expected, result)
				t.Fail()
			}
		})
	}
}
