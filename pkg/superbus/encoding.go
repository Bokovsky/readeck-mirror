// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package superbus

import "github.com/fxamacker/cbor/v2"

// Marshal returns the encoded v value.
func Marshal(v any) ([]byte, error) {
	return cbor.Marshal(v)
}

// Unmarshal encodes data into v.
func Unmarshal(data []byte, v any) error {
	return cbor.Unmarshal(data, v)
}
