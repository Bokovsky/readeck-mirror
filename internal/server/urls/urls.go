// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package urls provides functions to work with the server URLs.
// The main [AbsoluteURL] function deals with the application's prefix
// so you can simply call AbsoluteURL(r, "/bookmarks", someID) and you'll
// get a full URL with a prefixed path.
package urls

import (
	"net/http"
	"net/url"
	"strings"

	"codeberg.org/readeck/readeck/assets"
	"codeberg.org/readeck/readeck/configs"
)

// Prefix returns the configured URL prefix.
func Prefix() string {
	return configs.Config.Server.Prefix
}

// CurrentPath returns the path of the current request
// after striping the server's base path. This value
// can later be used in the AbsoluteURL
// or Redirect functions.
func CurrentPath(r *http.Request) string {
	p, ok := strings.CutPrefix(r.URL.Path, Prefix())
	if !ok {
		return ""
	}
	p = "/" + p
	if r.URL.RawQuery != "" {
		p += "?" + r.URL.RawQuery
	}

	return p
}

// AbsoluteURL resolve the absolute URL for the given ref path parts.
// If the ref starts with "./", it will resolve relative to the current
// URL.
func AbsoluteURL(r *http.Request, parts ...string) *url.URL {
	// First deal with parts
	for i, p := range parts {
		if i == 0 && strings.HasPrefix(p, "./") {
			p = "."
		}
		if i > 0 {
			parts[i] = strings.TrimLeft(p, "/")
		}
	}

	pathName := strings.Join(parts, "/")

	cur, _ := r.URL.Parse("")

	p, _ := url.Parse(pathName) // Never let a full URL pass in the parts
	pathName = p.Path

	// If the url is relative, we need a final slash on the original path
	if strings.HasPrefix(pathName, "./") && !strings.HasSuffix(cur.Path, "/") {
		cur.Path += "/"
	}

	// If the url is absolute, we must prepend the basePath
	if strings.HasPrefix(pathName, "/") {
		pathName = Prefix() + pathName[1:]
	}

	// Append query string if any
	if p.RawQuery != "" {
		pathName += "?" + p.RawQuery
	}

	var u *url.URL
	var err error
	if u, err = url.Parse(pathName); err != nil {
		return r.URL
	}

	return cur.ResolveReference(u)
}

// AssetURL returns the real URL for a given asset.
func AssetURL(r *http.Request, name string) *url.URL {
	return AbsoluteURL(r, "/assets", assets.AssetMap()[name])
}

// PathOnly returns the URL path + query + fragment.
func PathOnly(u *url.URL) string {
	var buf strings.Builder
	p := u.EscapedPath()
	if p != "" && p[0] != '/' {
		buf.WriteByte('/')
	}
	buf.WriteString(p)

	if u.RawQuery != "" {
		buf.WriteByte('?')
		buf.WriteString(u.RawQuery)
	}

	if u.Fragment != "" {
		buf.WriteByte('#')
		buf.WriteString(u.EscapedFragment())
	}

	return buf.String()
}
