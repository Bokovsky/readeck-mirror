// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var rxHTMLComment = regexp.MustCompile("(?ms)(<!--.*?(-->\n?))")

type documentCollection struct {
	root  *os.Root
	main  string
	files map[string]*yaml.Node
}

func newCollection(filename string) (*documentCollection, error) {
	dirname := filepath.Dir(filename)
	root, err := os.OpenRoot(dirname)
	if err != nil {
		return nil, err
	}

	col := &documentCollection{
		root:  root,
		main:  filepath.Base(filename),
		files: map[string]*yaml.Node{},
	}

	// First, load all the files
	if err := fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if slices.Contains([]string{".yaml", ".yml"}, filepath.Ext(p)) {
			fd, err := root.Open(p)
			if err != nil {
				return err
			}
			defer fd.Close() // nolint:errcheck
			doc := new(yaml.Node)
			dec := yaml.NewDecoder(fd)
			if err := dec.Decode(doc); err != nil {
				return err
			}
			col.files[p] = doc
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// Resolve includes
	for p, doc := range col.files {
		if err := col.resolveInclude(p, doc); err != nil {
			return nil, err
		}
	}

	// Resolve all the references
	for p, doc := range col.files {
		if err := col.resolveMerges(p, doc); err != nil {
			return nil, err
		}
	}

	return col, nil
}

func (col *documentCollection) resolveInclude(filename string, node *yaml.Node) error {
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, n := range node.Content {
			if err := col.resolveInclude(filename, n); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		if len(node.Content) == 2 && node.Content[0].Value == "$include" {
			content, err := col.readIncluded(node.Content[1])
			if err != nil {
				return err
			}

			content = rxHTMLComment.ReplaceAllString(content, "")

			// Replace the node for a string with the file's content
			node.Kind = yaml.ScalarNode
			node.Tag = "!!str"
			node.Value = content

			return nil
		}

		for i := 0; i < len(node.Content); i += 2 {
			if err := col.resolveInclude(filename, node.Content[i+1]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (col *documentCollection) readIncluded(node *yaml.Node) (string, error) {
	files := []string{}
	switch node.Kind {
	case yaml.SequenceNode:
		for _, n := range node.Content {
			files = append(files, n.Value)
		}
	case yaml.ScalarNode:
		files = []string{node.Value}
	}

	buf := new(bytes.Buffer)

	for _, filename := range files {
		if err := func() error {
			fd, err := col.root.Open(filename)
			if err != nil {
				return err
			}
			defer fd.Close() // nolint:errcheck
			if _, err = io.Copy(buf, fd); err != nil {
				return err
			}
			buf.WriteString("\n\n")
			return nil
		}(); err != nil {
			return "", err
		}
	}

	return buf.String(), nil
}

func (col *documentCollection) resolveMerges(filename string, node *yaml.Node) error {
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, n := range node.Content {
			if err := col.resolveMerges(filename, n); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		nodeMap := newMap(nil)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			n := node.Content[i+1]
			if err := col.resolveMerges(filename, n); err != nil {
				return err
			}

			if key.Value != "$merge" {
				// Each regular node updates the map
				nodeMap.update(newMap(&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{node.Content[i], node.Content[i+1]}}))
				continue
			}

			// $merge nodes are merged to the map
			nodes, err := col.resolve(filename, n)
			if err != nil {
				return err
			}

			for _, rMap := range nodes {
				nodeMap.update(rMap)
			}
		}

		// set the node content from the final map
		node.Content = nodeMap.contentNodes()
	}

	return nil
}

func (col *documentCollection) resolve(currentFile string, node *yaml.Node) ([]*yamlMap, error) {
	uris := []string{}
	switch node.Kind {
	case yaml.SequenceNode:
		for _, n := range node.Content {
			if n.ShortTag() != "!!str" {
				return nil, errors.New("merge URI can only be a string")
			}
			uris = append(uris, n.Value)
		}
	case yaml.ScalarNode:
		if node.ShortTag() != "!!str" {
			return nil, errors.New("merge URI can only be a string")
		}
		uris = []string{node.Value}
	default:
		return nil, errors.New("merge value must be a list or string")
	}

	res := []*yamlMap{}
	for _, uri := range uris {
		filename, selector, _ := strings.Cut(uri, "#")
		if selector == "" {
			return nil, errors.New("no selector")
		}
		if filename == "" {
			filename = currentFile
		}
		doc, ok := col.files[filename]
		if !ok {
			return nil, fmt.Errorf(`file "%s" not found`, filename)
		}
		selector = strings.TrimPrefix(selector, ".")

		// Find the node
		n := col.selectNode(doc, selector)
		if n == nil {
			return nil, fmt.Errorf(`node "%s#%s" not found`, filename, selector)
		}
		if n.Kind != yaml.MappingNode {
			return nil, fmt.Errorf(`node "%s#%s" is not a map`, filename, selector)
		}

		res = append(res, newMap(n))
	}

	return res, nil
}

func (col *documentCollection) selectNode(node *yaml.Node, selector string) *yaml.Node {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, n := range node.Content {
			if x := col.selectNode(n, selector); x != nil {
				return x
			}
		}
	case yaml.MappingNode:
		name, rest, _ := strings.Cut(selector, ".")
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == name {
				if rest == "" {
					return node.Content[i+1]
				}
				return col.selectNode(node.Content[i+1], rest)
			}
		}
	}

	return nil
}
