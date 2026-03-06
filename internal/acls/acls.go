// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package acls

import "iter"

// Load loads the default [Policy] with optionnal extra groups.
func Load(groups ...Group) {
	defaultPolicy = NewPolicy(defaultPermissions, append(defaultGroups, groups...)...)
}

// Clear empties the default policy (for tests).
func Clear() {
	defaultPolicy = &Policy{}
}

// Roles returns the default policy role list.
func Roles() iter.Seq[string] {
	return defaultPolicy.Keys()
}

// Enforce calls [Policy.Enforce] on the default policy.
func Enforce(sub, obj, act string) bool {
	return defaultPolicy.Enforce(sub, obj, act)
}

// GetPermissions calls [Policy.GetPermissions] on the default policy.
func GetPermissions(roles ...string) []string {
	return defaultPolicy.GetPermissions(roles...)
}

// ListGroups calls [Policy.ListGroups] on the default policy.
func ListGroups(parent string) []string {
	return defaultPolicy.ListGroups(parent)
}

// InGroup calls [Policy.InGroup] on the default policy.
func InGroup(src, dest string) bool {
	return defaultPolicy.InGroup(src, dest)
}

// DeletePermission calls [Policy.DeletePermission] on the default policy.
func DeletePermission(obj, act string) {
	defaultPolicy.DeletePermission(obj, act)
}
