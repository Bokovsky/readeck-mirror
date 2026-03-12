// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	. "codeberg.org/readeck/readeck/pkg/extract/testing" //revive:disable:dot-imports
)

func TestDrop(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/404", httpmock.NewJsonResponderOrPanic(404, ""))
	httpmock.RegisterResponder("GET", "/error", httpmock.NewErrorResponder(errors.New("HTTP")))
	httpmock.RegisterResponder("GET", "/ioerror", NewIOErrorResponder(200,
		map[string]string{"content-type": "text/html; charset=UTF-8"}))
	httpmock.RegisterResponder("GET", "/ct-unsupported",
		NewContentResponder(200,
			map[string]string{"content-type": "application/x-something-weird"},
			"html/ex1.html"))
	httpmock.RegisterResponder("GET", "/ch-default", func(req *http.Request) (*http.Response, error) {
		var buf bytes.Buffer
		// To avoid this response being confidently detected as UTF-8 by HTMLReader, make sure that
		// there is at least 3 kB padding before the first Unicode character.
		for i := 0; i < 3; i++ {
			fmt.Fprintf(&buf, "<!-- % 1024s -->\n", "")
		}
		fmt.Fprint(&buf, "<p>Otters 🦦 are playful animals.</p>\n")
		return &http.Response{
			Request:    req,
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"text/html"}},
			Body:       io.NopCloser(&buf),
		}, nil
	})
	httpmock.RegisterResponder("GET", "/ch1",
		NewContentResponder(200,
			map[string]string{"content-type": "text/html; charset=UTF-8"},
			"html/ch1.html"))
	httpmock.RegisterResponder("GET", "/ch1-nocharset",
		NewContentResponder(200,
			map[string]string{"content-type": "text/html"},
			"html/ch1.html"))
	httpmock.RegisterResponder("GET", "/ch1-notype",
		NewContentResponder(200, nil, "html/ch1.html"))
	httpmock.RegisterResponder("GET", "/ch2",
		NewContentResponder(200,
			map[string]string{"content-type": "text/html; charset=ISO-8859-15"},
			"html/ch2.html"))
	httpmock.RegisterResponder("GET", "/ch3",
		NewContentResponder(200,
			map[string]string{"content-type": "text/html; charset=EUC-JP"},
			"html/ch3.html"))
	httpmock.RegisterResponder("GET", "/ch3-detect",
		NewContentResponder(200,
			map[string]string{"content-type": "text/html"},
			"html/ch3.html"))
	httpmock.RegisterResponder("GET", "/ch3-xhtml",
		NewContentResponder(200,
			map[string]string{"content-type": "application/xhtml+xml; charset=EUC-JP"},
			"html/ch3.xhtml"))
	httpmock.RegisterResponder("GET", "/ch3-xhtml-detect",
		NewContentResponder(200,
			map[string]string{"content-type": "application/xhtml+xml"},
			"html/ch3.xhtml"))
	httpmock.RegisterResponder("GET", "/ch3-xml-detect",
		NewContentResponder(200,
			map[string]string{"content-type": "application/xml"},
			"html/ch3.xhtml"))
	httpmock.RegisterResponder("GET", "/ch4-detect",
		NewContentResponder(200,
			map[string]string{"content-type": "text/html"},
			"html/ch4.html"))
	httpmock.RegisterResponder("GET", "/xml",
		httpmock.NewStringResponder(200, "<message>Hello</message>").
			HeaderAdd(http.Header{"Content-Type": []string{"application/xml"}}))

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name string
			url  *url.URL
			err  string
		}{
			{"no url", nil, "No document URL"},
			{"http", mustParse("http://x/error"), `Get "http://x/error": HTTP`},
			{"404", mustParse("http://x/404"), "Invalid status code (404)"},
			{"ioerror", mustParse("http://x/ioerror"), "HTMLReader error: read error"},
			{"no type", mustParse("http://x/ch1-notype"), "unsupported content-type: \"\""},
			{"unsupported", mustParse("http://x/ct-unsupported"), "unsupported content-type: \"application/x-something-weird\""},
			{"non-XHTML", mustParse("http://x/xml"), "HTMLReader error: html tag not found in application/xml document"},
		}

		for _, x := range tests {
			t.Run(x.name, func(t *testing.T) {
				d := extract.NewDrop(x.url)
				err := d.Load(nil)
				if err == nil {
					t.Fatal("error is nil")
				}

				require.Equal(t, x.err, err.Error())
			})
		}
	})

	t.Run("url", func(t *testing.T) {
		tests := []struct {
			src string
			res string
			dom string
		}{
			{
				"http://example.net/test/test",
				"http://example.net/test/test",
				"example.net",
			},
			{
				"http://example.net:8888/test/test",
				"http://example.net:8888/test/test",
				"example.net",
			},
			{
				"http://example.net:80/test/test",
				"http://example.net/test/test",
				"example.net",
			},
			{
				"http://example.net:80/test/test",
				"http://example.net/test/test",
				"example.net",
			},
			{
				"https://example.net:443/test/test",
				"https://example.net/test/test",
				"example.net",
			},
			{
				"http://belgië.icom.museum/€test",
				"http://belgië.icom.museum/€test",
				"icom.museum",
			},
			{
				"http://xn--wgv71a.icom.museum/%C2%A9",
				"http://日本.icom.museum/©",
				"icom.museum",
			},
			{
				"http://日本.icom.museum/",
				"http://日本.icom.museum/",
				"icom.museum",
			},
			{
				"http://example.co.jp",
				"http://example.co.jp",
				"example.co.jp",
			},
			{
				"http://127.0.0.1:5000",
				"http://127.0.0.1:5000",
				"127.0.0.1",
			},
			{
				"http://[fd66:2244:0::0:1]:5000",
				"http://[fd66:2244::1]:5000",
				"fd66:2244::1",
			},
			{
				"http://[::1]/",
				"http://[::1]/",
				"::1",
			},
			{
				"http://[::1]:80/",
				"http://[::1]/",
				"::1",
			},
			{
				"https://[fd66:8282::a]:443/",
				"https://[fd66:8282::a]/",
				"fd66:8282::a",
			},
		}

		for _, x := range tests {
			t.Run(x.src, func(t *testing.T) {
				d := extract.NewDrop(mustParse(x.src))
				require.Equal(t, x.res, d.UnescapedURL())
				require.Equal(t, x.dom, d.Domain)
			})
		}
	})

	t.Run("charset", func(t *testing.T) {
		tests := []struct {
			path        string
			isHTML      bool
			isMedia     bool
			contentType string
			charset     string
			contains    string
		}{
			{"ch1", true, false, "text/html", "utf-8", ""},
			{"ch1-nocharset", true, false, "text/html", "utf-8", ""},
			{"ch2", true, false, "text/html", "iso-8859-15", "grand mammifère"},
			{"ch-default", true, false, "text/html", "utf-8", "Otters 🦦 are playful"},
			{"ch3", true, false, "text/html", "euc-jp", "センチメートル"},
			{"ch3-detect", true, false, "text/html", "euc-jp", "センチメートル"},
			{"ch3-xhtml", true, false, "application/xhtml+xml", "euc-jp", "センチメートル"},
			{"ch3-xhtml-detect", true, false, "application/xhtml+xml", "euc-jp", "センチメートル"},
			{"ch3-xml-detect", true, false, "application/xml", "euc-jp", "センチメートル"},
			{"ch4-detect", true, false, "text/html", "utf-8", ""},
		}

		for _, x := range tests {
			t.Run(x.path, func(t *testing.T) {
				d := extract.NewDrop(mustParse("http://x/" + x.path))

				err := d.Load(nil)
				require.NoError(t, err)
				assert.Equal(t, "x", d.Site)
				assert.Equal(t, x.isHTML, d.IsHTML())
				assert.Equal(t, x.isMedia, d.IsMedia())
				assert.Equal(t, x.contentType, d.ContentType)
				assert.Equal(t, x.charset, d.Charset)

				if x.contains != "" {
					assert.Contains(t, string(d.Body), x.contains)
				}
			})
		}
	})
}

func TestDropAuthors(t *testing.T) {
	assert := require.New(t)
	uri, _ := url.Parse("/")
	d := extract.NewDrop(uri)

	assert.Equal([]string{}, d.Authors)

	d.AddAuthors("John Doe")
	assert.Equal([]string{"John Doe"}, d.Authors)

	d.AddAuthors("john Doe")
	assert.Equal([]string{"John Doe"}, d.Authors)

	d.AddAuthors("Someone Else")
	assert.Equal([]string{"John Doe", "Someone Else"}, d.Authors)

	d.Authors = []string{}
	d.AddAuthors("By   John   Doe")
	assert.Equal([]string{"John Doe"}, d.Authors)
	d.AddAuthors(" john doe   ")
	assert.Equal([]string{"John Doe"}, d.Authors)
	d.AddAuthors("By:   John   Doe")
	assert.Equal([]string{"John Doe"}, d.Authors)
	d.AddAuthors("by :  John   Doe")
	assert.Equal([]string{"John Doe"}, d.Authors)
}

func TestDropMeta(t *testing.T) {
	assert := require.New(t)
	m := extract.DropMeta{}
	m.Add("meta1", "foo")

	assert.Equal([]string{"foo"}, m.Lookup("meta1"))

	m.Add("meta1", "bar")
	assert.Equal([]string{"foo", "bar"}, m.Lookup("meta1"))
	assert.Equal("foo", m.LookupGet("meta1"))

	assert.Equal([]string{}, m.Lookup("meta2"))
	assert.Empty(m.LookupGet("meta2"))

	m.Add("meta2", "m2a")
	m.Add("meta2", "m2b")
	m.Add("meta3", "m3")

	assert.Equal([]string{"m2a", "m2b"}, m.Lookup("metaZ", "meta2", "meta1"))
	assert.Equal("m2a", m.LookupGet("metaZ", "meta2", "meta1"))
}
