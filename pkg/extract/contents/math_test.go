// SPDX-FileCopyrightText: © 2025 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents_test

import (
	"bytes"
	"io"
	"os"
	"sort"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	. "codeberg.org/readeck/readeck/pkg/extract/testing" //revive:disable:dot-imports
)

func TestExtractor_ConvertMathBlocks(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/page1/okay.html", NewHTMLResponder(200, "math.html"))

	ex, _ := extract.New("http://example.net/page1/okay.html")
	ex.AddProcessors(contents.ConvertMathBlocks)
	ex.Run()
	assert.Empty(ex.Errors())

	finalDoc, err := html.Parse(bytes.NewReader(ex.HTML))
	assert.NoError(err)
	normalize(finalDoc)

	var finalBuf bytes.Buffer
	if err := html.Render(&finalBuf, finalDoc); err != nil {
		t.Fatal(err)
	}
	finalHTML := finalBuf.String()

	f, err := os.Open("test-fixtures/math.expected.html")
	assert.NoError(err)
	defer f.Close() //nolint:errcheck
	expectedBytes, err := io.ReadAll(f)
	assert.NoError(err)

	if string(expectedBytes) != finalHTML {
		// Not using assert.Equal here because its failure message for comparing two large strings
		// is way too long before it even starts displaying the diff
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(expectedBytes)),
			B:        difflib.SplitLines(finalHTML),
			FromFile: "Expected",
			ToFile:   "Actual",
			Context:  2,
		})
		t.Error("Expected and actual HTML does not match:\n" + diff)
	}
}

func normalize(n *html.Node) {
	if n.Type == html.ElementNode {
		sort.Slice(n.Attr, func(i, j int) bool {
			return n.Attr[i].Key < n.Attr[j].Key
		})
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		normalize(c)
	}
}
