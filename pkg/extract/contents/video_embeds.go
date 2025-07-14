// SPDX-FileCopyrightText: © 2025 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents

import (
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"codeberg.org/readeck/readeck/pkg/extract"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

var rxYoutubeEmbed = regexp.MustCompile(`\A/embed/([a-zA-Z0-9_-]+)`)

// ConvertVideoEmbeds converts video embed elements (such as iframes) to a
// regular hyperlink for the external video. Currently only works with YouTube.
func ConvertVideoEmbeds(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Dom == nil {
		return next
	}

	dom.ForEachNode(dom.QuerySelectorAll(m.Dom, "iframe"), func(iframe *html.Node, _ int) {
		attrSrc := dom.GetAttribute(iframe, "src")
		srcURL, err := url.Parse(attrSrc)
		if err != nil {
			return
		}
		switch strings.ToLower(srcURL.Hostname()) {
		case "www.youtube.com", "youtube.com", "www.youtube-nocookie.com":
			match := rxYoutubeEmbed.FindStringSubmatch(srcURL.Path)
			if match == nil {
				return
			}
			videoID := match[1]
			videoURL := "https://www.youtube.com/watch?v=" + videoID
			if startAt := srcURL.Query().Get("start"); startAt != "" {
				videoURL = fmt.Sprintf("%s&t=%ss", videoURL, startAt)
			}

			link := dom.CreateElement("a")
			dom.SetAttribute(link, "href", videoURL)
			srcURL.Host = "www.youtube-nocookie.com"
			captionLink := dom.Clone(link, false)
			dom.SetTextContent(captionLink, videoURL)
			m.SetDataAttribute(link, "video-iframe-src", srcURL.String())

			img := dom.CreateElement("img")
			dom.SetAttribute(img, "alt", "YouTube video")
			// Fetching the "maxresdefault" variant of the thumbnail instead of "hqdefault" would be preferrable,
			// but "maxresdefault" is not guaranteed to exist for older or lower-resolution videos.
			dom.SetAttribute(img, "src", "https://i.ytimg.com/vi/"+videoID+"/hqdefault.jpg")

			p := dom.CreateElement("figure")
			dom.AppendChild(link, img)
			dom.AppendChild(p, link)

			caption := dom.CreateElement("figcaption")
			dom.AppendChild(caption, captionLink)
			dom.AppendChild(p, caption)

			dom.ReplaceChild(iframe.Parent, p, iframe)

			m.Log().Debug("converted video embed", slog.String("url", videoURL))
		}
	})

	return next
}
