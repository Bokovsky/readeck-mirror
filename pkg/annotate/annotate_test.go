// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package annotate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
	"github.com/go-shiori/dom"
	"github.com/stretchr/testify/require"
)

func loadDocument() *html.Node {
	fd, err := os.Open("fixtures/test.html")
	if err != nil {
		panic(err)
	}

	doc, err := html.Parse(fd)
	fd.Close() //nolint:errcheck
	if err != nil {
		panic(err)
	}
	return doc
}

func TestFunctions(t *testing.T) {
	t.Run("getTextNodeBoundary", func(t *testing.T) {
		doc := loadDocument()
		root := dom.QuerySelector(doc, "body")
		tests := []struct {
			selector  string
			offset    int
			node      *html.Node
			newOffset int
			err       string
		}{
			{
				"../@", 0, nil, 0,
				"expression must evaluate to a node-set",
			},
			{
				"div[2]", 0, nil, 0,
				`element "div[2]" not found`,
			},
			{
				"p[1]", 260, nil, 0,
				`index "260" is out of range`,
			},
			{
				"p[1]", 0,
				htmlquery.FindOne(doc, "/html/body/p[1]/text()[1]"), 0,
				"",
			},
			{
				"p[1]", 5,
				htmlquery.FindOne(doc, "/html/body/p[1]/text()[1]"), 5,
				"",
			},
			{
				"p[2]", 30,
				htmlquery.FindOne(doc, "/html/body/p[2]/text()[2]"), 12,
				"",
			},
			{
				"div[1]/p[2]/strong[1]", 27,
				htmlquery.FindOne(doc, "/html/body/div[1]/p[2]/strong[1]/text()[2]"), 1,
				"",
			},
			{
				"p[3]", 179,
				htmlquery.FindOne(doc, "/html/body/p[3]/text()[2]"), 6,
				"",
			},
		}

		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				assert := require.New(t)
				node, offset, err := getTextNodeBoundary(boundaryStart, root, test.selector, test.offset)
				if test.err != "" {
					assert.EqualError(err, test.err)
				} else {
					assert.NoError(err)
					assert.Equal(html.TextNode, node.Type)
					assert.Equal(test.node, node)
					assert.Equal(test.newOffset, offset)
				}
			})
		}
	})

	t.Run("getSelector", func(t *testing.T) {
		doc := loadDocument()
		root := dom.QuerySelector(doc, "body")
		tests := []struct {
			node     *html.Node
			child    int
			selector string
			offset   int
			err      string
		}{
			{
				htmlquery.FindOne(doc, "/html/body/p[1]"), 0,
				"", 0,
				"node is not a text node",
			},
			{
				htmlquery.FindOne(doc, "/html/body/p[1]/text()[1]"), 0,
				"p[1]", 0,
				"",
			},
			{
				htmlquery.FindOne(doc, "/html/body/p[1]/text()[1]"), 5,
				"p[1]", 5,
				"",
			},
			{
				htmlquery.FindOne(doc, "/html/body/p[2]/text()[1]"), 7,
				"p[2]", 7,
				"",
			},
			{
				htmlquery.FindOne(doc, "/html/body/p[2]/text()[2]"), 12,
				"p[2]", 30,
				"",
			},
			{
				htmlquery.FindOne(doc, "/html/body/div[1]/p[2]/strong[1]/text()[2]"), 1,
				"div[1]/p[2]/strong[1]", 27,
				"",
			},
			{
				htmlquery.FindOne(doc, "/html/body/p[3]/text()[1]"), 100,
				"p[3]", 100,
				"",
			},
			{
				htmlquery.FindOne(doc, "/html/body/p[3]/text()[2]"), 9,
				"p[3]", 182,
				"",
			},
		}

		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				assert := require.New(t)
				selector, offset, err := getSelector(root, test.node, test.child)
				if test.err != "" {
					assert.EqualError(err, test.err)
				} else {
					assert.NoError(err)
					assert.Equal(test.selector, selector)
					assert.Equal(test.offset, offset)
				}
			})
		}
	})

	t.Run("wrapTextNode", func(t *testing.T) {
		contents := `<body>
		<p>Loren ipsum <strong>dolor</strong> sit amet</p>
		<p>“Med hjälp av en text som denna så ser man snabbt hur text kan placeras och ‘hur det därefter ser ut’”</p>
		</body>`

		tests := []struct {
			selector       string
			start          int
			end            int
			options        []func(*html.Node)
			resultSelector string
			result         string
		}{
			{
				"./p[1]/text()[1]", 0, 0, nil,
				"./p[1]",
				"<p><x-annotation></x-annotation>Loren ipsum <strong>dolor</strong> sit amet</p>",
			},
			{
				"./p[1]/text()[1]", 0, 12, nil,
				"./p[1]",
				"<p><x-annotation>Loren ipsum </x-annotation><strong>dolor</strong> sit amet</p>",
			},
			{
				"./p[1]/text()[1]", 6, 12, nil,
				"./p[1]",
				"<p>Loren <x-annotation>ipsum </x-annotation><strong>dolor</strong> sit amet</p>",
			},
			{
				"./p[1]/text()[1]", 0, 13, nil,
				"./p[1]",
				"<p>Loren ipsum <strong>dolor</strong> sit amet</p>",
			},
			{
				"./p[1]/text()[2]", 9, 10, nil,
				"./p[1]",
				"<p>Loren ipsum <strong>dolor</strong> sit amet</p>",
			},
			{
				"./p[1]/text()[1]", 1, 0, nil,
				"./p[1]",
				"<p>Loren ipsum <strong>dolor</strong> sit amet</p>",
			},
			{
				"./p[1]/text()[1]", -1, 13, nil,
				"./p[1]",
				"<p>Loren ipsum <strong>dolor</strong> sit amet</p>",
			},
			{
				"./p[1]/text()[1]", 0, -1, nil,
				"./p[1]",
				"<p>Loren ipsum <strong>dolor</strong> sit amet</p>",
			},
			{
				"./p[2]/text()[1]", 1, 4, nil,
				"./p[2]",
				"<p>“<x-annotation>Med</x-annotation> hjälp av en text som denna så ser man snabbt hur text kan placeras och ‘hur det därefter ser ut’”</p>",
			},
			{
				"./p[2]/text()[1]", 72, 102, nil,
				"./p[2]",
				"<p>“Med hjälp av en text som denna så ser man snabbt hur text kan placeras <x-annotation>och ‘hur det därefter ser ut’”</x-annotation></p>",
			},
			{
				"./p[1]", 0, 4, nil,
				"./p[1]",
				"<p>Loren ipsum <strong>dolor</strong> sit amet</p>",
			},
			{
				"./p[1]/text()[1]", 6, 12,
				[]func(*html.Node){
					func(n *html.Node) {
						n.Data = "span"
					},
					func(n *html.Node) {
						dom.SetAttribute(n, "class", "annotation")
					},
				},
				"./p[1]",
				`<p>Loren <span class="annotation">ipsum </span><strong>dolor</strong> sit amet</p>`,
			},
		}

		for _, test := range tests {
			t.Run(fmt.Sprintf("%d:%d", test.start, test.end), func(t *testing.T) {
				root, err := html.Parse(strings.NewReader(contents))
				if err != nil {
					panic(err)
				}
				n := htmlquery.FindOne(dom.QuerySelector(root, "body"), test.selector)
				wrapTextNode(n, test.start, test.end, test.options...)

				s := bytes.Buffer{}
				html.Render(&s, htmlquery.FindOne(dom.QuerySelector(root, "body"), test.resultSelector)) //nolint:errcheck
				require.Equal(t, test.result, s.String())
			})
		}
	})
}

func TestAnnotation(t *testing.T) {
	t.Run("annotation", func(t *testing.T) {
		doc := loadDocument()
		root := dom.QuerySelector(doc, "body")

		tests := []struct {
			startSelector string
			startOffset   int
			endSelector   string
			endOffset     int
			expected      *AnnotationRange
			err           string
		}{
			{
				"p[1]", 260,
				"p[1]", 270,
				nil,
				`index "260" is out of range`,
			},
			{
				"p[1]", 0,
				"p[1]", 270,
				nil,
				`index "270" is out of range`,
			},
			{
				"p[2]", 19,
				"p[1]", 24,
				nil,
				"no text nodes in range",
			},
			{
				"p[1]", 24,
				"p[1]", 19,
				nil,
				"invalid range",
			},
			{
				"p[1]", 19,
				"p[1]", 24,
				&AnnotationRange{
					root,
					nodeList{htmlquery.FindOne(root, "./p[1]/text()[1]")},
					htmlquery.FindOne(root, "./p[1]"),
					htmlquery.FindOne(root, "./p[1]/text()[1]"), 19,
					htmlquery.FindOne(root, "./p[1]/text()[1]"), 24,
				},
				"",
			},
			{
				"h2[1]/span[1]", 0,
				"p[2]/b[1]", 5,
				&AnnotationRange{
					root,
					nodeList{
						htmlquery.FindOne(root, "./h2[1]/span[1]/text()[1]"),
						htmlquery.FindOne(root, "./text()[3]"),
						htmlquery.FindOne(root, "./p[2]/text()[1]"),
						htmlquery.FindOne(root, "./p[2]/b[1]/text()[1]"),
					},
					root,
					htmlquery.FindOne(root, "./h2[1]/span[1]/text()[1]"), 0,
					htmlquery.FindOne(root, "./p[2]/b[1]/text()[1]"), 5,
				},
				"",
			},
			{
				"div[1]/p[2]", 66,
				"div[1]/p[2]", 122,
				&AnnotationRange{
					root,
					nodeList{
						htmlquery.FindOne(root, "./div[1]/p[2]/text()[1]"),
						htmlquery.FindOne(root, "./div[1]/p[2]/strong[1]/text()[1]"),
						htmlquery.FindOne(root, "./div[1]/p[2]/strong[1]/a[1]/text()[1]"),
						htmlquery.FindOne(root, "./div[1]/p[2]/strong[1]/text()[2]"),
						htmlquery.FindOne(root, "./div[1]/p[2]/text()[2]"),
					},
					htmlquery.FindOne(root, "./div[1]/p[2]"),
					htmlquery.FindOne(root, "./div[1]/p[2]/text()[1]"), 66,
					htmlquery.FindOne(root, "./div[1]/p[2]/text()[2]"), 13,
				},
				"",
			},
		}

		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				assert := require.New(t)
				a := NewAnnotation(root, test.startSelector, test.startOffset, test.endSelector, test.endOffset)
				r, err := a.ToRange()
				if test.err != "" {
					assert.EqualError(err, test.err)
				} else {
					assert.NoError(err)
					assert.Equal(test.expected, r)
				}
			})
		}
	})

	t.Run("range validator", func(t *testing.T) {
		doc := loadDocument()
		root := dom.QuerySelector(doc, "body")

		tests := []struct {
			startSelector string
			startOffset   int
			endSelector   string
			endOffset     int
			validators    []func(r *AnnotationRange) error
			err           string
		}{
			{
				"p[1]", 19,
				"p[1]", 24,
				[]func(r *AnnotationRange) error{
					func(_ *AnnotationRange) error {
						return nil
					},
				},
				"",
			},
			{
				"p[1]", 19,
				"p[1]", 24,
				[]func(r *AnnotationRange) error{
					func(_ *AnnotationRange) error {
						return nil
					},
					func(_ *AnnotationRange) error {
						return errors.New("invalid range")
					},
				},
				"invalid range",
			},
			{
				"p[1]", 19,
				"p[1]", 24,
				[]func(r *AnnotationRange) error{
					func(_ *AnnotationRange) error {
						return nil
					},
					func(_ *AnnotationRange) error {
						return errors.New("error 1")
					},
					func(_ *AnnotationRange) error {
						return errors.New("error 2")
					},
				},
				"error 1",
			},
		}

		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				assert := require.New(t)
				a := NewAnnotation(root, test.startSelector, test.startOffset, test.endSelector, test.endOffset)
				r, err := a.ToRange(test.validators...)
				if test.err != "" {
					assert.Nil(r)
					assert.EqualError(err, test.err)
				} else {
					assert.NoError(err)
				}
			})
		}
	})

	t.Run("range on null root", func(t *testing.T) {
		assert := require.New(t)
		a := NewAnnotation(nil, "", 0, "", 0)
		r, err := a.ToRange()
		assert.Nil(r)
		assert.EqualError(err, "root node is not defined")
	})

	t.Run("wrap", func(t *testing.T) {
		tests := []struct {
			annotations []*Annotation
			expected    [][2]string
		}{
			{
				[]*Annotation{
					NewAnnotation(nil, "p[1]", 19, "p[1]", 24),
				},
				[][2]string{
					{"./p[1]/x-annotation[1]", "dolor"},
				},
			},
			{
				[]*Annotation{
					NewAnnotation(nil, "h2[1]/span[1]", 0, "p[2]/b[1]", 5),
				},
				[][2]string{
					{"./h2[1]/span[1]/x-annotation[1]", "test"},
					{"./x-annotation[1]", ""},
					{"./p[2]/x-annotation[1]", "Lorem"},
					{"./p[2]/b[1]/x-annotation[1]", "ipsum"},
				},
			},
			{
				[]*Annotation{
					NewAnnotation(nil, "p[1]", 19, "p[1]", 24),
					NewAnnotation(nil, "h2[1]/span[1]", 0, "p[2]/b[1]", 5),
				},
				[][2]string{
					{"./p[1]/x-annotation[1]", "dolor"},
					{"./h2[1]/span[1]/x-annotation[1]", "test"},
					{"./x-annotation[1]", ""},
					{"./p[2]/x-annotation[1]", "Lorem"},
					{"./p[2]/b[1]/x-annotation[1]", "ipsum"},
				},
			},
			{
				[]*Annotation{
					NewAnnotation(nil, "h2[1]/span[1]", 0, "p[2]/b[1]", 5),
					NewAnnotation(nil, "p[1]", 19, "p[1]", 24),
				},
				[][2]string{
					{"./p[1]/x-annotation[1]", "dolor"},
					{"./h2[1]/span[1]/x-annotation[1]", "test"},
					{"./x-annotation[1]", ""},
					{"./p[2]/x-annotation[1]", "Lorem"},
					{"./p[2]/b[1]/x-annotation[1]", "ipsum"},
				},
			},
			{
				[]*Annotation{
					NewAnnotation(nil, "h2[1]/span[1]", 0, "p[2]/b[1]", 5),
					NewAnnotation(nil, "p[1]", 19, "p[1]", 24),
					NewAnnotation(nil, "div[1]/p[2]/strong[1]", 12, "div[1]/p[2]/strong[1]/a[1]", 3),
				},
				[][2]string{
					{"./p[1]/x-annotation[1]", "dolor"},
					{"./h2[1]/span[1]/x-annotation[1]", "test"},
					{"./x-annotation[1]", ""},
					{"./p[2]/x-annotation[1]", "Lorem"},
					{"./p[2]/b[1]/x-annotation[1]", "ipsum"},
					{"./div[1]/p[2]/strong[1]/x-annotation[1]", "sciunt"},
					{"./div[1]/p[2]/strong[1]/a[1]/x-annotation[1]", "sit"},
				},
			},
			{
				[]*Annotation{
					NewAnnotation(nil, "./p[3]", 174, "./p[3]", 204),
				},
				[][2]string{
					{"./p[3]/x-annotation[1]", "och ‘hur det därefter ser ut’”"},
				},
			},
			{
				[]*Annotation{
					NewAnnotation(nil, "./p[3]", 155, "./p[3]", 182),
				},
				[][2]string{
					{"./p[3]/x-annotation[1]", "kan"},
					{"./p[3]/b[1]/x-annotation[1]", "placeras"},
					{"./p[3]/x-annotation[2]", "och ‘hur"},
				},
			},
		}

		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				assert := require.New(t)
				doc := loadDocument()
				root := dom.QuerySelector(doc, "body")

				for _, annotation := range test.annotations {
					annotation.root = root
					r, err := annotation.ToRange()
					if err != nil {
						panic(err)
					}
					r.Wrap()
				}

				// s := bytes.Buffer{}
				// html.Render(&s, root)
				// println(s.String())

				nodes := htmlquery.Find(root, "//x-annotation")
				assert.Len(nodes, len(test.expected))

				for _, expected := range test.expected {
					n := htmlquery.FindOne(root, expected[0])
					assert.NotNil(n)
					assert.Equal(html.TextNode, n.FirstChild.Type)
					assert.Equal(expected[1], strings.TrimSpace(n.FirstChild.Data))
				}
			})
		}
	})
}

func TestAddAnnotation(t *testing.T) {
	tests := []struct {
		annotations []*Annotation
		expected    [][3]string
		err         string
	}{
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]", 19, "p[1]", 24),
			},
			[][3]string{
				{"./p[1]/my-annotation[1]", "dolor", "0"},
			},
			"",
		},
		{
			[]*Annotation{
				NewAnnotation(nil, "h2[1]/span[1]", 0, "p[2]/b[1]", 5),
				NewAnnotation(nil, "p[1]", 19, "p[1]", 24),
				NewAnnotation(nil, "div[1]/p[2]/strong[1]", 12, "div[1]/p[2]/strong[1]/a[1]", 3),
			},
			[][3]string{
				{"./p[1]/my-annotation[1]", "dolor", "1"},
				{"./h2[1]/span[1]/my-annotation[1]", "test", "0"},
				{"./my-annotation[1]", "", "0"},
				{"./p[2]/my-annotation[1]", "Lorem", "0"},
				{"./p[2]/b[1]/my-annotation[1]", "ipsum", "0"},
				{"./div[1]/p[2]/strong[1]/my-annotation[1]", "sciunt", "2"},
				{"./div[1]/p[2]/strong[1]/a[1]/my-annotation[1]", "sit", "2"},
			},
			"",
		},
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]", 1, "p[1]", 15),
				NewAnnotation(nil, "p[1]", 15, "p[1]", 23),
			},
			[][3]string{
				{"./p[1]/my-annotation[1]", "Lorem ip", "0"},
				{"./p[1]/my-annotation[2]", "sum dolo", "1"},
			},
			"",
		},
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]", 34, "p[1]", 62),
				NewAnnotation(nil, "p[1]", 87, "p[1]", 109),
			},
			[][3]string{
				{"./p[1]/my-annotation[1]", "consectetur adipisicing elit", "0"},
				{"./p[1]/my-annotation[2]", "inventore a voluptatem", "1"},
			},
			"",
		},
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]", 34, "p[1]", 62),
				NewAnnotation(nil, "p[1]", 87, "p[1]", 109),
				NewAnnotation(nil, "p[1]", 58, "p[1]", 71),
			},
			[][3]string{},
			"overlapping annotation",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := require.New(t)
			doc := loadDocument()
			root := dom.QuerySelector(doc, "body")

			var err error
			for j, a := range test.annotations {
				err = AddAnnotation(root, "my-annotation", a.startSelector, a.startOffset, a.endSelector, a.endOffset, func(n *html.Node, _, _ int) {
					dom.SetAttribute(n, "data-annotation-id", strconv.Itoa(j))
				})
				if err != nil {
					break
				}
			}

			if test.err != "" {
				assert.EqualError(err, test.err)
				return
			}
			assert.NoError(err)

			nodes := htmlquery.Find(root, "//my-annotation")
			assert.Len(nodes, len(test.expected))

			for _, expected := range test.expected {
				n := htmlquery.FindOne(root, expected[0])
				assert.NotNil(n)
				assert.Equal(html.TextNode, n.FirstChild.Type)
				assert.Equal(expected[1], strings.TrimSpace(n.FirstChild.Data))
				assert.Equal(expected[2], dom.GetAttribute(n, "data-annotation-id"))
			}
		})
	}
}

func TestContiguity(t *testing.T) {
	contents := "<p>12<span>3<b>45</b>67</span>89</p>"

	tests := []struct {
		annotations []*Annotation
		expect      string
	}{
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]", 0, "p[1]", 2),
			},
			"<body><p><abc>12</abc><span>3<b>45</b>67</span>89</p></body>",
		},
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]", 0, "p[1]", 9),
			},
			"<body><p><abc>12</abc><span><abc>3</abc><b><abc>45</abc></b><abc>67</abc></span><abc>89</abc></p></body>",
		},
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]", 2, "p[1]", 4),
			},
			"<body><p>12<span><abc>3</abc><b><abc>4</abc>5</b>67</span>89</p></body>",
		},
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]/span[1]/b[1]", 2, "p[1]", 9),
			},
			"<body><p>12<span>3<b>45</b><abc>67</abc></span><abc>89</abc></p></body>",
		},
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]/span[1]", 1, "p[1]/span[1]", 3),
			},
			"<body><p>12<span>3<b><abc>45</abc></b>67</span>89</p></body>",
		},
		{
			[]*Annotation{
				NewAnnotation(nil, "p[1]", 9, "p[1]", 9),
			},
			"<body><p>12<span>3<b>45</b>67</span>89</p></body>",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			doc, _ := html.Parse(strings.NewReader(contents))
			root := dom.QuerySelector(doc, "body")
			assert := require.New(t)
			for _, a := range test.annotations {
				err := AddAnnotation(root, "abc", a.startSelector, a.startOffset, a.endSelector, a.endOffset)
				assert.NoError(err)
			}

			w := new(bytes.Buffer)
			html.Render(w, root)
			assert.Equal(test.expect, w.String())
		})
	}
}
