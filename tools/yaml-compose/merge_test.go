// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var baseDoc = `
---

key1:
  $merge: test/doc1.yaml#deep.selector
  $merge: test/doc2.yaml#other.selector
`

func TestMerge(t *testing.T) {
	doc := new(yaml.Node)
	dec := yaml.NewDecoder(strings.NewReader(baseDoc))
	err := dec.Decode(doc)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf(">>> %#v\n", doc.Content[0])
}
