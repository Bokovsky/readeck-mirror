// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"iter"
	"slices"

	"gopkg.in/yaml.v3"
)

type yamlMap struct {
	nodes map[string]*yaml.Node
	keys  []string
}

func newMap(node *yaml.Node) *yamlMap {
	if node == nil {
		node = &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{}}
	}

	if node.Kind != yaml.MappingNode {
		panic("not a map")
	}

	res := &yamlMap{
		nodes: map[string]*yaml.Node{},
		keys:  []string{},
	}

	for i := 0; i < len(node.Content); i += 2 {
		res.nodes[node.Content[i].Value] = node.Content[i+1]
		res.keys = append(res.keys, node.Content[i].Value)
	}

	return res
}

func (m *yamlMap) get(name string) *yaml.Node {
	return m.nodes[name]
}

func (m *yamlMap) set(name string, node *yaml.Node) {
	m.nodes[name] = node
	if !slices.Contains(m.keys, name) {
		m.keys = append(m.keys, name)
	}
}

func (m *yamlMap) items() iter.Seq2[string, *yaml.Node] {
	return func(yield func(string, *yaml.Node) bool) {
		for _, key := range m.keys {
			if !yield(key, m.nodes[key]) {
				return
			}
		}
	}
}

func (m *yamlMap) contentNodes() []*yaml.Node {
	res := []*yaml.Node{}
	for k, v := range m.items() {
		res = append(res,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			v,
		)
	}
	return res
}

func (m *yamlMap) update(x *yamlMap) {
	for k, v := range x.items() {
		if m.get(k) == nil {
			m.set(k, v)
			continue
		}

		switch v.Kind {
		case yaml.MappingNode:
			if m.get(k).Kind != yaml.MappingNode {
				// Destination is not a map, replace with the new value
				m.set(k, v)
				continue
			}

			// (shallow) copy the original node
			vCopy := new(yaml.Node)
			*vCopy = *v

			// Get the map and update it with the new one
			kMap := newMap(m.get(k))
			kMap.update(newMap(vCopy))

			// Then set the updated values on the initial copy
			// and set it on our key.
			vCopy.Content = kMap.contentNodes()
			m.set(k, vCopy)
		case yaml.SequenceNode:
			if m.get(k).Kind != yaml.SequenceNode {
				m.set(k, v)
				continue
			}

			vCopy := new(yaml.Node)
			*vCopy = *m.get(k)
			vCopy.Content = append(vCopy.Content, v.Content...)
			m.set(k, vCopy)
		default:
			// Not a map, replace with new value
			m.set(k, v)
		}
	}
}
