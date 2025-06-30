// SPDX-FileCopyrightText: © 2025 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents_test

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	. "codeberg.org/readeck/readeck/pkg/extract/testing" //revive:disable:dot-imports
)

func TestExtractor_StripHeadingAnchors(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/page1", NewHTMLResponder(200, "heading_anchors.html"))

	ex, _ := extract.New("http://example.net/page1")
	ex.AddProcessors(contents.StripHeadingAnchors)
	ex.Run()
	assert.Empty(ex.Errors())

	f, err := os.Open("test-fixtures/heading_anchors.expected.html")
	assert.NoError(err)
	defer f.Close() //nolint:errcheck
	expectedBytes, err := io.ReadAll(f)
	assert.NoError(err)

	finalHTML := strings.TrimSpace(string(ex.HTML))
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
