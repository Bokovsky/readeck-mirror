// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package acls defines a simple wrapper on top of casbin functions.
package acls

import (
	"bytes"
	"embed"
	"errors"
	"io"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	defaultrolemanager "github.com/casbin/casbin/v2/rbac/default-role-manager"
)

//go:embed config/*
var confFiles embed.FS

var enforcer *Enforcer

// Enforcer is an [casbin.Enforcer] with some extensions.
type Enforcer struct {
	*casbin.Enforcer
}

// NewEnforcer creates an new [Enforcer] instance concatening
// all the provided [io.Reader] as a policy.
func NewEnforcer(r io.Reader) (*Enforcer, error) {
	c, err := confFiles.ReadFile("config/model.ini")
	if err != nil {
		return nil, err
	}
	m, err := model.NewModelFromString(string(c))
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, r); err != nil {
		return nil, err
	}

	sa := newAdapter(buf.String())
	e, _ := casbin.NewEnforcer()
	err = e.InitWithModelAndAdapter(m, sa)
	if err != nil {
		return nil, err
	}

	rm := e.GetRoleManager()
	rm.(*defaultrolemanager.RoleManagerImpl).AddMatchingFunc("g", globMatch)

	return &Enforcer{e}, err
}

// GetPermissions returns the permissions for a list of groups.
func (e *Enforcer) GetPermissions(groups ...string) ([]string, error) {
	perms := map[string]struct{}{}

	for _, group := range groups {
		plist, err := e.GetImplicitPermissionsForUser(group)
		if err != nil {
			return []string{}, err
		}
		for _, p := range plist {
			perms[p[1]+":"+p[2]] = struct{}{}
		}
	}

	res := []string{}
	for k := range perms {
		res = append(res, k)
	}
	sort.Strings(res)
	return res, nil
}

// ListGroups returns the groups that explicitly belong to a parent group.
func (e *Enforcer) ListGroups(parent string) (res []string, err error) {
	roles, err := e.GetGroupingPolicy()
	if err != nil {
		return nil, err
	}

	res = []string{}
	for _, r := range roles {
		if r[1] == parent && !slices.Contains(res, parent) {
			res = append(res, r[0])
		}
	}

	return
}

// InGroup returns true if permissions from "src" group are all in "dest" group.
func (e *Enforcer) InGroup(src, dest string) bool {
	srcPermissions, _ := e.GetImplicitPermissionsForUser(src)
	dstPermissions, _ := e.GetImplicitPermissionsForUser(dest)

	dmap := map[string]struct{}{}
	for _, x := range dstPermissions {
		dmap[x[0]] = struct{}{}
	}

	i := 0
	for _, x := range srcPermissions {
		if _, ok := dmap[x[0]]; !ok {
			return false
		}
		i++
	}

	return i > 0
}

// Load loads the default [Enforcer] using the assets.
func Load(policies ...io.Reader) (err error) {
	fd, err := confFiles.Open("config/policy.conf")
	if err != nil {
		return err
	}
	defer fd.Close() //nolint:errcheck

	enforcer, err = NewEnforcer(io.MultiReader(append([]io.Reader{fd}, policies...)...))
	return err
}

// Enforce calls [Enforcer.Enforce] on the default enforcer.
func Enforce(group, path, act string) (bool, error) {
	return enforcer.Enforce(group, path, act)
}

// GetPermissions calls [Enforcer.GetPermissions] on the default enforcer.
func GetPermissions(groups ...string) ([]string, error) {
	return enforcer.GetPermissions(groups...)
}

// ListGroups calls [Enforcer.ListGroups] on the default enforcer.
func ListGroups(parent string) ([]string, error) {
	return enforcer.ListGroups(parent)
}

// InGroup calls [Enforcer.InGroup] on the default enforcer.
func InGroup(src, dest string) bool {
	return enforcer.InGroup(src, dest)
}

// DeleteRole deletes a role. Returns false if a role does not exist.
func DeleteRole(name string) (bool, error) {
	return enforcer.DeleteRole(name)
}

// globMatch is our own casbin matcher function. It only matches
// path like patterns. It's enough since that's how we define policy subjects
// and it's way faster than KeyMatch2 that compiles regexp on each test.
func globMatch(key1, key2 string) (ok bool) {
	ok, _ = path.Match(key2, key1)
	return
}

type adapter struct {
	contents string
}

func newAdapter(contents string) *adapter {
	return &adapter{
		contents: contents,
	}
}

func (sa *adapter) LoadPolicy(model model.Model) error {
	if sa.contents == "" {
		return errors.New("invalid line, line cannot be empty")
	}
	for str := range strings.SplitSeq(sa.contents, "\n") {
		if str == "" {
			continue
		}
		if err := persist.LoadPolicyLine(str, model); err != nil {
			return err
		}
	}

	return nil
}

func (sa *adapter) SavePolicy(_ model.Model) error {
	return errors.New("not implemented")
}

func (sa *adapter) AddPolicy(_ string, _ string, _ []string) error {
	return errors.New("not implemented")
}

func (sa *adapter) RemovePolicy(_ string, _ string, _ []string) error {
	return errors.New("not implemented")
}

func (sa *adapter) RemoveFilteredPolicy(_ string, _ string, _ int, _ ...string) error {
	return errors.New("not implemented")
}
