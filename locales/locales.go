// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package locales provides l10n tools to render gettext text.
//
// Translations are in the "translations" embedded folder.
package locales

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"log/slog"
	"path"

	"github.com/leonelquinteros/gotext"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// localesFS contains all the translation files
//
//go:embed translations/*/*.mo
var localesFS embed.FS

var (
	catalog = make(map[language.Tag]*Locale)
	allTags = []language.Tag{}
)

// Locale combines a gotext.Translator instance for a given language
// identified by a language.Tag.
type Locale struct {
	Translator gotext.Translator
	Tag        language.Tag
}

// Gettext returns a translation.
func (t *Locale) Gettext(str string, vars ...any) string {
	return t.Translator.Get(str, vars...)
}

// Ngettext returns a pluralized translation.
func (t *Locale) Ngettext(str, plural string, n int, vars ...any) string {
	return t.Translator.GetN(str, plural, n, vars...)
}

// Pgettext returns a contextualized translation.
func (t *Locale) Pgettext(ctx, str string, vars ...any) string {
	return t.Translator.GetC(str, ctx, vars...)
}

// Npgettext returns a pluralized contextualized translation.
func (t *Locale) Npgettext(ctx, str, plural string, n int, vars ...any) string {
	return t.Translator.GetNC(str, plural, n, ctx, vars...)
}

// LoadTranslation loads the best match translation for a given locale code.
func LoadTranslation(lang string) *Locale {
	_, i := language.MatchStrings(language.NewMatcher(allTags), lang, "en")

	return catalog[allTags[i]]
}

// Load loads all the available translations.
func Load() {
	// Add en (empty), first
	if err := addLocale(language.English, new(bytes.Buffer)); err != nil {
		panic(err)
	}

	files, err := fs.Glob(localFilesFS(), "*/messages.mo")
	if err != nil {
		panic(err)
	}

	for _, filename := range files {
		tag := language.Make(path.Dir(filename))
		var r io.Reader
		if r, err = localFilesFS().Open(filename); err != nil {
			panic(err)
		}

		if err = addLocale(tag, r); err != nil {
			panic(err)
		}
	}

	slog.Debug("locales loaded",
		slog.Int("count", len(allTags)),
		slog.Any("locales", allTags),
	)
}

// Available returns the available locales as a list of pair
// containing the langage code and its localized name.
func Available(tag language.Tag) [][2]string {
	res := make([][2]string, 0, len(allTags))
	d := display.Languages(tag)

	for _, t := range allTags {
		s := display.Self.Name(t)
		if d != nil && t != tag {
			s += " (" + d.Name(t) + ")"
		}

		res = append(res, [2]string{t.String(), s})
	}

	return res
}

func localFilesFS() fs.FS {
	sub, err := fs.Sub(localesFS, "translations")
	if err != nil {
		panic(err)
	}
	return sub
}

func addLocale(tag language.Tag, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	mo := gotext.NewMo()
	mo.Parse(b)

	catalog[tag] = &Locale{
		Translator: mo,
		Tag:        tag,
	}

	allTags = append(allTags, tag)
	return nil
}
