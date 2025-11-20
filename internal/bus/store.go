// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bus

import (
	"encoding/json"
	"time"
)

// SetJSON stores a value as a JSON string.
func SetJSON(key string, value any, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return store.Set(key, string(data), expiration)
}

// GetJSON retrieves a value as a JSON string. It returns [ErrNotExists]
// when the value was not in the store already.
func GetJSON(key string, value any) error {
	data := store.Get(key)
	if data == "" {
		return nil
	}

	return json.Unmarshal([]byte(data), value)
}
