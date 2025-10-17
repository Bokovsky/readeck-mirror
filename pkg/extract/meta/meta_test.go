// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
	"github.com/go-shiori/dom"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
)

func getFileContents(name string) []byte {
	fd, err := os.Open(path.Join("../test-fixtures", name))
	if err != nil {
		panic(err)
	}
	defer fd.Close() //nolint:errcheck

	data, err := io.ReadAll(fd)
	if err != nil {
		panic(err)
	}

	return data
}

func newFileResponder(name string) httpmock.Responder {
	return httpmock.NewBytesResponder(200, getFileContents(name))
}

func TestMeta(t *testing.T) {
	t.Run("ExtractMeta", func(t *testing.T) {
		t.Run("bad step", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepBody)
			ExtractMeta(pm, nil)
			assert.Equal(extract.DropMeta{}, ex.Drop().Meta)
		})

		t.Run("process meta1", func(t *testing.T) {
			assert := require.New(t)
			ex, _ := extract.New("http://example.net/", nil)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, _ = html.Parse(bytes.NewReader(getFileContents("meta/meta1.html")))

			ExtractMeta(pm, nil)

			assert.Equal("My Document Title", ex.Drop().Title)
			assert.Equal("Some description here", ex.Drop().Description)
			assert.Equal([]string{"author 3", "author 4"}, ex.Drop().Authors)
			assert.Equal("My website", ex.Drop().Site)
			assert.Equal("en", ex.Drop().Lang)

			assert.Equal(extract.DropMeta{
				"dc.creator":          {"author 3", "author 4"},
				"graph.image":         {"/squirrel.jpg"},
				"graph.site_name":     {"My website"},
				"html.author":         {"author 1", "author 2"},
				"html.byl":            {"author 5"},
				"html.copyright":      {"Part€"},
				"html.date":           {"sep 1 2020 11:12:34"},
				"html.description":    {"Some meta description", "Some more description"},
				"html.keywords":       {"a reporter at large, biology,space exploration,magazine"},
				"html.lang":           {"en"},
				"html.title":          {"My Document"},
				"twitter.card":        {"summary"},
				"twitter.description": {"Some description here"},
				"twitter.image":       {"/squirrel.jpg"},
				"twitter.title":       {"My Document Title"},
				"twitter.url":         {"http://localhost:8000/"},
			}, ex.Drop().Meta)
		})

		t.Run("process meta2", func(t *testing.T) {
			ex, _ := extract.New("http://example.net/", nil)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, _ = html.Parse(bytes.NewReader(getFileContents("meta/meta2.html")))

			ExtractMeta(pm, nil)

			assert.Equal(t, "为何前端圈现在不关注源码了？", ex.Drop().Title)
			assert.Equal(t, "", ex.Drop().Description)
			assert.Equal(t, "掘金", ex.Drop().Site)
			assert.Equal(t, []string{"Another Human", "前端双越老师"}, ex.Drop().Authors)
		})

		t.Run("text direction", func(t *testing.T) {
			tests := []struct {
				dir      string
				expected string
			}{
				{"", ""},
				{"rtl", "rtl"},
				{"ltr", "ltr"},
				{"invalid", ""},
			}

			for i, test := range tests {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					assert := require.New(t)
					ex, err := extract.New("http://example.net/", nil)
					assert.NoError(err)
					pm := ex.NewProcessMessage(extract.StepDom)
					pm.Dom, err = html.Parse(strings.NewReader(`<html dir="` + test.dir + `"><html>`))
					assert.NoError(err)

					ExtractMeta(pm, nil)
					assert.Equal(test.expected, ex.Drop().TextDirection)
				})
			}
		})
	})

	t.Run("ExtractOembed", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "http://www.youtube.com/oembed",
			newFileResponder("meta/oembed-video.json"))
		httpmock.RegisterResponder("GET", "https://www.flickr.com/services/oembed",
			newFileResponder("meta/oembed-photo.json"))

		httpmock.RegisterResponder("GET", "/404",
			httpmock.NewJsonResponderOrPanic(404, ""))
		httpmock.RegisterResponder("GET", "/error",
			httpmock.NewErrorResponder(errors.New("HTTP")))

		t.Run("bad step", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepBody)
			ExtractOembed(pm, nil)
			assert.Equal(extract.DropMeta{}, ex.Drop().Meta)
		})

		t.Run("nil meta", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/meta1.html")))
			assert.NoError(err)
			ex.Drop().Meta = nil

			ExtractOembed(pm, nil)
			assert.Equal(extract.DropMeta(nil), ex.Drop().Meta)
		})

		t.Run("no meta", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/meta1.html")))
			assert.NoError(err)

			ExtractOembed(pm, nil)
			assert.Equal(extract.DropMeta{}, ex.Drop().Meta)
		})

		t.Run("meta error", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/video.html")))
			assert.NoError(err)

			node, err := htmlquery.Query(pm.Dom, "//link[@href][@type='application/json+oembed']")
			assert.NoError(err)

			dom.SetAttribute(node, "href", "")
			ExtractOembed(pm, nil)
			assert.Equal(extract.DropMeta{}, ex.Drop().Meta)

			dom.SetAttribute(node, "href", "/test/\b0x7f")
			ExtractOembed(pm, nil)
			assert.Equal(extract.DropMeta{}, ex.Drop().Meta)

			dom.SetAttribute(node, "href", "/error")
			ExtractOembed(pm, nil)
			assert.Equal(extract.DropMeta{}, ex.Drop().Meta)

			dom.SetAttribute(node, "href", "/404")
			ExtractOembed(pm, nil)
			assert.Equal(extract.DropMeta{}, ex.Drop().Meta)
		})

		t.Run("process", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/video.html")))
			assert.NoError(err)

			ExtractOembed(pm, nil)

			assert.Equal(extract.DropMeta{
				"oembed.author_name":      {"To Scale:"},
				"oembed.author_url":       {"https://www.youtube.com/channel/UCPdA3AvoSH-d96mLaEjhZyw"},
				"oembed.height":           {"270"},
				"oembed.html":             {"u003ciframe width=\"480\" height=\"270\" src=\"https://www.youtube.com/embed/zR3Igc3Rhfg?feature=oembed\" frameborder=\"0\" allow=\"accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture\" allowfullscreenu003eu003c/iframeu003e"},
				"oembed.provider_name":    {"YouTube"},
				"oembed.provider_url":     {"https://www.youtube.com/"},
				"oembed.thumbnail_height": {"360"},
				"oembed.thumbnail_url":    {"https://i.ytimg.com/vi/zR3Igc3Rhfg/hqdefault.jpg"},
				"oembed.thumbnail_width":  {"480"},
				"oembed.title":            {"To Scale: The Solar System"},
				"oembed.type":             {"video"},
				"oembed.version":          {"1.0"},
				"oembed.width":            {"480"},
			}, ex.Drop().Meta)
		})
	})

	t.Run("SetDropProperties", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "http://www.youtube.com/oembed",
			newFileResponder("meta/oembed-video.json"))
		httpmock.RegisterResponder("GET", "https://www.flickr.com/services/oembed",
			newFileResponder("meta/oembed-photo.json"))

		t.Run("bad step", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepBody)
			SetDropProperties(pm, nil)
			assert.Equal(extract.DropMeta{}, ex.Drop().Meta)
		})

		t.Run("process", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/meta1.html")))
			assert.NoError(err)

			ExtractMeta(pm, nil)
			ExtractOembed(pm, nil)
			SetDropProperties(pm, nil)

			assert.Equal("article", ex.Drop().DocumentType)
			assert.Equal(time.Date(2020, 9, 1, 11, 12, 34, 0, time.Local), ex.Drop().Date)
		})

		t.Run("process video", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/video.html")))
			assert.NoError(err)

			ExtractMeta(pm, nil)
			ExtractOembed(pm, nil)
			SetDropProperties(pm, nil)

			assert.Equal(extract.DropMeta{
				"graph.description":           {"On a dry lakebed in Nevada, a group of friends build the first scale model of the solar system with complete planetary orbits: a true illustration of our pla..."},
				"graph.image":                 {"https://i.ytimg.com/vi/zR3Igc3Rhfg/maxresdefault.jpg"},
				"graph.image:height":          {"720"},
				"graph.image:width":           {"1280"},
				"graph.site_name":             {"YouTube"},
				"graph.title":                 {"To Scale: The Solar System"},
				"graph.type":                  {"video.other"},
				"graph.url":                   {"https://www.youtube.com/watch?v=zR3Igc3Rhfg"},
				"graph.video:height":          {"720"},
				"graph.video:secure_url":      {"https://www.youtube.com/embed/zR3Igc3Rhfg"},
				"graph.video:tag":             {"Scale", "Science", "Solar System (Star System)", "Astronomy (Field Of Study)"},
				"graph.video:type":            {"text/html"},
				"graph.video:url":             {"https://www.youtube.com/embed/zR3Igc3Rhfg"},
				"graph.video:width":           {"1280"},
				"html.description":            {"Enjoy the videos and music you love, upload original content and share it all with friends, family and the world on YouTube."},
				"html.dir":                    {"ltr"},
				"html.keywords":               {"video, sharing, camera phone, video phone, free, upload"},
				"html.lang":                   {"en-GB"},
				"html.title":                  {"YouTube"},
				"link.alternate":              {"android-app://com.google.android.youtube/http/www.youtube.com/watch?v=zR3Igc3Rhfg", "ios-app://544007664/vnd.youtube/www.youtube.com/watch?v=zR3Igc3Rhfg", "http://www.youtube.com/oembed?format=json&url=https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3DzR3Igc3Rhfg", "http://www.youtube.com/oembed?format=xml&url=https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3DzR3Igc3Rhfg"},
				"link.manifest":               {"/s/notifications/manifest/manifest.json"},
				"link.search":                 {"https://www.youtube.com/opensearch?locale=en_GB"},
				"oembed.author_name":          {"To Scale:"},
				"oembed.author_url":           {"https://www.youtube.com/channel/UCPdA3AvoSH-d96mLaEjhZyw"},
				"oembed.height":               {"270"},
				"oembed.html":                 {"u003ciframe width=\"480\" height=\"270\" src=\"https://www.youtube.com/embed/zR3Igc3Rhfg?feature=oembed\" frameborder=\"0\" allow=\"accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture\" allowfullscreenu003eu003c/iframeu003e"},
				"oembed.provider_name":        {"YouTube"},
				"oembed.provider_url":         {"https://www.youtube.com/"},
				"oembed.thumbnail_height":     {"360"},
				"oembed.thumbnail_url":        {"https://i.ytimg.com/vi/zR3Igc3Rhfg/hqdefault.jpg"},
				"oembed.thumbnail_width":      {"480"},
				"oembed.title":                {"To Scale: The Solar System"},
				"oembed.type":                 {"video"},
				"oembed.version":              {"1.0"},
				"oembed.width":                {"480"},
				"twitter.app:id:googleplay":   {"com.google.android.youtube"},
				"twitter.app:id:ipad":         {"544007664"},
				"twitter.app:id:iphone":       {"544007664"},
				"twitter.app:name:googleplay": {"YouTube"},
				"twitter.app:name:ipad":       {"YouTube"},
				"twitter.app:name:iphone":     {"YouTube"},
				"twitter.app:url:googleplay":  {"https://www.youtube.com/watch?v=zR3Igc3Rhfg"},
				"twitter.app:url:ipad":        {"vnd.youtube://www.youtube.com/watch?v=zR3Igc3Rhfg&feature=applinks"},
				"twitter.app:url:iphone":      {"vnd.youtube://www.youtube.com/watch?v=zR3Igc3Rhfg&feature=applinks"},
				"twitter.card":                {"player"},
				"twitter.description":         {"On a dry lakebed in Nevada, a group of friends build the first scale model of the solar system with complete planetary orbits: a true illustration of our pla..."},
				"twitter.image":               {"https://i.ytimg.com/vi/zR3Igc3Rhfg/maxresdefault.jpg"},
				"twitter.player":              {"https://www.youtube.com/embed/zR3Igc3Rhfg"},
				"twitter.player:height":       {"720"},
				"twitter.player:width":        {"1280"},
				"twitter.site":                {"@youtube"},
				"twitter.title":               {"To Scale: The Solar System"},
				"twitter.url":                 {"https://www.youtube.com/watch?v=zR3Igc3Rhfg"},
			}, ex.Drop().Meta)

			assert.Equal("video", ex.Drop().DocumentType)
			assert.Equal([]string{"To Scale:"}, ex.Drop().Authors)
			assert.Equal("YouTube", ex.Drop().Site)
		})

		t.Run("process photo", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/photo.html")))
			assert.NoError(err)

			ExtractMeta(pm, nil)
			ExtractOembed(pm, nil)
			SetDropProperties(pm, nil)

			assert.Equal(extract.DropMeta{
				"html.lang":               {"en"},
				"html.title":              {"Document"},
				"link.alternative":        {"https://www.flickr.com/services/oembed?url=https://www.flickr.com/photos/randomwire/9050180936&format=json"},
				"oembed.author_name":      {"randomwire"},
				"oembed.author_url":       {"https://www.flickr.com/photos/randomwire/"},
				"oembed.cache_age":        {"3600"},
				"oembed.height":           {"575"},
				"oembed.html":             {"<a data-flickr-embed=\"true\" href=\"https://www.flickr.com/photos/randomwire/9050180936/\" title=\"Authentically Dingy Bathroom by randomwire, on Flickr\"><img src=\"https://live.staticflickr.com/7302/9050180936_43804d2e1c_b.jpg\" width=\"1024\" height=\"575\" alt=\"Authentically Dingy Bathroom\"></a><script async src=\"https://embedr.flickr.com/assets/client-code.js\" charset=\"utf-8\"></script>"},
				"oembed.provider_name":    {"Flickr"},
				"oembed.provider_url":     {"https://www.flickr.com/"},
				"oembed.thumbnail_height": {"150"},
				"oembed.thumbnail_url":    {"https://live.staticflickr.com/7302/9050180936_43804d2e1c_q.jpg"},
				"oembed.thumbnail_width":  {"150"},
				"oembed.title":            {"Authentically Dingy Bathroom"},
				"oembed.type":             {"photo"},
				"oembed.url":              {"https://live.staticflickr.com/7302/9050180936_43804d2e1c_b.jpg"},
				"oembed.version":          {"1.0"},
				"oembed.width":            {"1024"},
				"x.picture_url":           {"https://live.staticflickr.com/7302/9050180936_43804d2e1c_b.jpg"},
			}, ex.Drop().Meta)

			assert.Equal("photo", ex.Drop().DocumentType)
			assert.Equal([]string{"randomwire"}, ex.Drop().Authors)
			assert.Equal("Flickr", ex.Drop().Site)
		})
	})

	t.Run("ExtractPicture", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", "/img.jpeg",
			newFileResponder("images/img.jpeg"))
		httpmock.RegisterResponder("GET", "/404",
			httpmock.NewJsonResponderOrPanic(404, ""))

		t.Run("bad step", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepBody)
			ExtractPicture(pm, nil)
			assert.Empty(ex.Drop().Pictures)
		})

		t.Run("errors", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/meta1.html")))
			assert.NoError(err)

			d := ex.Drop()

			d.Meta = nil
			ExtractPicture(pm, nil)
			assert.Empty(d.Pictures)

			d.Meta = extract.DropMeta{
				"graph.image": {""},
			}
			ExtractPicture(pm, nil)
			assert.Empty(d.Pictures)

			d.Meta["graph.image"] = []string{"http://example.net/404"}
			ExtractPicture(pm, nil)
			assert.Empty(d.Pictures)
		})

		t.Run("process", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepFinish)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/meta1.html")))
			assert.NoError(err)

			d := ex.Drop()
			d.Meta = extract.DropMeta{
				"graph.image": {"http://example.net/img.jpeg"},
			}

			ExtractPicture(pm, nil)
			assert.Equal(800, d.Pictures["image"].Size[0])
			assert.Equal(380, d.Pictures["thumbnail"].Size[0])
		})

		t.Run("process photo", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepFinish)
			pm.Dom, err = html.Parse(bytes.NewReader(getFileContents("meta/meta1.html")))
			assert.NoError(err)

			d := ex.Drop()
			d.DocumentType = "photo"

			d.Meta = extract.DropMeta{
				"x.picture_url": {"http://example.net/img.jpeg"},
			}

			ExtractPicture(pm, nil)
			assert.Equal(1452, d.Pictures["image"].Size[0])
			assert.Equal(380, d.Pictures["thumbnail"].Size[0])
		})
	})

	t.Run("ExtractFavicon", func(t *testing.T) {
		doc := dom.CreateElement("head")
		icon1 := dom.CreateElement("link")
		dom.SetAttribute(icon1, "rel", "icon")
		dom.SetAttribute(icon1, "href", "/icon.ico")

		icon2 := dom.CreateElement("link")
		dom.SetAttribute(icon2, "rel", "shortcut icon")
		dom.SetAttribute(icon2, "href", "/favicon.png")
		dom.SetAttribute(icon2, "type", "image/png")
		dom.SetAttribute(icon2, "sizes", "64x64")

		icon3 := dom.CreateElement("link")
		dom.SetAttribute(icon3, "rel", "icon")
		dom.SetAttribute(icon3, "href", "/\b0x7ficon.png")

		dom.AppendChild(doc, icon1)
		dom.AppendChild(doc, icon2)
		dom.AppendChild(doc, icon3)

		base, _ := url.Parse("http://example.net/")

		t.Run("favicon", func(t *testing.T) {
			assert := require.New(t)
			var fi *extract.Picture
			var err error

			node := dom.CreateElement("link")
			dom.SetAttribute(node, "href", "/favicon.png")
			dom.SetAttribute(node, "type", "image/png")
			dom.SetAttribute(node, "sizes", "64x64 32x32")

			fi, err = newFavicon(node, base)
			assert.NoError(err)
			assert.Equal("http://example.net/favicon.png", fi.Href)
			assert.Equal("image/png", fi.Type)
			assert.Equal([2]int{64, 64}, fi.Size)

			dom.SetAttribute(node, "href", "/\b0x7ficon.png")
			fi, err = newFavicon(node, base)
			assert.Nil(fi)
			assert.Error(err)

			dom.SetAttribute(node, "href", "/favicon.png")
			dom.RemoveAttribute(node, "sizes")
			dom.RemoveAttribute(node, "type")

			fi, err = newFavicon(node, base)
			assert.NoError(err)
			assert.Equal("http://example.net/favicon.png", fi.Href)
			assert.Equal("image/png", fi.Type)
			assert.Equal([2]int{32, 32}, fi.Size)
		})

		t.Run("faviconList", func(t *testing.T) {
			list := newFaviconList(doc, base)

			require.Equal(t, faviconList{
				&extract.Picture{
					Href: "http://example.net/favicon.png",
					Type: "image/png",
					Size: [2]int{64, 64},
				},
				&extract.Picture{
					Href: "http://example.net/icon.ico",
					Type: "image/ico",
					Size: [2]int{32, 32},
				},
				&extract.Picture{
					Href: "http://example.net/favicon.ico",
					Type: "image/ico",
					Size: [2]int{32, 32},
				},
			}, list)
		})

		t.Run("bad step", func(t *testing.T) {
			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepBody)
			ExtractFavicon(pm, nil)
			assert.Empty(ex.Drop().Pictures)
		})

		t.Run("process", func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder("GET", "/icon.ico",
				newFileResponder("images/img.ico"))
			httpmock.RegisterResponder("GET", "/favicon.png",
				httpmock.NewJsonResponderOrPanic(404, ""))

			assert := require.New(t)
			ex, err := extract.New("http://example.net/", nil)
			assert.NoError(err)
			pm := ex.NewProcessMessage(extract.StepDom)
			pm.Dom = doc

			ExtractFavicon(pm, nil)
			p := ex.Drop().Pictures["icon"]
			assert.Equal("http://example.net/icon.ico", p.Href)
			assert.Equal("image/png", p.Type)
			assert.Equal([]byte{137, 80, 78, 71, 13, 10, 26, 10}, p.Bytes()[0:8])
		})
	})
}
