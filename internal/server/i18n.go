// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"context"
	"net/http"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/locales"
)

type (
	ctxLocaleKey struct{}
)

// LoadLocale is a middleware that loads the correct locale for the current user.
// It defaults to English if no user is connected or no language is set.
func LoadLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetRequestUser(r)
		lang := "en-US"
		var tr *locales.Locale
		if !user.IsAnonymous() {
			lang = user.Settings.Lang
		} else {
			// No user connected, used the browser preference
			lang = r.Header.Get("accept-language")
		}

		tr = locales.LoadTranslation(lang)
		ctx := context.WithValue(r.Context(), ctxLocaleKey{}, tr)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Locale returns the current user's locale.
func Locale(r *http.Request) *locales.Locale {
	if t, ok := r.Context().Value(ctxLocaleKey{}).(*locales.Locale); ok {
		return t
	}
	return locales.LoadTranslation("en-US")
}
