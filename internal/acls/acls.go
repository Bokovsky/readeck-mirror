// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package acls provides the RBAC policy for Readeck.
package acls

import (
	"embed"
	"io"
)

//go:embed config/*
var confFiles embed.FS

var policy Policy

// Load loads the default [Policy] using the assets.
func Load(policies ...io.Reader) (err error) {
	fd, err := confFiles.Open("config/policy.conf")
	if err != nil {
		return err
	}
	defer fd.Close() // nolint:errcheck

	policy, err = LoadPolicy(io.MultiReader(append([]io.Reader{fd}, policies...)...))
	return err
}

// Enforce calls [Policy.Enforce] on the default policy.
func Enforce(sub, obj, act string) bool {
	return policy.Enforce(sub, obj, act)
}

// GetPermissions calls [Policy.GetPermissions] on the default policy.
func GetPermissions(roles ...string) []string {
	return policy.GetPermissions(roles...)
}

// ListGroups calls [Policy.ListGroups] on the default policy.
func ListGroups(parent string) []string {
	return policy.ListGroups(parent)
}

// InGroup calls [Policy.InGroup] on the default policy.
func InGroup(src, dest string) bool {
	return policy.InGroup(src, dest)
}

// DeletePermission calls [Policy.DeletePermission] on the default policy.
func DeletePermission(obj, act string) {
	policy.DeletePermission(obj, act)
}
