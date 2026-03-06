// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package acls

import (
	"iter"
	"slices"
	"sync"
)

// Set is a very simple set implented using [sync.Map] and a string list.
type Set struct {
	m     sync.Map
	items []string
}

// Add adds one or more items to a set. Note that it's more efficient
// to add many items at once.
func (s *Set) Add(values ...string) {
	for _, v := range values {
		s.m.LoadOrStore(v, struct{}{})
	}
	s.items = slices.Sorted(s.all())
}

// Replace clears the set and adds the new values.
func (s *Set) Replace(values ...string) {
	s.m.Clear()
	s.Add(values...)
}

// Del removes one or more values from the set.
func (s *Set) Del(values ...string) {
	for _, v := range values {
		s.m.Delete(v)
	}
	s.items = slices.Sorted(s.all())
}

// Contains returns true if the set contains a value.
func (s *Set) Contains(value string) (ok bool) {
	_, ok = s.m.Load(value)
	return ok
}

func (s *Set) all() iter.Seq[string] {
	return func(yield func(string) bool) {
		s.m.Range(func(key, _ any) bool {
			return yield(key.(string))
		})
	}
}

// Items returns the sorted list of items in the set.
func (s *Set) Items() []string {
	return s.items
}
