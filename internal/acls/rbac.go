// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package acls

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
)

// Permission is a triplet of a path, object and action.
type Permission struct {
	Path   string
	Object string
	Action string
}

func newPermission(p, obj, act string) *Permission {
	return &Permission{Path: p, Object: obj, Action: act}
}

// Role is a group of permissions that inherits from its parents.
type Role struct {
	Name        string
	Permissions map[string]struct{}
	Parents     map[string]*Role
}

func newRole(name string) *Role {
	return &Role{
		Name:        name,
		Permissions: map[string]struct{}{},
		Parents:     map[string]*Role{},
	}
}

func (r Role) addPermission(p *Permission) {
	r.Permissions[p.Object+":"+p.Action] = struct{}{}
}

// HasPermission returns true when obj and act are present in the
// role's permission list or in one of its parents.
func (r Role) HasPermission(obj, act string) bool {
	if _, ok := r.Permissions[obj+":"+act]; ok {
		return true
	}
	for _, p := range r.Parents {
		if p.HasPermission(obj, act) {
			return true
		}
	}

	return false
}

// ListPermissions returns all the role's permissions.
func (r Role) ListPermissions() []string {
	perms := []string{}
	for k := range r.Permissions {
		perms = append(perms, k)
	}

	for _, p := range r.Parents {
		perms = append(perms, p.ListPermissions()...)
	}

	slices.Sort(perms)
	perms = slices.Compact(perms)

	return perms
}

// Policy is a list of [Role]s.
type Policy map[string]*Role

// Enforce returns true when a subject (the role's name)
// contains obj and act permission.
func (p Policy) Enforce(sub, obj, act string) bool {
	if r, ok := p[sub]; ok {
		return r.HasPermission(obj, act)
	}
	return false
}

// GetPermissions returns the given permissions of roles, including
// permissions inherited from their parent roles.
func (p Policy) GetPermissions(roles ...string) []string {
	perms := []string{}
	for _, r := range roles {
		if role, ok := p[r]; ok {
			perms = append(perms, role.ListPermissions()...)
		}
	}

	slices.Sort(perms)
	perms = slices.Compact(perms)
	return perms
}

// ListGroups returns the groups with "parent" as a direct parent group.
func (p Policy) ListGroups(parent string) []string {
	res := []string{}
	for name, r := range p {
		if _, ok := r.Parents[parent]; ok {
			res = append(res, name)
		}
	}

	slices.Sort(res)
	return slices.Compact(res)
}

// InGroup returns true if permissions from src group are all in dest group.
func (p Policy) InGroup(src, dest string) bool {
	srcPermissions := p.GetPermissions(src)
	dstPermissions := p.GetPermissions(dest)

	dmap := map[string]struct{}{}
	for _, x := range dstPermissions {
		dmap[x] = struct{}{}
	}

	i := 0
	for _, x := range srcPermissions {
		if _, ok := dmap[x]; !ok {
			return false
		}
		i++
	}

	return i > 0
}

// DeletePermission remove a permission from the policy.
func (p Policy) DeletePermission(obj, act string) {
	for _, r := range p {
		delete(r.Permissions, obj+":"+act)
	}
}

// LoadPolicy loads a policy from an [io.Reader].
func LoadPolicy(r io.Reader) (Policy, error) {
	br := bufio.NewReader(r)
	permissions := []*Permission{}
	groups := [][2]string{}

	for {
		b, err := br.ReadSlice('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		line := strings.TrimSpace(string(b))
		if line == "" || line[0] == '#' {
			continue
		}

		fields := []string{}
		for f := range strings.SplitSeq(line, ",") {
			fields = append(fields, strings.TrimSpace(f))
		}

		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "p":
			if len(fields) != 4 {
				return nil, fmt.Errorf("invalid permission definition: %s", line)
			}
			permissions = append(permissions, newPermission(fields[1], fields[2], fields[3]))
		case "g":
			if len(fields) != 3 {
				return nil, fmt.Errorf("invalid group definition: %s", line)
			}
			groups = append(groups, [2]string(fields[1:3]))
		}

	}

	policy := Policy{}

	// First, create all the roles
	for _, g := range groups {
		if _, ok := policy[g[0]]; !ok {
			policy[g[0]] = newRole(g[0])
		}

		if !strings.HasPrefix(g[1], "/") {
			policy[g[1]] = newRole(g[0])
		}
	}

	// Add permissions and parents to roles
	for _, g := range groups {
		role := g[0]
		subj := g[1]
		if strings.HasPrefix(subj, "/") {
			for _, p := range permissions {
				if ok, _ := path.Match(subj, p.Path); ok {
					policy[role].addPermission(p)
				}
			}
		} else {
			policy[role].Parents[subj] = policy[subj]
		}
	}

	return policy, nil
}
