// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bus

import (
	"time"

	"codeberg.org/readeck/readeck/pkg/superbus"
)

// Set stores a value as a JSON string.
func Set(key string, value any, expiration time.Duration) error {
	data, err := superbus.Marshal(value)
	if err != nil {
		return err
	}
	return store.Set(key, data, expiration)
}

// Get retrieves a value as a JSON string. It returns [ErrNotExists]
// when the value was not in the store already.
func Get(key string, value any) error {
	data := store.Get(key)
	if data == nil {
		return nil
	}

	return superbus.Unmarshal(data, value)
}
