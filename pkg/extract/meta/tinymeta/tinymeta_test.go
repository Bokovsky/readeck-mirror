// SPDX-FileCopyrightText: © 2026 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package tinymeta_test

import (
	"strings"
	"testing"

	"codeberg.org/readeck/readeck/pkg/extract/meta/tinymeta"
)

func TestScan(t *testing.T) {
	r := strings.NewReader(`
		<!DOCTYPE html>
		<meta charset="utf-8"/>
		<base href="/path">
		<title>Demystifying the <base> element</title>
		<script>document.write("hello!")</script>
		<!-- a comment -->
		<link rel=stylesheet href="/style.css">
		<meta content="Hi &larr; social media" property="twitter:title">
		<style>body { font-size: 16px }</style>
		<meta name="og:description" content="Some &lt;description&gt; here"/>
		<p>
			This is where the body starts.
			<meta name="ignored" content="does not matter">
		</p>
	`)

	var got [][2]string
	for key, value := range tinymeta.Scan(r) {
		got = append(got, [2]string{key, value})
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %#v", got)
	}

	assert := func(idx int, expectKey, expectValue string) {
		if gotKey := got[idx][0]; gotKey != expectKey {
			t.Errorf("expected %q, got %q", expectKey, gotKey)
		}
		if gotValue := got[idx][1]; gotValue != expectValue {
			t.Errorf("expected %q, got %q", expectValue, gotValue)
		}
	}

	assert(0, "title", "Demystifying the <base> element")
	assert(1, "twitter:title", "Hi ← social media")
	assert(2, "og:description", "Some <description> here")
}
