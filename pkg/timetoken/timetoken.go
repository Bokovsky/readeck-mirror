// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package timetoken is a simple utility to convert a text into a time value.
package timetoken

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
)

var relativeTokenRx = regexp.MustCompile(`^([+-])(\d+)([dwmy])$`)

// TimeToken contains the parsed token information.
type TimeToken struct {
	Now      bool
	Factor   int
	Value    int
	Unit     string
	Absolute *time.Time
}

// New parses a text and returns a TimeToken instance.
// The text can be:
// "now", "-<n><unit>", "+<n><unit>" or an absolute time.
func New(s string) (res TimeToken, err error) {
	s = strings.ToLower(strings.TrimSpace(s))

	if s == "now" || s == "" {
		res.Now = true
		return
	}

	if m := relativeTokenRx.FindStringSubmatch(s); m != nil {
		res.Value, err = strconv.Atoi(m[2])
		if err != nil {
			return
		}

		if m[1] == "-" {
			res.Factor = -1
		} else {
			res.Factor = 1
		}

		res.Unit = m[3]
		return
	}

	// try to parse the string as a date in a last resort
	var d time.Time
	d, err = dateparse.ParseAny(s, dateparse.RetryAmbiguousDateWithSwap(true))
	if err != nil {
		err = fmt.Errorf(`cannot parse "%s"`, s)
		return
	}

	res.Absolute = &d
	return
}

// RelativeTo returns the time.Time value relative to the
// time given in its argument. If the input is nil, it's
// relative to time.Now().
func (t *TimeToken) RelativeTo(ts *time.Time) time.Time {
	now := time.Now().UTC()
	if ts != nil {
		now = *ts
	}

	if t.Absolute != nil {
		return *t.Absolute
	}
	if t.Now {
		return now
	}

	switch t.Unit {
	case "d":
		return now.AddDate(0, 0, t.Factor*t.Value)
	case "w":
		return now.AddDate(0, 0, t.Factor*t.Value*7)
	case "m":
		return now.AddDate(0, t.Factor*t.Value, 0)
	case "y":
		return now.AddDate(t.Factor*t.Value, 0, 0)
	}

	return now
}
