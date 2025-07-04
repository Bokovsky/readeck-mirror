// SPDX-FileCopyrightText: © 2017 Mads Jacobsen
//
// SPDX-License-Identifier: MIT

// Original code: https://github.com/lukasbob/srcset

// Package srcset is an srcset value parser.
package srcset

import (
	"regexp"
	"strconv"
	"strings"
)

// ImageSource is a structure that contains an image definition.
type ImageSource struct {
	URL     string
	Width   int64
	Height  int64
	Density float64
}

// SourceSet is the result of parsing the value of a srcset attribute.
// A SourceSet consists of multiple ImageSource instances.
type SourceSet []ImageSource

const (
	comma       = ','
	leftParens  = '('
	rightParens = ')'
)

const (
	stateNone = iota
	stateInDescriptor
	stateInParens
	stateAfterDescriptor
)

var (
	regexLeadingSpaces         = regexp.MustCompile("^[ \t\n\r\u000c]+")
	regexLeadingCommasOrSpaces = regexp.MustCompile("^[, \t\n\r\u000c]+")
	regexLeadingNotSpaces      = regexp.MustCompile("^[^ \t\n\r\u000c]+")
	regexTrailingCommas        = regexp.MustCompile("[,]+$")
	regexNonNegativeInteger    = regexp.MustCompile(`^\d+$`)
	regexFloatingPoint         = regexp.MustCompile(`^-?(?:[0-9]+|[0-9]*\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)
)

func isSpace(c rune) bool {
	switch c {
	case
		'\u0020', // space
		'\u0009', // horizontal tab
		'\u000A', // new line
		'\u000C', // form feed
		'\u000D': // carriage return
		return true
	default:
		return false
	}
}

// Parse takes the value of a srcset attribute and parses it.
//
//nolint:gocognit,gocyclo
func Parse(input string) SourceSet {
	var (
		url         string
		pos         = 0
		currState   = stateNone
		end         = len(input)
		candidates  = SourceSet{}
		descriptors = []string{}
	)

	collectChars := func(rx *regexp.Regexp) string {
		if match := rx.FindString(input[pos:]); match != "" {
			pos += len(match)
			return match
		}

		return ""
	}

	parseDescriptors := func() {
		var (
			isErr = false
			h     int64
			w     int64
			d     float64
		)

		for _, desc := range descriptors {
			lastIdx := len(desc) - 1
			lastChar, numericVal := desc[lastIdx], desc[:lastIdx]
			intVal, intErr := strconv.ParseInt(numericVal, 10, 64)
			floatVal, floatErr := strconv.ParseFloat(numericVal, 64)

			switch {
			case regexNonNegativeInteger.MatchString(numericVal) && lastChar == 'w':
				if w != 0 || d != 0 {
					isErr = true
				}
				if intErr != nil || intVal == 0 {
					isErr = true
				} else {
					w = intVal
				}
			case regexFloatingPoint.MatchString(numericVal) && lastChar == 'x':
				if w != 0 || d != 0 || h != 0 {
					isErr = true
				}
				if floatErr != nil || floatVal < 0 {
					isErr = true
				} else {
					d = floatVal
				}
			case regexNonNegativeInteger.MatchString(numericVal) && lastChar == 'h':
				if h != 0 || d != 0 {
					isErr = true
				}
				if intErr != nil || intVal == 0 {
					isErr = true
				} else {
					h = intVal
				}
			default:
				isErr = true
			}
		}

		if !isErr {
			candidates = append(candidates, ImageSource{
				URL:     url,
				Density: d,
				Width:   w,
				Height:  h,
			})
		}
	}

	tokenize := func() {
		collectChars(regexLeadingSpaces)
		currDescriptor := ""
		currState = stateInDescriptor

		for {
			if pos == len(input) {
				if currState != stateAfterDescriptor && currDescriptor != "" {
					descriptors = append(descriptors, currDescriptor)
				}

				parseDescriptors()
				return
			}

			c := rune(input[pos])

			switch currState {
			case stateInDescriptor:
				switch {
				case isSpace(c):
					if currDescriptor != "" {
						descriptors = append(descriptors, currDescriptor)
						currDescriptor = ""
						currState = stateAfterDescriptor
					}
				case c == comma:
					pos++
					if currDescriptor != "" {
						descriptors = append(descriptors, currDescriptor)
						parseDescriptors()
						return
					}
				case c == leftParens:
					currDescriptor += string(c)
					currState = stateInParens
				default:
					currDescriptor += string(c)
				}

			case stateInParens:
				switch c {
				case rightParens:
					currDescriptor += string(c)
					currState = stateInDescriptor
				default:
					currDescriptor += string(c)
				}

			case stateAfterDescriptor:
				switch {
				case isSpace(c):
				default:
					currState = stateInDescriptor
					pos--
				}
			}

			pos++
		}
	}

	for {
		collectChars(regexLeadingCommasOrSpaces)
		if pos >= end {
			return candidates
		}

		url = collectChars(regexLeadingNotSpaces)
		descriptors = []string{}

		if url[len(url)-1] == ',' {
			url = regexTrailingCommas.ReplaceAllString(url, "")
			parseDescriptors()
		} else {
			tokenize()
		}
	}
}

func (s ImageSource) String() string {
	b := new(strings.Builder)
	b.WriteString(s.URL)

	if s.Width > 0 {
		b.WriteString(" " + strconv.FormatInt(s.Width, 10) + "w")
	}
	if s.Height > 0 {
		b.WriteString(" " + strconv.FormatInt(s.Height, 10) + "h")
	}
	if s.Density > 0 {
		b.WriteString(" " + strconv.FormatFloat(s.Density, 'f', -1, 64) + "x")
	}

	return b.String()
}

func (s SourceSet) String() string {
	res := make([]string, len(s))
	for i, p := range s {
		res[i] = p.String()
	}

	return strings.Join(res, ", ")
}
