// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package tasks

import (
	"context"
	"log/slog"
	"net/url"
	"slices"

	"github.com/go-shiori/dom"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/pkg/bleach"
	"codeberg.org/readeck/readeck/pkg/ctxr"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/http/linkheader"
)

type ctxExtractLinksKey struct{}

var withExtractLinks, checkExtractLinks = ctxr.WithChecker[bookmarks.BookmarkLinks](ctxExtractLinksKey{})

// OriginalLinkProcessor looks for a rel=original link in HTTP headers.
// If it finds one, it sets the extracted URL to the original one.
// In the special case where a "readeck-original" header is present, it
// fully swaps the extracted page to its original version.
func OriginalLinkProcessor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Position() > 0 {
		return next
	}

	// Look for Readeck-Original and Link rel=original headers.
	var original linkheader.Link
	originalURL := m.Extractor.Drop().Header.Get("readeck-original")
	for _, l := range linkheader.ParseLink(m.Extractor.Drop().Header) {
		if l.Rel == "original" {
			original = l
			break
		}
	}
	if original.URL == "" {
		return next
	}

	u, err := url.Parse(original.URL)
	if err != nil {
		m.Log().Error("cannot parse URL",
			slog.String("url", original.URL),
			slog.Any("err", err),
		)
		return next
	}

	// The link URL equals the "readeck-original" header value.
	// We can then fully swap the requested page.
	if original.URL == originalURL {
		m.Log().Debug("found readeck-originl header", slog.String("url", originalURL))
		if err = m.Extractor.ReplaceDrop(u); err != nil {
			m.Log().Error("cannot replace page", slog.Any("err", err))
			return nil
		}

		m.ResetPosition()
		return nil
	}

	m.Log().Debug("found original link", slog.String("uri", original.URL))
	m.Extractor.Drop().SetURL(u)
	m.Extractor.Drop().Site = u.Hostname()

	return next
}

// CleanDomProcessor is a last pass of cleaning on the resulting DOM node.
// It removes unwanted attributes, empty tags and set some defaults.
func CleanDomProcessor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	if m.Dom == nil {
		return next
	}

	m.Log().Debug("cleaning resulting DOM")

	bleach.DefaultPolicy.Clean(m.Dom)
	bleach.DefaultPolicy.RemoveEmptyNodes(m.Dom)
	bleach.DefaultPolicy.SetLinkRel(m.Dom)

	return next
}

// extractLinksProcessor extracts all the web links (http and https) in the page
// and store the list in the extractor context.
func extractLinksProcessor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	if m.Dom == nil {
		return next
	}

	m.Log().Debug("extract links from content")
	links := bookmarks.BookmarkLinks{}
	seen := map[string]*extract.Drop{}

	for _, node := range dom.QuerySelectorAll(m.Dom, "a[href]") {
		href := dom.GetAttribute(node, "href")
		URL, err := url.Parse(href)
		if err != nil {
			continue
		}
		URL.Fragment = ""

		if _, ok := seen[URL.String()]; ok {
			continue
		}

		d := extract.NewDrop(URL)
		if d.URL.String() == m.Extractor.Drop().URL.String() {
			continue
		}

		if URL.Scheme == "http" || URL.Scheme == "https" {
			seen[URL.String()] = extract.NewDrop(URL)
			links = append(links, bookmarks.BookmarkLink{URL: URL.String(), Domain: d.Domain})
		}
	}

	links = slices.CompactFunc(links, func(a, b bookmarks.BookmarkLink) bool {
		return a.URL == b.URL
	})

	m.Extractor.Context = withExtractLinks(m.Extractor.Context, links)
	return next
}

// GetExtractedLinks returns the extracted link list previously
// stored in the extractor context.
func GetExtractedLinks(ctx context.Context) bookmarks.BookmarkLinks {
	if links, ok := checkExtractLinks(ctx); ok {
		return links
	}
	return bookmarks.BookmarkLinks{}
}
