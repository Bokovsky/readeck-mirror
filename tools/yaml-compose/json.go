// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type jsonEncoder struct {
	w io.Writer
}

func newJSONEncoder(w io.Writer) *jsonEncoder {
	return &jsonEncoder{w}
}

func (e *jsonEncoder) encodeScalar(node *yaml.Node) error {
	var v any
	if err := node.Decode(&v); err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := e.w.Write(bytes.TrimRight(buf.Bytes(), "\n"))
	return err
}

func (e *jsonEncoder) encodeSequence(node *yaml.Node, indent string) error {
	for i, n := range node.Content {
		if _, err := e.w.Write([]byte(indent)); err != nil {
			return err
		}

		if err := e.encodeNode(n, indent); err != nil {
			return err
		}

		if i+1 < len(node.Content) {
			if _, err := e.w.Write([]byte(",\n")); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *jsonEncoder) encodeMapping(node *yaml.Node, indent string) error {
	if len(node.Content) < 2 {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		k, err := json.Marshal(key.Value)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(e.w, "%s%s: ", indent, string(k)); err != nil {
			return err
		}

		if err := e.encodeNode(val, indent); err != nil {
			return err
		}
		if i+2 < len(node.Content) {
			if _, err := e.w.Write([]byte(",\n")); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *jsonEncoder) encodeNode(node *yaml.Node, indent string) error {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, n := range node.Content {
			if err := e.encodeNode(n, indent); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			_, err := e.w.Write([]byte("[]"))
			return err
		}

		if _, err := e.w.Write([]byte("[\n")); err != nil {
			return err
		}
		if err := e.encodeSequence(node, indent+"  "); err != nil {
			return err
		}
		if _, err := e.w.Write([]byte("\n" + indent + "]")); err != nil {
			return err
		}
	case yaml.MappingNode:
		if _, err := e.w.Write([]byte("{\n")); err != nil {
			return err
		}
		if err := e.encodeMapping(node, indent+"  "); err != nil {
			return err
		}
		if _, err := e.w.Write([]byte("\n" + indent + "}")); err != nil {
			return err
		}
	case yaml.ScalarNode:
		if err := e.encodeScalar(node); err != nil {
			return err
		}
	}

	return nil
}

func (e *jsonEncoder) encode(node *yaml.Node) error {
	return e.encodeNode(node, "")
}
