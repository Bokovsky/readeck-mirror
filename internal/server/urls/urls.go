// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package urls provides functions to work with the server URLs.
// The main [AbsoluteURL] function deals with the application's prefix
// so you can simply call AbsoluteURL(r, "/bookmarks", someID) and you'll
// get a full URL with a prefixed path.
package urls

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"codeberg.org/readeck/readeck/assets"
	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/http/request"
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

// AbsoluteURL resolves the absolute URL for the given ref path parts.
// If the ref starts with "./", it will resolve relative to the current
// URL.
// The context must have been initialized with [request.InitRequest].
func AbsoluteURL(r *http.Request, parts ...string) *url.URL {
	return absoluteURL(r.URL, parts...)
}

// AbsoluteURLContext resolves the absolute URL for the given ref path parts.
// If the ref starts with "./", it will resolve relative to the current
// URL.
// The context must have been initialized with [request.InitRequest].
func AbsoluteURLContext(ctx context.Context, parts ...string) *url.URL {
	return absoluteURL(request.GetURL(ctx), parts...)
}

func absoluteURL(ref *url.URL, parts ...string) *url.URL {
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

	cur := &url.URL{}
	*cur = *ref

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
		return ref
	}

	return cur.ResolveReference(u)
}

// AssetURL returns an asset's URL using the request.
// The context must have been initialized with [request.InitRequest].
func AssetURL(r *http.Request, name string) *url.URL {
	return assetURL(r.URL, name)
}

// AssetURLContext returns an asset's URL using a context.
// The context must have been initialized with [request.InitRequest].
func AssetURLContext(ctx context.Context, name string) *url.URL {
	return assetURL(request.GetURL(ctx), name)
}

func assetURL(ref *url.URL, name string) *url.URL {
	return absoluteURL(ref, "/assets", assets.AssetMap()[name])
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
