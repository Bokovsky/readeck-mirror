// SPDX-FileCopyrightText: © 2026 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
)

func TestReadability(t *testing.T) {
	// normalizes the randomness added by setIDs
	normalizeIDsRE := regexp.MustCompile(` id="[a-zA-Z]{2}\.[a-zA-Z]{4}\.`)

	tests := []struct {
		name         string
		initialHTML  string
		expectedHTML string
		readability  bool
	}{
		{
			name:        "noscript with readability",
			readability: true,
			initialHTML: `<!DOCTYPE html>
<h1>Breaking news: Chickens are actually brave</h1>
<h2>New study on chickens suggests that you don't want to mess with a chicken</h2>
<noscript>
	<img src="path/to/chicken.jpeg 800w">
</noscript>
<picture><img alt="Fierce chicken" src="fallback.png"></picture>
<p>Whoever thought of calling cowardly people “chicken” has never actually met many chickens.</p>
<figure class="media">
  <img class="photo" src="photo.jpg">
  <figcaption>Photo by photographer</figcaption>
</figure>`,
			expectedHTML: `<!-- page 1 -->
<section id="{RANDOM}readability-page-1" class="page">
<h2>New study on chickens suggests that you don&#39;t want to mess with a chicken</h2>
<img src="http://example.net/page1/path/to/chicken.jpeg%20800w" data-old-src="http://example.net/page1/fallback.png" srcset="http://example.net/page1/fallback.png"/>

<p>Whoever thought of calling cowardly people “chicken” has never actually met many chickens.</p>
<figure>
  <img src="http://example.net/page1/photo.jpg"/>
  <figcaption>Photo by photographer</figcaption>
</figure></section>`,
		},
		{
			name:        "noscript no readability",
			readability: false,
			initialHTML: `<!DOCTYPE html>
<h1>Breaking news: Chickens are actually brave</h1>
<noscript>
	<img srcset="path/to/chicken.jpeg 800w">
</noscript>
<img alt="Fierce chicken" src="fallback.png">
<p>Whoever thought of calling cowardly people “chicken” has never actually met many chickens.</p>`,
			expectedHTML: `<!-- page 1 -->
<section><h1>Breaking news: Chickens are actually brave</h1>
<img alt="Fierce chicken" src="http://example.net/page1/fallback.png"/><noscript>
	<img srcset="path/to/chicken.jpeg 800w">
</noscript>

<p>Whoever thought of calling cowardly people “chicken” has never actually met many chickens.</p></section>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder("GET", "/page1/okay.html",
				httpmock.NewStringResponder(200, tt.initialHTML).
					HeaderAdd(http.Header{"Content-Type": {"text/html"}}))

			ex, err := extract.New("http://example.net/page1/okay.html")
			ex.Context = contents.EnableReadability(ex.Context, tt.readability)
			require.NoError(t, err)
			ex.AddProcessors(contents.Readability())
			ex.Run()
			require.Empty(t, ex.Errors())

			finalHTML := normalizeIDsRE.ReplaceAllLiteralString(string(ex.HTML), ` id="{RANDOM}`)
			finalHTML = strings.TrimSpace(finalHTML)

			if tt.expectedHTML != finalHTML {
				// Not using assert.Equal here because its failure message for comparing two large strings
				// is way too long before it even starts displaying the diff
				diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
					A:        difflib.SplitLines(tt.expectedHTML),
					B:        difflib.SplitLines(finalHTML),
					FromFile: "Expected",
					ToFile:   "Actual",
					Context:  2,
				})
				t.Error("Expected and actual HTML does not match:\n" + diff)
			}
		})
	}
}
