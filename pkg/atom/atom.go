// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package atom provides an Atom feed generator.
// See https://datatracker.ietf.org/doc/html/rfc4287
package atom

import (
	"encoding/xml"
	"fmt"
	"io"
	"time"
)

const (
	// NS is the Atom namespace.
	NS = "http://www.w3.org/2005/Atom"
	// MimeType is the Atom mime type. We use application/xml here so it can be showd in a browser.
	MimeType = "application/xml"
)

// Time returns an RFC3339 formatted time.
// https://datatracker.ietf.org/doc/html/rfc4287#section-3.3
func Time(t time.Time) string {
	return t.Format(time.RFC3339)
}

// Person is an Atom contact.
type Person struct {
	Name  string `xml:"name,omitempty"`
	URI   string `xml:"uri,omitempty"`
	Email string `xml:"email,omitempty"`
}

// Summary is an Atom summary.
type Summary struct {
	XMLName xml.Name `xml:"summary"`
	Content string   `xml:",chardata"`
	Type    string   `xml:"type,attr"`
}

// Content is an Atom content.
type Content struct {
	XMLName xml.Name `xml:"content"`
	Content string   `xml:",chardata"`
	Type    string   `xml:"type,attr"`
}

// Author is an Atom author.
type Author struct {
	XMLName xml.Name `xml:"author"`
	Person
}

// Contributor is an Atom contributor.
type Contributor struct {
	XMLName xml.Name `xml:"contributor"`
	Person
}

// Entry is an Atom entry.
type Entry struct {
	XMLName     xml.Name `xml:"entry"`
	Xmlns       string   `xml:"xmlns,attr,omitempty"`
	Title       string   `xml:"title"`   // required
	Updated     string   `xml:"updated"` // required
	ID          string   `xml:"id"`      // required
	Category    string   `xml:"category,omitempty"`
	Content     *Content
	Rights      string `xml:"rights,omitempty"`
	Source      string `xml:"source,omitempty"`
	Published   string `xml:"published,omitempty"`
	Contributor *Contributor
	Links       []*Link
	Summary     *Summary
	Author      *Author
}

// Link is an Atom link entry.
type Link struct {
	// Atom 1.0 <link rel="enclosure" type="audio/mpeg" title="MP3" href="http://www.example.org/myaudiofile.mp3" length="1234" />
	XMLName xml.Name `xml:"link"`
	Href    string   `xml:"href,attr"`
	Rel     string   `xml:"rel,attr,omitempty"`
	Type    string   `xml:"type,attr,omitempty"`
	Length  string   `xml:"length,attr,omitempty"`
}

// Feed is an Atom feed.
type Feed struct {
	XMLName     xml.Name `xml:"feed"`
	Xmlns       string   `xml:"xmlns,attr"`
	Stylesheet  string   `xml:"-"`
	Title       string   `xml:"title"`   // required
	ID          string   `xml:"id"`      // required
	Updated     string   `xml:"updated"` // required
	Category    string   `xml:"category,omitempty"`
	Icon        string   `xml:"icon,omitempty"`
	Logo        string   `xml:"logo,omitempty"`
	Rights      string   `xml:"rights,omitempty"` // copyright used
	Subtitle    string   `xml:"subtitle,omitempty"`
	Link        []*Link
	Author      *Author `xml:"author,omitempty"`
	Contributor *Contributor
	Entries     []*Entry `xml:"entry"`
}

// WriteXML writes the Atom feed as XML into the given [io.Writer].
func (f *Feed) WriteXML(w io.Writer) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}

	if f.Stylesheet != "" {
		if _, err := fmt.Fprintf(w,
			`<?xml-stylesheet type="text/xsl" href="%s"?>`+"\n", f.Stylesheet,
		); err != nil {
			return err
		}
	}

	e := xml.NewEncoder(w)
	e.Indent("", "  ")
	return e.Encode(f)
}
