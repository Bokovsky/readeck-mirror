// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package acls provides the RBAC policy for Readeck.
package acls

import (
	"iter"
	"path"
	"slices"
	"strings"
	"sync"
)

// Permissions is a mapping of path to a pair of object and action.
type Permissions map[string][2]string

// Group is the representation of a role from configuration.
type Group struct {
	Name    string   `json:"name"`
	Parents []string `json:"parents"`
	Grants  []string `json:"grants"`
}

// Role is the resolved role contained in [Policy].
type Role struct {
	permissions Set
	groups      Set
}

// Policy is a map of [Role]s.
type Policy struct {
	m sync.Map
}

// zeroRole is returned when calling [Policy.Get] with a non existing role.
var zeroRole = new(Role)

// resolvePermissions recursively resolves all permissions of a role and
// returns them as a list.
func (policy *Policy) resolvePermissions(name string) []string {
	res := &Set{}
	role := policy.Get(name)
	for _, g := range role.groups.Items() {
		res.Add(policy.resolvePermissions(g)...)
	}
	res.Add(role.permissions.Items()...)

	for _, v := range res.Items() {
		if x, ok := strings.CutPrefix(v, "!"); ok {
			res.Del(v, x)
		}
	}

	return res.Items()
}

// Set add a new [Role] to the policy.
func (policy *Policy) Set(key string, role *Role) {
	policy.m.Store(key, role)
}

// Get returns a [Role] for the given name. If the name does not exist,
// an empty role is returned so you can still perform chained queries.
func (policy *Policy) Get(key string) *Role {
	if r, ok := policy.m.Load(key); ok {
		return r.(*Role)
	}
	return zeroRole
}

// Keys returns all the policy's role names.
func (policy *Policy) Keys() iter.Seq[string] {
	return func(yield func(string) bool) {
		policy.m.Range(func(key, _ any) bool {
			return yield(key.(string))
		})
	}
}

// Enforce returns true when a subject (the role's name)
// contains obj and act permission.
func (policy *Policy) Enforce(sub, obj, act string) bool {
	return policy.Get(sub).Contains(obj, act)
}

// GetPermissions returns all the permissions granted to one or several roles.
func (policy *Policy) GetPermissions(roles ...string) (res []string) {
	switch len(roles) {
	case 0:
		return res
	case 1:
		// direct shortcut, no allocation
		return policy.Get(roles[0]).permissions.Items()
	default:
		for _, r := range roles {
			res = append(res, policy.Get(r).permissions.Items()...)
		}

		slices.Sort(res)
		return slices.Compact(res)
	}
}

// ListGroups returns the groups with "parent" as a direct parent group.
func (policy *Policy) ListGroups(parent string) []string {
	res := []string{}
	for name := range policy.Keys() {
		if policy.Get(name).groups.Contains(parent) {
			res = append(res, name)
		}
	}

	slices.Sort(res)
	return slices.Compact(res)
}

// InGroup returns true if permissions from src group are all in dest group.
func (policy *Policy) InGroup(src, dest string) bool {
	return slices.Compare(policy.GetPermissions(src), policy.GetPermissions(dest)) >= 0
}

// DeletePermission remove a permission from the policy.
func (policy *Policy) DeletePermission(obj, act string) {
	for name := range policy.Keys() {
		policy.Get(name).Delete(obj, act)
	}
}

// NewPolicy returns a new [Policy] from [Permissions] and a list of [Group].
func NewPolicy(permissions Permissions, groups ...Group) *Policy {
	policy := &Policy{}
	for _, g := range groups {
		// add initial permissions
		plist := make([]string, 0, len(permissions))

		for _, grant := range g.Grants {
			negate := false
			prefix := ""
			grant, negate = strings.CutPrefix(grant, "!")
			if negate {
				prefix = "!"
			}

			for p, v := range permissions {
				if ok, _ := path.Match(grant, p); ok {
					plist = append(plist, prefix+v[0]+":"+v[1])
				}
			}
		}

		role := &Role{}
		role.groups.Add(g.Parents...)
		role.permissions.Add(plist...)
		policy.Set(g.Name, role)
	}

	// Resolve all permissions
	for name := range policy.Keys() {
		policy.Get(name).permissions.Replace(policy.resolvePermissions(name)...)
	}

	return policy
}

// Contains returns true if obj:act exists in the role.
func (role *Role) Contains(obj, act string) bool {
	return role.permissions.Contains(obj + ":" + act)
}

// Delete removes obj:act from the role.
func (role *Role) Delete(obj, act string) {
	role.permissions.Del(obj + ":" + act)
}
