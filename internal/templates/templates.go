// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package templates provides the Jet template loader and catalog.
package templates

import (
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/CloudyKit/jet/v6"

	"codeberg.org/readeck/readeck/assets"
	"codeberg.org/readeck/readeck/locales"
	"codeberg.org/readeck/readeck/pkg/libjet"
	"codeberg.org/readeck/readeck/pkg/strftime"
)

// tplLoader implements a jet.Loader using fs.FS so we can use it
// with embed fs.
type tplLoader struct {
	fs.FS
}

// Exists returns true if the template exists in the filesystem.
func (l *tplLoader) Exists(templatePath string) bool {
	_, err := l.Open(templatePath)
	return err == nil && !os.IsNotExist(err)
}

// Open opens the template at the give path.
func (l *tplLoader) Open(templatePath string) (io.ReadCloser, error) {
	templatePath = strings.TrimPrefix(templatePath, "/")
	return l.FS.Open(templatePath)
}

// Catalog returns a new template set.
func Catalog() *jet.Set {
	set := jet.NewSet(
		&tplLoader{assets.TemplatesFS()},
		jet.WithTemplateNameExtensions([]string{"", ".jet.html"}),
	)

	for k, v := range libjet.FuncMap() {
		set.AddGlobalFunc(k, v)
	}

	for k, v := range libjet.VarMap() {
		set.AddGlobal(k, v)
	}

	set.AddGlobalFunc("date", func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("date", 2, 2)
		v, isNil := libjet.Indirect(args.Get(0))
		if isNil {
			return reflect.ValueOf("")
		}

		date, ok := v.(time.Time)
		if !ok {
			panic("first argument must be a time.Time value or pointer")
		}

		var result string
		format := libjet.ToString(args.Get(1))
		switch format {
		case "RFC3339":
			result = date.Format(time.RFC3339)
		case "RFC3339Nano":
			result = date.Format(time.RFC3339Nano)
		default:
			tr, ok := args.Runtime().Resolve("translator").Interface().(*locales.Locale)
			if !ok {
				result = strftime.Strftime(format, date)
			} else {
				result = strftime.New(tr).Strftime(format, date)
			}
		}

		return reflect.ValueOf(result)
	})

	set.AddGlobalFunc("qrcode", renderQRCode)

	return set
}
