// SPDX-FileCopyrightText: © 2025 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"context"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	. "codeberg.org/readeck/readeck/pkg/extract/testing" //revive:disable:dot-imports
)

func TestExtractBody(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/page1", NewHTMLResponder(200, "extract_body.html"))
	expectedHTML := `<!-- page 1 -->
<section class="article" id="article"><div class="article">
        <h1>Breaking news</h1>
        <p>This is absolutely a body</p>
    </div><div class="archive">
        <h1>Older news</h1>
    </div></section>
`

	ex, _ := extract.New("http://example.net/page1")
	ex.Context = context.WithValue(ex.Context, configCtxKey, &SiteConfig{
		BodySelectors: []string{
			"//div[@class = 'article'] | //div[@class = 'archive']",
			"//section",
		},
	})
	ex.AddProcessors(ExtractBody)
	ex.Run()
	assert.Empty(ex.Errors())
	assert.Equal(expectedHTML, string(ex.HTML))
}
