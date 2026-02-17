// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package strftime_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/strftime"
)

func TestStrftime(t *testing.T) {
	t.Setenv("TZ", "Europe/Amsterdam")

	tests := []struct {
		date     string
		format   string
		expected string
	}{
		{"2006-01-02T15:04:05Z", "%A - %a", "Monday - Mon"},
		{"2006-01-02T15:04:05Z", "%B - %b", "January - Jan"},
		{"2006-01-02T15:04:05Z", "%C", "20"},
		{"2006-01-02T15:04:05Z", "%D", "01/02/2006"},
		{"2006-01-02T15:04:05Z", "%e", "2"},
		{"2006-01-02T15:04:05Z", "%F", "2006-01-02"},
		{"2006-01-02T15:04:05Z", "%h", "Jan"},
		{"2024-02-29T15:04:05Z", "%j", "060"},
		{"2024-12-31T15:04:05Z", "%j", "366"},
		{"2006-01-02T15:04:05Z", "%c", "Mon Jan 2 15:04:05 06"},
		{"2006-01-02T15:04:05Z", "%I", "03"},
		{"2006-01-02T05:04:05Z", "%k", "5"},
		{"2006-01-02T15:04:05Z", "%l", "3"},
		{"2006-01-02T15:04:05Z", "%Y%n", "2006\n"},
		{"2006-01-02T15:04:05Z", "%p", "p.m."},
		{"2006-01-02T00:00:00Z", "%l %p", "12 a.m."},
		{"2006-01-02T12:00:00Z", "%l %p", "12 p.m."},
		{"2006-01-02T15:04:05Z", "%R", "15:04"},
		{"2006-01-02T15:04:05Z", "%r", "03:04:05 p.m."},
		{"2006-01-02T15:04:05Z", "%T", "15:04:05"},
		{"2006-01-02T15:04:05Z", "%t %%", "\t %"},
		{"2006-01-02T15:04:05Z", "%U", "01"},
		{"2024-03-03T15:04:05Z", "%u", "7"},
		{"2006-01-02T15:04:05Z", "%V", "01"},
		{"2024-03-04T15:04:05Z", "%V", "09"},
		{"2006-01-02T15:04:05Z", "%W", "00"},
		{"2024-03-04T15:04:05Z", "%W", "08"},
		{"2024-03-03T15:04:05Z", "%w", "0"},
		{"2006-01-02T15:04:05Z", "%v", "2-Jan-2006"},
		{"2006-01-02T15:04:05Z", "%X", "15:04:05"},
		{"2006-01-02T15:04:05Z", "%x", "01/02/2006"},
		{"2006-01-02T15:04:05Z", "%Z", "UTC"},
		{"2006-01-02T15:04:05+01:00", "%Z", "CET"},
		{"2006-01-02T15:04:05Z", "%z", "+0000"},
		{"2006-01-02T15:04:05+01:00", "%z", "+0100"},
		{"2006-01-02T15:04:05+01:00", "%-", "%-"},

		{"2006-01-02T05:04:05Z", "%A, %e %B %Y at %H:%M:%S %% test%", "Monday, 2 January 2006 at 05:04:05 % test"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			date, err := time.Parse(time.RFC3339, test.date)
			require.NoError(t, err)
			t.Log(date, test.format, strftime.Strftime(test.format, date))

			require.Equal(t, test.expected, strftime.Strftime(test.format, date))
		})
	}
}

func TestTranslator(t *testing.T) {
	t.Setenv("TZ", "Europe/Amsterdam")

	tests := []struct {
		date     string
		format   string
		expected string
	}{
		{"2006-01-02T15:04:05Z", "%A - %a", "lundi - lun"},
		{"2006-01-02T15:04:05Z", "%B - %b", "janvier - jan"},
		{"2006-01-02T15:04:05Z", "%h", "jan"},
		{"2006-01-02T15:04:05Z", "%c", "lun 2 jan 2006 15:04:05"},
		{"2006-01-02T15:04:05Z", "%p", "PM"},
		{"2006-01-02T00:00:00Z", "%l %p", "12 AM"},
		{"2006-01-02T12:00:00Z", "%l %p", "12 PM"},
		{"2006-01-02T15:04:05Z", "%r", "03:04:05 PM"},
		{"2006-01-02T15:04:05Z", "%v", "2-jan-2006"},
		{"2006-01-02T15:04:05Z", "%x", "02/01/2006"},

		{"2006-01-02T05:04:05Z", "%A, %e %B %Y at %H:%M:%S %% test%", "lundi, 2 janvier 2006 at 05:04:05 % test"},
	}

	f := strftime.New(translation)
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			date, err := time.Parse(time.RFC3339, test.date)
			require.NoError(t, err)
			t.Log(date, test.format, f.Strftime(test.format, date))

			require.Equal(t, test.expected, f.Strftime(test.format, date))
		})
	}
}

type translator map[string]map[string]string

func (t translator) Pgettext(ctx string, s string, args ...any) string {
	if v, ok := args[0].(string); len(args) == 1 && ok {
		if r, ok := t[ctx][v]; ok && r != "" {
			return fmt.Sprintf(s, r)
		}
	}
	return s
}

var translation = translator{
	"datetime_dl": map[string]string{
		"Sunday":    "dimanche",
		"Monday":    "lundi",
		"Tuesday":   "mardi",
		"Wednesday": "mercredi",
		"Thursday":  "jeudi",
		"Friday":    "vendredi",
		"Saturday":  "samedi",
	},
	"datetime_ds": map[string]string{
		"Sun": "dim",
		"Mon": "lun",
		"Tue": "mar",
		"Wed": "mer",
		"Thu": "jeu",
		"Fri": "ven",
		"Sat": "sam",
	},
	"datetime_ml": map[string]string{
		"January":   "janvier",
		"February":  "février",
		"March":     "mars",
		"April":     "avril",
		"May":       "mai",
		"June":      "juin",
		"July":      "juillet",
		"August":    "août",
		"September": "septembre",
		"October":   "octobre",
		"November":  "novembre",
		"December":  "décembre",
	},
	"datetime_ms": map[string]string{
		"Jan": "jan",
		"Feb": "fév",
		"Mar": "mar",
		"Apr": "avr",
		"May": "mai",
		"Jun": "jui",
		"Jul": "jui",
		"Aug": "aoû",
		"Sep": "sep",
		"Oct": "oct",
		"Nov": "nov",
		"Dec": "déc",
	},
	"datetime": map[string]string{
		"a.m.":                 "AM",
		"p.m.":                 "PM",
		"%a %b %e %H:%M:%S %y": "%a %e %b %Y %H:%M:%S",
		"%m/%d/%Y":             "%d/%m/%Y",
		"%H:%M:%S":             "",
	},
}
