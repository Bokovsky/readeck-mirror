// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package acls defines a simple wrapper on top of casbin functions.
package acls

import (
	"bufio"
	"embed"
	"errors"
	"path"
	"sort"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	defaultrolemanager "github.com/casbin/casbin/v2/rbac/default-role-manager"
)

//go:embed config/*
var confFiles embed.FS

var enforcer *casbin.Enforcer

// Check performs the rule enforcment for a given user, path and action.
func Check(group, path, act string) (bool, error) {
	return enforcer.Enforce(group, path, act)
}

// GetPermissions returns the permissions for a list of groups.
func GetPermissions(groups ...string) ([]string, error) {
	perms := map[string]struct{}{}

	for _, group := range groups {
		plist, err := enforcer.GetImplicitPermissionsForUser(group)
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

// InGroup returns true if permissions from "src" group are all in "dest" group.
func InGroup(src, dest string) bool {
	srcPermissions, _ := enforcer.GetImplicitPermissionsForUser(src)
	dstPermissions, _ := enforcer.GetImplicitPermissionsForUser(dest)

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

// DeleteRole deletes a role. Returns false if a role does not exist.
func DeleteRole(name string) (bool, error) {
	return enforcer.DeleteRole(name)
}

func init() {
	var err error
	enforcer, err = newEnforcer()
	if err != nil {
		panic(err)
	}
}

func newEnforcer() (*casbin.Enforcer, error) {
	c, err := confFiles.ReadFile("config/model.ini")
	if err != nil {
		return nil, err
	}
	m, err := model.NewModelFromString(string(c))
	if err != nil {
		return nil, err
	}

	policy, err := confFiles.ReadFile("config/policy.conf")
	if err != nil {
		return nil, err
	}
	sa := newAdapter(string(policy))
	e, _ := casbin.NewEnforcer()
	err = e.InitWithModelAndAdapter(m, sa)
	if err != nil {
		return nil, err
	}

	rm := e.GetRoleManager()
	rm.(*defaultrolemanager.RoleManagerImpl).AddMatchingFunc("g", globMatch)

	return e, err
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

	scanner := bufio.NewScanner(strings.NewReader(sa.contents))
	for scanner.Scan() {
		str := scanner.Text()
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
