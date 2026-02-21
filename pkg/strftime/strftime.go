// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package strftime provides an strftime() implementation
// for Time formating.
package strftime

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	mark = '%'
)

var longDayNames = []translatable{
	Pgettext("datetime_dl", "Sunday"),
	Pgettext("datetime_dl", "Monday"),
	Pgettext("datetime_dl", "Tuesday"),
	Pgettext("datetime_dl", "Wednesday"),
	Pgettext("datetime_dl", "Thursday"),
	Pgettext("datetime_dl", "Friday"),
	Pgettext("datetime_dl", "Saturday"),
}

var shortDayNames = []translatable{
	Pgettext("datetime_ds", "Sun"),
	Pgettext("datetime_ds", "Mon"),
	Pgettext("datetime_ds", "Tue"),
	Pgettext("datetime_ds", "Wed"),
	Pgettext("datetime_ds", "Thu"),
	Pgettext("datetime_ds", "Fri"),
	Pgettext("datetime_ds", "Sat"),
}

var longMonthNames = []translatable{
	Pgettext("datetime_ml", "January"),
	Pgettext("datetime_ml", "February"),
	Pgettext("datetime_ml", "March"),
	Pgettext("datetime_ml", "April"),
	Pgettext("datetime_ml", "May"),
	Pgettext("datetime_ml", "June"),
	Pgettext("datetime_ml", "July"),
	Pgettext("datetime_ml", "August"),
	Pgettext("datetime_ml", "September"),
	Pgettext("datetime_ml", "October"),
	Pgettext("datetime_ml", "November"),
	Pgettext("datetime_ml", "December"),
}

var shortMonthNames = []translatable{
	Pgettext("datetime_ms", "Jan"),
	Pgettext("datetime_ms", "Feb"),
	Pgettext("datetime_ms", "Mar"),
	Pgettext("datetime_ms", "Apr"),
	Pgettext("datetime_ms", "May"),
	Pgettext("datetime_ms", "Jun"),
	Pgettext("datetime_ms", "Jul"),
	Pgettext("datetime_ms", "Aug"),
	Pgettext("datetime_ms", "Sep"),
	Pgettext("datetime_ms", "Oct"),
	Pgettext("datetime_ms", "Nov"),
	Pgettext("datetime_ms", "Dec"),
}

var (
	anteMeridiem           = Pgettext("datetime", "a.m.")
	postMeridiem           = Pgettext("datetime", "p.m.")
	dateTimeRepresentation = Pgettext("datetime", "%a %b %e %H:%M:%S %y")
	dateRepresentation     = Pgettext("datetime", "%m/%d/%Y")
	timeRepresentation     = Pgettext("datetime", "%H:%M:%S")
)

// translator is an interface for type implementing a pgettext method without extra argument.
type translator interface {
	Pgettext(string, string) string
}

// localeTranslator is an interface for types implementing a full pgettext method.
type localeTranslator interface {
	Pgettext(string, string, ...any) string
}

type translatable string

func (t translatable) Translate(ctx string, tr translator) string {
	return tr.Pgettext(ctx, string(t))
}

func newTranslatable(_, s string) translatable {
	return translatable(s)
}

// gettextWrapper is a wrapper that implements [translator] around a [localeTranslator].
type gettextWrapper struct {
	Translator localeTranslator
}

func (l *gettextWrapper) Pgettext(ctx, s string) string {
	return l.Translator.Pgettext(ctx, s)
}

// Pgettext is an alias for newTranslatable so it can be picked up by a locales extractor.
var Pgettext = newTranslatable

// Formatter is the time formatter.
type Formatter struct {
	tr translator
}

// New returns a new [Formatter] with a given translator.
func New(tr localeTranslator) *Formatter {
	return &Formatter{&gettextWrapper{tr}}
}

// Strftime returns a formatted time string.
func (f *Formatter) Strftime(format string, t time.Time) string {
	buf := &bytes.Buffer{}
	r := bufio.NewReader(strings.NewReader(format))

	for {
		ch, _, err := r.ReadRune()
		if err != nil {
			break
		}

		if ch == mark {
			s, _, err := r.ReadRune()
			if err != nil {
				break
			}
			buf.WriteString(f.repr(s, t))
			continue
		}

		buf.WriteRune(ch)
	}

	return buf.String()
}

func (f *Formatter) repr(ch rune, t time.Time) string { //nolint:gocyclo
	switch ch {
	case 'A':
		// 	national representation of the full weekday name
		return longDayNames[t.Weekday()].Translate("datetime_dl", f.tr)
	case 'a':
		// 	national representation of the abbreviated weekday
		return shortDayNames[t.Weekday()].Translate("datetime_ds", f.tr)
	case 'B':
		// 	national representation of the full month name
		return longMonthNames[t.Month()-1].Translate("datetime_ml", f.tr)
	case 'b', 'h':
		// 	national representation of the abbreviated month name
		return shortMonthNames[t.Month()-1].Translate("datetime_ms", f.tr)
	case 'C':
		// 	(year / 100) as decimal number; single digits are preceded by a zero
		return fmt.Sprintf("%02d", t.Year()/100)
	case 'c':
		// 	national representation of time and date
		return f.Strftime(dateTimeRepresentation.Translate("datetime", f.tr), t)
	case 'D':
		// 	equivalent to %m/%d/%y
		return fmt.Sprintf("%02d/%02d/%04d", t.Month(), t.Day(), t.Year())
	case 'd':
		// 	day of the month as a decimal number (01-31)
		return fmt.Sprintf("%02d", t.Day())
	case 'e':
		// 	the day of the month as a decimal number (1-31)
		return strconv.Itoa(t.Day())
	case 'F':
		// 	equivalent to %Y-%m-%d
		return fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
	case 'H':
		// 	the hour (24-hour clock) as a decimal number (00-23)
		return fmt.Sprintf("%02d", t.Hour())
	case 'I':
		// 	the hour (12-hour clock) as a decimal number (01-12)
		return fmt.Sprintf("%02d", twelveHour(t))
	case 'j':
		// 	the day of the year as a decimal number (001-366)
		return fmt.Sprintf("%03d", t.YearDay())
	case 'k':
		// 	the hour (24-hour clock) as a decimal number (0-23)
		return strconv.Itoa(t.Hour())
	case 'l':
		// 	the hour (12-hour clock) as a decimal number (1-12)
		return strconv.Itoa(twelveHour(t))
	case 'M':
		// 	the minute as a decimal number (00-59)
		return fmt.Sprintf("%02d", t.Minute())
	case 'm':
		// 	the month as a decimal number (01-12)
		return fmt.Sprintf("%02d", t.Month())
	case 'n':
		// 	a newline
		return "\n"
	case 'p':
		// 	national representation of either "ante meridiem" (a.m.) or "post meridiem" (p.m.) as appropriate.
		return amPm(t).Translate("datetime", f.tr)
	case 'R':
		// 	equivalent to %H:%M
		return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
	case 'r':
		// 	equivalent to %I:%M:%S %p
		return fmt.Sprintf("%02d:%02d:%02d %s", twelveHour(t), t.Minute(), t.Second(), amPm(t).Translate("datetime", f.tr))
	case 'S':
		// 	the second as a decimal number (00-60)
		return fmt.Sprintf("%02d", t.Second())
	case 'T':
		// 	equivalent to %H:%M:%S
		return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
	case 't':
		// 	a tab
		return "\t"
	case 'U':
		// 	the week number of the year (Sunday as the first day of the week) as a decimal number (00-53)
		return fmt.Sprintf("%02d", weekNumber(t, false))
	case 'u':
		// 	the weekday (Monday as the first day of the week) as a decimal number (1-7)
		return fmt.Sprintf("%d", 7-t.Weekday())
	case 'V':
		// 	the week number of the year (Monday as the first day of the week) as a decimal number (01-53)
		return fmt.Sprintf("%02d", weekNumber(t, true))
	case 'v':
		// 	equivalent to %e-%b-%Y
		return fmt.Sprintf("%d-%s-%04d", t.Day(), shortMonthNames[t.Month()-1].Translate("datetime_ms", f.tr), t.Year())
	case 'W':
		// 	the week number of the year (Monday as the first day of the week) as a decimal number (00-53)
		return fmt.Sprintf("%02d", weekNumber(t, true)-1)
	case 'w':
		// 	the weekday (Sunday as the first day of the week) as a decimal number (0-6)
		return fmt.Sprintf("%d", t.Weekday())
	case 'X':
		// 	national representation of the time
		return f.Strftime(timeRepresentation.Translate("datetime", f.tr), t)
	case 'x':
		// 	national representation of the date
		return f.Strftime(dateRepresentation.Translate("datetime", f.tr), t)
	case 'Y':
		// 	the year with century as a decimal number
		return fmt.Sprintf("%04d", t.Year())
	case 'y':
		// 	the year without century as a decimal number (00-99)
		return fmt.Sprintf("%02d", t.Year()%100)
	case 'Z':
		// 	the time zone name
		return t.Format("MST")
	case 'z':
		// 	the time zone offset from UTC
		return t.Format("-0700")

	case mark:
		return string(mark)
	default:
		return string([]rune{mark, ch})
	}
}

func twelveHour(t time.Time) int {
	hr := t.Hour() % 12
	if hr == 0 {
		hr = 12
	}
	return hr
}

func amPm(t time.Time) translatable {
	if t.Hour() >= 12 {
		return postMeridiem
	}
	return anteMeridiem
}

func weekNumber(t time.Time, startsOnMonday bool) int {
	offset := int(t.Weekday())
	if startsOnMonday {
		offset = 6 - offset
	} else if offset != 0 {
		offset = 7 - offset
	}
	return (t.YearDay() + offset) / 7
}

type dummyTranslator struct{}

func (t *dummyTranslator) Pgettext(_, s string, _ ...any) string {
	return s
}

var defaultFormatter = New(&dummyTranslator{})

// Strftime returns a formatted string in English.
func Strftime(format string, t time.Time) string {
	return defaultFormatter.Strftime(format, t)
}
