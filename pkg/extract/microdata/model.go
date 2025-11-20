// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package microdata

import (
	"iter"
	"maps"
	"slices"
)

// Microdata contains a list of [Node] and provides query methods.
type Microdata struct {
	Nodes []*Node
}

// NodeType is a node type.
type NodeType uint8

const (
	// Item is a node with children.
	Item NodeType = iota
	// Property is a node with scalar data.
	Property
)

// Node is an element of the node hierarchy.
type Node struct {
	Type     NodeType `json:"type"`
	Name     string   `json:"name,omitempty"`
	Path     string   `json:"path"`
	Data     any      `json:"data,omitempty"`
	Parent   *Node    `json:"-"`
	Children []*Node  `json:"children,omitempty"`

	raw map[string]any
}

// load loads a given node and insert its children, if any, in the hierarchy.
func (node *Node) load(val any) {
	switch t := val.(type) {
	case map[string]any:
		// top level @graph restarts loading with its own content
		if g, ok := t["@graph"]; ok && node.Parent == nil {
			node.load(g)
			break
		}

		// item node type
		if typ, ok := t["@type"].(string); ok && node.Path == "" {
			// Store the type as root name
			node.Path = typ
		}

		for _, k := range slices.Sorted(maps.Keys(t)) {
			n := &Node{
				Name:   k,
				Path:   node.Path + "." + k,
				Parent: node,
			}
			n.load(t[k])

			node.Children = append(node.Children, n)
		}
	case []any:
		// list of nodes
		for _, x := range t {
			n := &Node{Parent: node, Path: node.Path}
			n.load(x)
			node.Children = append(node.Children, n)
		}
	case float64:
		// number node. We come from JSON so every number is a float64
		// for convenience, we convert them to int when possible
		node.Type = Property
		node.Data = t
		if t == float64(int(t)) {
			node.Data = int(t)
		}
	default:
		// property node
		node.Type = Property
		node.Data = t
	}
}

func (node *Node) every(f func(*Node) bool, filter func(*Node) bool) bool {
	if filter != nil && filter(node) {
		if !f(node) {
			return false
		}
	}
	return node.everyChild(f, filter)
}

func (node *Node) everyChild(f func(*Node) bool, filter func(*Node) bool) bool {
	for _, c := range node.Children {
		if !c.every(f, filter) {
			return false
		}
	}
	return true
}

// Raw returns the initial JSON-LD values.
func (md *Microdata) Raw() []any {
	res := []any{}
	for _, x := range md.Nodes {
		res = append(res, x.raw)
	}
	return res
}

// All returns a recursive iterator over all nodes with a filter function (can be nil).
func (md *Microdata) All(filter func(*Node) bool) iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		for _, node := range md.Nodes {
			if !node.every(yield, filter) {
				break
			}
		}
	}
}
