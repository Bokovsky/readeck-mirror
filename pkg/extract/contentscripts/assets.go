// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"embed"
	"io/fs"
	"path"
)

//go:embed assets assets/**/*
var assets embed.FS

var (
	// SiteConfigFiles is the default site-config files discovery.
	SiteConfigFiles *SiteConfigDiscovery

	// ContentScriptFS is an sub [fs.FS] targeted at content scripts.
	ContentScriptFS fs.FS

	siteConfigFS     fs.FS
	preloadedScripts []*Program
)

func init() {
	var err error

	if ContentScriptFS, err = fs.Sub(assets, "assets/scripts"); err != nil {
		panic(err)
	}

	if siteConfigFS, err = fs.Sub(assets, "assets/site-config"); err != nil {
		panic(err)
	}
	SiteConfigFiles = NewSiteconfigDiscovery(siteConfigFS)

	// Preload scripts
	scripts, err := fs.Glob(ContentScriptFS, "*.js")
	if err != nil {
		panic(err)
	}

	for _, x := range scripts {
		r, err := ContentScriptFS.Open(x)
		if err != nil {
			panic(err)
		}

		p, err := NewProgram(path.Join("builtin", path.Base(x)), r)
		if err != nil {
			panic(err)
		}
		preloadedScripts = append(preloadedScripts, p)
	}
}
