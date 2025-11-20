// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package searchstring provides a search string parser.
package searchstring

import (
	"bufio"
	"bytes"
	"io"
	"slices"
	"strconv"
	"strings"
)

// kind is the type of parsed kind.
type kind int

const (
	// EOF is the end of file token.
	EOF kind = iota
	// SPACE is a space token.
	SPACE
	// FIELD is a field token.
	FIELD
	// NEG is a negation token.
	NEG
	// WILDCARD is a wildcard token.
	WILDCARD
	// STR is a string token.
	STR
	// STRQ is a quoted string.
	STRQ
)

// eof is the EOF rune.
var eof = rune(0)

// token is a parsed token.
type token struct {
	kind  kind
	value string
}

// scanner is the search string scanner.
type scanner struct {
	*bufio.Reader
}

// newScanner returns a new instance of Scanner.
func newScanner(rd io.Reader) *scanner {
	return &scanner{Reader: bufio.NewReader(rd)}
}

// next reads the next rune in the string.
func (s scanner) next() rune {
	ch, _, err := s.ReadRune()
	if err != nil {
		return eof
	}
	return ch
}

// prev rewinds the scanner position to the previous rune.
func (s scanner) prev() {
	s.UnreadRune() //nolint:errcheck
}

// scan scans the next token in the string.
func (s scanner) scan() token {
	for {
		ch := s.next()
		switch {
		case ch == eof:
			return token{kind: EOF}
		case ch == '-':
			return token{kind: NEG, value: string(ch)}
		case ch == ':':
			return token{kind: FIELD, value: string(ch)}
		case ch == '*':
			return token{kind: WILDCARD, value: string(ch)}
		case ch == '"' || ch == '\'':
			return token{kind: STRQ, value: s.scanQuoted(ch)}
		case isSpace(ch):
			return token{kind: SPACE, value: " "}
		default:
			s.prev()
			return token{kind: STR, value: s.scanString()}
		}
	}
}

// scanString scans a regular (unquoted) string.
func (s scanner) scanString() string {
	var b bytes.Buffer
	b.WriteRune(s.next())

	ch := s.next()
	for ch != eof {
		if ch == ':' || ch == '*' || isSpace(ch) {
			s.prev()
			break
		}
		b.WriteRune(ch)
		ch = s.next()
	}

	return b.String()
}

// scanQuoted scans a quoted string.
func (s scanner) scanQuoted(delim rune) string {
	// Scan until we find a closing " or EOF
	var b bytes.Buffer

	ch := s.next()
	for ch != eof {
		if ch == '\\' { //nolint:gocritic
			switch c := s.next(); c {
			case delim, '\\':
				b.WriteRune(c)
			default:
				b.WriteRune('\\')
				b.WriteRune(c)
			}
		} else if ch == delim {
			break
		} else if isSpace(ch) {
			b.WriteString(" ")
		} else {
			b.WriteRune(ch)
		}
		ch = s.next()
	}

	return b.String()
}

// isSpace returns true if the rune is a space.
func isSpace(r rune) bool {
	if r <= '\u00FF' {
		// Obvious ASCII ones: \t through \r plus space. Plus two Latin-1 oddballs.
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r':
			return true
		case '\u0085', '\u00A0':
			return true
		}
		return false
	}
	// High-valued ones.
	if '\u2000' <= r && r <= '\u200a' {
		return true
	}
	switch r {
	case '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000':
		return true
	}
	return false
}

func parseTokens(rd io.Reader) [][]token {
	s := newScanner(rd)
	tokens := [][]token{}

	// Cut the list on space tokens
	tok := s.scan()
	for tok.kind != EOF {
		switch tok.kind {
		case SPACE:
			// A space will "flush" the current token list and add
			// a new empty list to the result.
			// The token is discarded; we don't need it anymore.
			if len(tokens) > 0 && len(tokens[len(tokens)-1]) > 0 {
				tokens = append(tokens, []token{})
			}
		default:
			idx := len(tokens) - 1
			if idx < 0 {
				tokens = append(tokens, []token{})
				idx = 0
			}
			tokens[idx] = append(tokens[idx], tok)
		}

		tok = s.scan()
	}

	return slices.DeleteFunc(tokens, func(t []token) bool {
		return len(t) == 0
	})
}

// SearchTerm is a search term part.
type SearchTerm struct {
	Field    string
	Value    string
	Exact    bool
	Exclude  bool
	Wildcard bool
}

// String returns a term's string.
func (st SearchTerm) String() string {
	b := strings.Builder{}
	if st.Exclude {
		b.WriteRune('-')
	}
	if st.Field != "" {
		b.WriteString(st.Field)
		b.WriteRune(':')
	}
	if st.Exact {
		b.WriteString(strconv.Quote(st.Value))
	} else {
		b.WriteString(st.Value)
	}

	if st.Wildcard {
		b.WriteRune('*')
	}

	return b.String()
}

func newSearchTerm(tl []token, ignoreField bool) SearchTerm {
	res := SearchTerm{}

	// Starts with a "-" sign, negates the search term.
	if len(tl) > 0 && tl[0].kind == NEG {
		res.Exclude = true
		tl = tl[1:]
	}

	// Contains a field separator on second position, make it
	// a keyword search using the previous text.
	if !ignoreField && len(tl) > 1 && tl[1].kind == FIELD {
		res.Field = tl[0].value
		tl = tl[2:]
	}

	// The rest becomes the string
	res.Exact = slices.ContainsFunc(tl, func(t token) bool {
		return t.kind == STRQ
	})

	for i, t := range tl {
		if t.kind == WILDCARD && i == len(tl)-1 {
			// A wildcard on the last place makes the query a wildcard search term.
			res.Wildcard = true
		} else {
			res.Value += t.value
		}
	}

	// unquote value
	if res.Exact {
		if x, err := strconv.Unquote(`"` + res.Value + `"`); err == nil {
			res.Value = x
		}
	}

	return res
}

// SearchQuery is a search query that can be transformed into
// a database query.
type SearchQuery struct {
	Terms []SearchTerm
}

// String returns the query as a string.
func (q SearchQuery) String() string {
	b := strings.Builder{}
	for i, t := range q.Terms {
		if i > 0 {
			b.WriteRune(' ')
		}
		b.WriteString(t.String())
	}
	return b.String()
}

// Dedup returns a new SearchQuery without duplicate entries.
func (q SearchQuery) Dedup() SearchQuery {
	seen := map[SearchTerm]struct{}{}
	res := SearchQuery{Terms: []SearchTerm{}}
	for _, t := range q.Terms {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		res.Terms = append(res.Terms, t)
	}
	return res
}

// PopField returns a SearchQuery for the given field and the SearchQuery
// without the removed search terms. The initial SearchQuery is unchanged.
func (q SearchQuery) PopField(name string) (sq, nsq SearchQuery) {
	sq = SearchQuery{Terms: []SearchTerm{}}
	nsq = SearchQuery{}
	nsq.Terms = slices.DeleteFunc(q.Terms, func(st SearchTerm) bool {
		if st.Field == name {
			sq.Terms = append(sq.Terms, st)
			return true
		}
		return false
	})

	return
}

// ExtractField returns a SearchQuery with only the terms for
// a specific field.
func (q SearchQuery) ExtractField(name string) SearchQuery {
	res := SearchQuery{Terms: []SearchTerm{}}
	for _, t := range q.Terms {
		if t.Field == name {
			res.Terms = append(res.Terms, t)
		}
	}
	return res
}

// RemoveFieldInfo returns a SearchQuery with the terms stripped of their
// field name.
func (q SearchQuery) RemoveFieldInfo() SearchQuery {
	res := SearchQuery{Terms: []SearchTerm{}}
	for _, t := range q.Terms {
		t.Field = ""
		res.Terms = append(res.Terms, t)
	}
	return res
}

// Unfield transforms every field search term that is not in "names"
// into a normal string.
func (q SearchQuery) Unfield(names ...string) SearchQuery {
	res := SearchQuery{Terms: []SearchTerm{}}
	for _, t := range q.Terms {
		if t.Field == "" || slices.Contains(names, t.Field) {
			res.Terms = append(res.Terms, t)
		} else {
			t2 := t
			t2.Value = t2.Field + ":" + t2.Value
			t2.Field = ""
			res.Terms = append(res.Terms, t2)
		}
	}

	return res
}

// ParseQuery returns a new SearchQuery after parsing
// the input string.
func ParseQuery(s string) SearchQuery {
	q := SearchQuery{Terms: []SearchTerm{}}

	tokens := parseTokens(strings.NewReader(s))

	for _, x := range tokens {
		q.Terms = append(q.Terms, newSearchTerm(x, false))
	}

	return q
}

// ParseField returns a new SearchQuery, ignoring any field definition
// in the tokens, but taking into acount exclusion or wildcards.
// "-test*" becomes a search term with wildcard and exclusion
// for the given label.
func ParseField(s, name string) SearchQuery {
	q := SearchQuery{Terms: []SearchTerm{}}

	tokens := parseTokens(strings.NewReader(s))

	for _, x := range tokens {
		t := newSearchTerm(x, true)
		t.Field = name
		q.Terms = append(q.Terms, t)
	}

	return q
}
