// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package dataset

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"codeberg.org/readeck/readeck/internal/bookmarks"
)

// BookmarkDiagnosis contains the log and properties of a bookmark.
type BookmarkDiagnosis struct {
	Log   []byte
	Props []byte
}

// LogLine is a log line with its level and content.
type LogLine struct {
	Level string
	Line  string
}

// NewBokmarkDiagnosis returns a [BookmarkDiagnosis] from a [bookmarks.Bookmark].
func NewBokmarkDiagnosis(b *bookmarks.Bookmark) (*BookmarkDiagnosis, error) {
	c, err := b.OpenContainer()
	if err != nil {
		return nil, err
	}
	defer c.Close()

	res := &BookmarkDiagnosis{}
	if res.Log, err = c.GetFile("log"); err != nil {
		res.Log = []byte(fmt.Errorf("no log found: %W", err).Error())
	}

	if res.Props, err = c.GetFile("props.json"); err != nil {
		res.Log = []byte(fmt.Errorf("no properties found: %W", err).Error())
	}

	return res, nil
}

// LogLines return a list of [LogLine].
func (d *BookmarkDiagnosis) LogLines() []LogLine {
	res := []LogLine{}

	// We need to scan each line and accumulate them until we find a new level
	// token. We can't just split by NL because some log "line" can contain a NL
	// character.
	getToken := func(s string) (string, bool) {
		if strings.HasPrefix(s, "[DEBU]") || strings.HasPrefix(s, "[INFO]") ||
			strings.HasPrefix(s, "[WARN]") || strings.HasPrefix(s, "[ERRO]") {
			return s[1:5], true
		}
		return "", false
	}
	s := bufio.NewScanner(bytes.NewReader(d.Log))

	p := LogLine{}
	for s.Scan() {
		l := s.Text()
		var ok bool
		t, ok := getToken(l)
		if ok {
			if len(p.Line) > 0 {
				res = append(res, p)
				p = LogLine{}
			}
			p.Level = t
			p.Line = strings.TrimSpace(l[6:])
		} else {
			p.Line += l
		}
	}

	if len(p.Line) > 0 {
		res = append(res, p)
	}

	return res
}
