// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package searchstring

import (
	"fmt"
	"strconv"
	"testing"

	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	"github.com/stretchr/testify/require"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		s        string
		expected []SearchTerm
	}{
		{"\u00A0 simple \t\u200a\n test \u1680", []SearchTerm{
			{Exact: false, Field: "", Value: "simple"},
			{Exact: false, Field: "", Value: "test"},
		}},
		{`"a test" to multiple "unclosed`, []SearchTerm{
			{Exact: true, Field: "", Value: `a test`},
			{Exact: false, Field: "", Value: "to"},
			{Exact: false, Field: "", Value: "multiple"},
			{Exact: true, Field: "", Value: `unclosed`},
		}},
		{fmt.Sprintf("%s %s %s %s",
			strconv.Quote("quoted"),
			strconv.Quote(`q "test`),
			strconv.Quote(`tt\Ab`),
			strconv.Quote(`test]\`),
		), []SearchTerm{
			{Exact: true, Field: "", Value: "quoted"},
			{Exact: true, Field: "", Value: `q "test`},
			{Exact: true, Field: "", Value: `tt\Ab`},
			{Exact: true, Field: "", Value: `test]\`},
		}},
		{fmt.Sprintf("%s %s %s label:%s",
			strconv.Quote("quoted"),
			strconv.Quote(`q "test`),
			strconv.Quote(`tt\Ab`),
			strconv.Quote(`test" label]\`),
		), []SearchTerm{
			{Exact: true, Field: "", Value: "quoted"},
			{Exact: true, Field: "", Value: `q "test`},
			{Exact: true, Field: "", Value: `tt\Ab`},
			{Exact: true, Field: "label", Value: `test" label]\`},
		}},
		{`-test1 test2 -label:"name" label2:-name`, []SearchTerm{
			{Value: "test1", Exclude: true},
			{Value: "test2"},
			{Field: "label", Value: "name", Exact: true, Exclude: true},
			{Field: "label2", Value: "-name"},
		}},
		{`title:test other:"long string" bar:foo string`, []SearchTerm{
			{Exact: false, Field: "title", Value: "test"},
			{Exact: true, Field: "other", Value: `long string`},
			{Exact: false, Field: "bar", Value: "foo"},
			{Exact: false, Field: "", Value: "string"},
		}},
		{"", []SearchTerm{}},
		{`"`, []SearchTerm{
			{Exact: true, Value: ""},
		}},
		{"🦊 title:🐼", []SearchTerm{
			{Field: "", Value: "🦊"},
			{Field: "title", Value: "🐼"},
		}},
		{"label:test:image word", []SearchTerm{
			{Field: "label", Value: "test:image"},
			{Field: "", Value: "word"},
		}},
		{"label:test:*", []SearchTerm{
			{Field: "label", Value: "test:", Wildcard: true},
		}},
		{`"some * tesxt *"`, []SearchTerm{
			{Value: "some * tesxt *", Exact: true},
		}},
		{`"🧑‍🎓 learn"`, []SearchTerm{
			{Value: "🧑‍🎓 learn", Exact: true},
		}},
		{strconv.Quote("🧑‍🎓 learn"), []SearchTerm{
			{Value: "🧑‍🎓 learn", Exact: true},
		}},
		{`label:"🧑‍🎓 learn"`, []SearchTerm{
			{Field: "label", Value: "🧑‍🎓 learn", Exact: true},
		}},
		{`label:` + strconv.Quote("🧑‍🎓 learn"), []SearchTerm{
			{Field: "label", Value: "🧑‍🎓 learn", Exact: true},
		}},
		{"some*full* test:* not*hing", []SearchTerm{
			{Value: "some*full", Wildcard: true},
			{Field: "test", Wildcard: true},
			{Value: "not*hing"},
		}},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			q := ParseQuery(test.s)
			require.Equal(t, test.expected, q.Terms)
		})
	}
}

func TestParseField(t *testing.T) {
	fieldName := "some-field"

	tests := []struct {
		s        string
		expected []SearchTerm
	}{
		{"test:field abc", []SearchTerm{
			{Field: fieldName, Value: "test:field"},
			{Field: fieldName, Value: "abc"},
		}},
		{"test:field abc -foo:bar*", []SearchTerm{
			{Field: fieldName, Value: "test:field"},
			{Field: fieldName, Value: "abc"},
			{Field: fieldName, Value: "foo:bar", Exclude: true, Wildcard: true},
		}},
		{`"some test" match`, []SearchTerm{
			{Field: fieldName, Value: "some test", Exact: true},
			{Field: fieldName, Value: "match"},
		}},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			q := ParseField(test.s, fieldName)
			require.Equal(t, test.expected, q.Terms)
		})
	}
}

func TestUnfield(t *testing.T) {
	tests := []struct {
		q        SearchQuery
		expected SearchQuery
	}{
		{
			SearchQuery{Terms: []SearchTerm{
				{Field: "foo", Value: "bar"},
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--label--"},
				{Value: "some text", Exact: true},
			}},
			SearchQuery{Terms: []SearchTerm{
				{Value: "foo:bar"},
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--label--"},
				{Value: "some text", Exact: true},
			}},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			nq := test.q.Unfield("label", "title")
			require.Equal(t, test.expected, nq)
		})
	}
}

func TestDedup(t *testing.T) {
	tests := []struct {
		terms    []SearchTerm
		expected []SearchTerm
	}{
		{
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title--"},
				{Value: "some text"},
				{Field: "label", Value: "--label--"},
			},
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title--"},
				{Value: "some text"},
			},
		},
		{
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Value: "some text"},
				{Field: "title", Value: "--title--"},
				{Value: "some text", Exclude: true},
			},
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Value: "some text"},
				{Field: "title", Value: "--title--"},
				{Value: "some text", Exclude: true},
			},
		},
		{
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Value: "some text"},
				{Field: "title", Value: "--title--"},
				{Field: "label", Value: "--label--"},
				{Value: "some text"},
				{Value: "some text", Exclude: true},
			},
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Value: "some text"},
				{Field: "title", Value: "--title--"},
				{Value: "some text", Exclude: true},
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			q := SearchQuery{Terms: test.terms}
			require.Equal(t, test.expected, q.Dedup().Terms)
		})
	}
}

func TestPop(t *testing.T) {
	tests := []struct {
		name     string
		terms    []SearchTerm
		result   []SearchTerm
		newTerms []SearchTerm
	}{
		{
			"label",
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title--"},
				{Value: "some text"},
			},
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
			},
			[]SearchTerm{
				{Field: "title", Value: "--title--"},
				{Value: "some text"},
			},
		},
		{
			"",
			[]SearchTerm{
				{Value: "some text"},
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title--"},
				{Value: "more text"},
			},
			[]SearchTerm{
				{Value: "some text"},
				{Value: "more text"},
			},
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title--"},
			},
		},
		{
			"unknown",
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title--"},
				{Value: "some text"},
			},
			[]SearchTerm{},
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title--"},
				{Value: "some text"},
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			q := SearchQuery{Terms: test.terms}
			result, nq := q.PopField(test.name)
			require.Equal(t, test.result, result.Terms)
			require.Equal(t, test.newTerms, nq.Terms)
			require.Equal(t, test.terms, q.Terms)
		})
	}
}

func TestExtractField(t *testing.T) {
	tests := []struct {
		name   string
		terms  []SearchTerm
		expect []SearchTerm
	}{
		{
			"label",
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title--"},
				{Value: "some text"},
			},
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
			},
		},
		{
			"title",
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title 1--"},
				{Field: "title", Value: "--title 2--"},
				{Value: "some text"},
			},
			[]SearchTerm{
				{Field: "title", Value: "--title 1--"},
				{Field: "title", Value: "--title 2--"},
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			q := SearchQuery{Terms: test.terms}
			result := q.ExtractField(test.name)
			require.Equal(t, test.expect, result.Terms)
		})
	}
}

func TestRemoveFieldInfo(t *testing.T) {
	tests := []struct {
		name   string
		terms  []SearchTerm
		expect []SearchTerm
	}{
		{
			"label",
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title--"},
				{Value: "some text"},
			},
			[]SearchTerm{
				{Value: "--label--"},
				{Value: "--title--"},
				{Value: "some text"},
			},
		},
		{
			"title",
			[]SearchTerm{
				{Field: "label", Value: "--label--"},
				{Field: "title", Value: "--title 1--"},
				{Field: "title", Value: "--title 2--"},
				{Value: "some text"},
			},
			[]SearchTerm{
				{Value: "--label--"},
				{Value: "--title 1--"},
				{Value: "--title 2--"},
				{Value: "some text"},
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			q := SearchQuery{Terms: test.terms}
			result := q.RemoveFieldInfo()
			require.Equal(t, test.expect, result.Terms)
		})
	}
}

func TestTermToString(t *testing.T) {
	tests := []struct {
		term     SearchTerm
		expected string
	}{
		{
			SearchTerm{Value: "test"},
			"test",
		},
		{
			SearchTerm{Value: "test", Exclude: true},
			"-test",
		},
		{
			SearchTerm{Field: "label", Value: "test", Exclude: true, Wildcard: true},
			"-label:test*",
		},
		{
			SearchTerm{Value: `some "content`, Exact: true},
			`"some \"content"`,
		},
		{
			SearchTerm{Value: `some "content`, Exact: true, Exclude: true},
			`-"some \"content"`,
		},
		{
			SearchTerm{Value: `some "content`, Exact: true, Exclude: true, Wildcard: true},
			`-"some \"content"*`,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			require.Equal(t, test.expected, test.term.String())
		})
		t.Run(fmt.Sprintf("%d-reversed", i), func(t *testing.T) {
			q := ParseQuery(test.expected)
			require.Equal(t, []SearchTerm{test.term}, q.Terms)
		})
	}
}

func TestQueryToString(t *testing.T) {
	tests := []struct {
		terms    []SearchTerm
		expected string
	}{
		{[]SearchTerm{}, ""},
		{[]SearchTerm{{Exact: true}}, `""`},
		{[]SearchTerm{
			{Exact: false, Field: "", Value: "simple"},
			{Exact: false, Field: "", Value: "test"},
		}, "simple test"},
		{[]SearchTerm{
			{Exact: true, Field: "", Value: "quoted"},
			{Exact: true, Field: "", Value: `q "test`},
			{Exact: true, Field: "", Value: `tt\ab`},
		}, `"quoted" "q \"test" "tt\\ab"`},
		{[]SearchTerm{
			{Exact: true, Field: "", Value: "quoted"},
			{Exact: true, Field: "", Value: `q "test`},
			{Exact: true, Field: "", Value: `tt\ab`},
			{Exact: true, Field: "label", Value: `test" label]\`},
		}, `"quoted" "q \"test" "tt\\ab" label:"test\" label]\\"`},
		{[]SearchTerm{
			{Value: "test1", Exclude: true},
			{Value: "test2"},
			{Field: "label", Value: "name", Exact: true, Exclude: true},
			{Field: "label2", Value: "-name"},
		}, `-test1 test2 -label:"name" label2:-name`},
		{[]SearchTerm{
			{Exact: false, Field: "title", Value: "test"},
			{Exact: true, Field: "other", Value: `long string`},
			{Exact: false, Field: "bar", Value: "foo"},
			{Exact: false, Field: "", Value: "string"},
		}, `title:test other:"long string" bar:foo string`},
		{[]SearchTerm{
			{Field: "", Value: "🦊"},
			{Field: "title", Value: "🐼"},
		}, "🦊 title:🐼"},
		{[]SearchTerm{
			{Field: "label", Value: "test:image"},
			{Field: "", Value: "word"},
		}, "label:test:image word"},
		{[]SearchTerm{
			{Field: "label", Value: "test:", Wildcard: true},
		}, "label:test:*"},
		{[]SearchTerm{
			{Value: "some * tesxt *", Exact: true},
		}, `"some * tesxt *"`},
		{[]SearchTerm{
			{Value: "some*full", Wildcard: true},
			{Field: "test", Wildcard: true},
			{Value: "not*hing"},
		}, "some*full* test:* not*hing"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			q := SearchQuery{Terms: test.terms}
			require.Equal(t, test.expected, q.String())
		})
	}
}
