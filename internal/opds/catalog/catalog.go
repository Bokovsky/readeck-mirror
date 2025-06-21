// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package catalog provides type to create an OPDS catalog of entries.
package catalog

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/bleach"
	"codeberg.org/readeck/readeck/pkg/opds"
)

// Catalog is a wraper around opds.Feed.
type Catalog struct {
	*opds.Feed
}

// New creates a new catalog with some prepared information.
func New(srv *server.Server, r *http.Request, options ...func(*opds.Feed)) *Catalog {
	feed := &opds.Feed{
		FeedType: opds.OPDSTypeNavigation,
		Links: []opds.Link{
			{
				Rel:      "start",
				Href:     urls.AbsoluteURL(r, "/opds").String(),
				TypeLink: opds.OPDSTypeNavigation,
			},
		},
		Entries: []opds.Entry{},
		Author: []opds.Author{
			{
				Name: "Readeck",
				URI:  urls.AbsoluteURL(r, "/").String(),
			},
		},
	}

	for _, f := range options {
		f(feed)
	}

	return &Catalog{feed}
}

// WithFeedType sets the feed type.
func WithFeedType(t string) func(*opds.Feed) {
	return func(feed *opds.Feed) {
		feed.FeedType = t
	}
}

// WithURL sets the "self" feed link entry.
func WithURL(href string) func(*opds.Feed) {
	return func(feed *opds.Feed) {
		feed.ID = opds.URLID(href)
		feed.Links = append(feed.Links, opds.Link{
			Rel:      "self",
			Href:     href,
			TypeLink: opds.OPDSTypeNavigation,
		})
	}
}

// WithLink adds a link entry to the feed.
func WithLink(t, rel, href string) func(*opds.Feed) {
	return func(feed *opds.Feed) {
		feed.Links = append(feed.Links, opds.Link{Rel: rel, Href: href, TypeLink: t})
	}
}

// WithTitle sets the feed's title.
func WithTitle(title string) func(*opds.Feed) {
	return func(feed *opds.Feed) {
		feed.Title = title
	}
}

// WithUpdated sets the feed last update value.
func WithUpdated(t time.Time) func(*opds.Feed) {
	return func(feed *opds.Feed) {
		feed.Updated = *opds.AtomDate(t)
	}
}

// WithNavEntry adds a new navigation entry to the feed.
func WithNavEntry(title string, updated time.Time, href string, options ...func(*opds.Entry)) func(*opds.Feed) {
	return func(feed *opds.Feed) {
		e := opds.Entry{
			Title:      bleach.SanitizeString(title),
			Updated:    *opds.AtomDate(updated),
			Content:    &opds.Content{ContentType: "text", Content: bleach.SanitizeString(title)},
			ID:         opds.URLID(href),
			Identifier: opds.URLID(href),
			Links: []opds.Link{
				{
					Rel:      "subsection",
					Href:     href,
					TypeLink: opds.OPDSTypeAcquisistion,
				},
			},
		}
		for _, f := range options {
			f(&e)
		}
		feed.Entries = append(feed.Entries, e)
	}
}

// WithBookEntry adds a new book entry to the feed.
func WithBookEntry(
	id uuid.UUID, title string, href string,
	issued, published, updated time.Time,
	publisher string, language string, description string,
) func(*opds.Feed) {
	return func(feed *opds.Feed) {
		e := opds.Entry{
			ID:         opds.ID(id),
			Identifier: opds.ID(id),
			Issued:     opds.AtomDate(issued),
			Published:  opds.AtomDate(published),
			Updated:    *opds.AtomDate(updated),
			Title:      bleach.SanitizeString(title),
			Publisher:  bleach.SanitizeString(publisher),
			Language:   language,
			Links: []opds.Link{
				{
					Rel:      "http://opds-spec.org/acquisition",
					TypeLink: "application/epub+zip",
					Href:     href,
				},
			},
		}

		if description != "" {
			e.Content = &opds.Content{
				ContentType: "html",
				Content:     bleach.SanitizeString(description),
			}
		}

		feed.Entries = append(feed.Entries, e)
	}

	// Note: if we want to add an image
	// e.Links = append(e.Links, opds.Link{
	// 	Rel:  "http://opds-spec.org/image",
	// 	Href: imgSrc,
	// })
}

// Render write the full catalog to a writer.
func (c *Catalog) Render(w http.ResponseWriter, r *http.Request) error {
	buf := new(bytes.Buffer)
	err := c.Encode(buf)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", c.FeedType)
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	if r.Method == http.MethodHead {
		return nil
	}

	_, err = io.Copy(w, buf)
	return err
}
