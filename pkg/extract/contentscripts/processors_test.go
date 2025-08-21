// SPDX-FileCopyrightText: © 2025 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"context"
	"net/http"
	"strings"
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

func TestScriptVideoEmbed(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/page1", NewHTMLResponder(200, "video_embed.html"))
	httpmock.RegisterResponder("POST", "/youtubei/v1/player", func(_ *http.Request) (*http.Response, error) {
		rsp := httpmock.NewBytesResponse(200, []byte(`{
			"videoDetails": {"title": "Test Embed"}
		}`))
		rsp.Header.Set("Content-Type", "application/json")
		return rsp, nil
	})

	ex, _ := extract.New("http://example.net/page1")
	ex.AddProcessors(
		LoadScripts(),
		ProcessDom("documentReady"),
		ProcessDom("documentDone"),
	)
	ex.Run()
	assert.Empty(ex.Errors())

	expectHTML := `<!-- page 1 -->

    <p>This is a video:</p>
    <figure class="content"><img srcset="https://i.ytimg.com/vi/haAimDKxo40/maxresdefault.jpg, https://i.ytimg.com/vi/haAimDKxo40/hqdefault.jpg" alt="Youtube - Test Embed" data-iframe-params="src=https%3A%2F%2Fwww.youtube-nocookie.com%2Fembed%2FhaAimDKxo40%3Fsi%3DXZY%26start%3D123&amp;w=560&amp;h=315"/><figcaption><a href="https://www.youtube.com/watch?v=haAimDKxo40&amp;t=123s">Youtube - Test Embed</a></figcaption></figure>
    <p>Conclusion</p>`

	assert.Equal(expectHTML, strings.TrimSpace(string(ex.HTML)))
}
