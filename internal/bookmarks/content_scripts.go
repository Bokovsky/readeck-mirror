// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/extract/contentscripts"
)

type contentScriptRegistry []*contentscripts.Program

func (r contentScriptRegistry) get(name string) *contentscripts.Program {
	for _, p := range r {
		if p.Name == name {
			return p
		}
	}
	return nil
}

var contentScripts = contentScriptRegistry{}

// LoadContentScripts finds the user content scripts, compile them and keep
// a cache of the result.
// When a script changes, it's reloaded and replaces the previous cache entry.
func LoadContentScripts(logger *slog.Logger) []*contentscripts.Program {
	for _, root := range configs.Config.Extractor.ContentScripts {
		if err := func(root string) error {
			rootFS, err := os.OpenRoot(root)
			if err != nil {
				return err
			}
			defer rootFS.Close() //nolint:errcheck

			return fs.WalkDir(rootFS.FS(), ".", func(name string, x fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if x.IsDir() || path.Ext(name) != ".js" {
					return nil
				}

				pName := filepath.Join(root, name)

				info, err := x.Info()
				if err != nil {
					logger.Error("content script", slog.Any("err", err))
					return nil
				}

				// The file exists already in the cache, check the date
				if p := contentScripts.get(pName); p != nil {
					// No change, stop here
					if p.ModTime.Equal(info.ModTime()) {
						return nil
					}

					contentScripts = slices.DeleteFunc(contentScripts, func(a *contentscripts.Program) bool {
						return a.Name == pName
					})
					logger.Debug("refresh content script", slog.String("name", pName))
				}

				fd, err := rootFS.Open(name)
				if err != nil {
					logger.Error("content script", slog.Any("err", err))
					return nil
				}

				p, err := contentscripts.NewProgram(pName, fd)
				if err != nil {
					logger.Error("content script", slog.Any("err", err))
					return nil
				}
				p.ModTime = info.ModTime()

				contentScripts = append(contentScripts, p)
				logger.Debug("load content script", slog.String("name", pName))

				return nil
			})
		}(root); err != nil {
			logger.Error("content script", slog.Any("err", err))
		}
	}

	return contentScripts
}
