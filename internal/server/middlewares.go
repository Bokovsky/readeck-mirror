// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/klauspost/compress/gzhttp"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/pkg/http/accept"
)

const (
	gzipEtagSuffix = "-gzip"
)

var acceptOffers = []string{
	"text/plain",
	"text/html",
	"application/json",
}

var csrfProtection = http.NewCrossOriginProtection()

// Csrf setup the CSRF protection, using the native Go [http.CrossOriginProtection].
// https://words.filippo.io/csrf/
func Csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := csrfProtection.Check(r); err != nil {
			Log(r).Warn("Cross Origin", slog.Any("err", err))
			Status(w, r, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// WithPermission enforce a permission check on the request's path for
// the given action.
//
// In the RBAC configuration, the user's group is the subject, the
// given "obj" is the object and "act" is the action.
func WithPermission(obj, act string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := auth.GetRequestUser(r)
			ok := auth.HasPermission(r, obj, act)

			logger := Log(r).With(
				slog.String("user", u.Username),
				slog.String("sub", u.Group),
				slog.String("obj", obj),
				slog.String("act", act),
				slog.Bool("granted", ok),
			)

			if logger.Enabled(context.Background(), slog.LevelDebug) {
				logger.Debug("access control", slog.Any("permissions", auth.GetPermissions(r)))
			}

			if !ok {
				logger.Warn("access denied")
				w.Header().Set("content-type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("access denied"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CannonicalPaths cleans the URL path and removes trailing slashes.
// It returns a 308 redirection so any form will pass through.
func CannonicalPaths(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p string
		rctx := chi.RouteContext(r.Context())
		if rctx != nil && rctx.RoutePath != "" {
			p = rctx.RoutePath
		} else {
			p = r.URL.Path
		}

		if len(p) > 1 {
			p2 := path.Clean(p)
			if strings.HasSuffix(p, "/") {
				p2 += "/"
			}
			if p != p2 {
				if r.URL.RawQuery != "" {
					p2 = fmt.Sprintf("%s?%s", p2, r.URL.RawQuery)
				}
				http.Redirect(w, r, fmt.Sprintf("//%s%s", r.Host, p2), http.StatusPermanentRedirect)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// CompressResponse returns a gzipped response for some content types.
// It uses gzhttp that provides a BREACH mittigation.
func CompressResponse(next http.Handler) http.Handler {
	w, err := gzhttp.NewWrapper(
		gzhttp.CompressionLevel(5),
		gzhttp.ContentTypes([]string{
			"application/json", "application/atom+xml",
			"text/html", "text/plain", "text/vnd.turbo-stream.html",
			"image/svg+xml",
		}),
		gzhttp.SuffixETag(gzipEtagSuffix),
		gzhttp.MinSize(1024),
		gzhttp.RandomJitter(32, 0, false),
	)
	if err != nil {
		panic(err)
	}
	return w(next)
}

// ErrorPages is a middleware that overrides the response writer so
// that, under some conditions, it can send a response matching the
// "accept" request header.
//
// Conditions are: response status must be >= 400, its content-type
// is text/plain and it has some content.
func ErrorPages(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wi := &responseWriterInterceptor{
			ResponseWriter: w,
			r:              r,
			accept:         accept.NegotiateContentType(r.Header, acceptOffers, "text/html"),
			errorTemplates: make(map[int]string),
		}

		next.ServeHTTP(wi, r)
	})
}

type responseWriterInterceptor struct {
	http.ResponseWriter
	r              *http.Request
	accept         string
	contentType    string
	statusCode     int
	errorTemplates map[int]string
}

// needsOverride returns true when a content-type is text/plain and status >= 400.
func (w *responseWriterInterceptor) needsOverride() bool {
	return w.contentType == "text/plain" && w.statusCode >= 400
}

// WriteHeader intercepts the status code sent to the writter and saves some
// information if needed.
func (w *responseWriterInterceptor) WriteHeader(statusCode int) {
	defer func() {
		w.ResponseWriter.WriteHeader(statusCode)
	}()

	if statusCode < 400 || statusCode == 422 { // immediate shortcut
		return
	}
	w.statusCode = statusCode

	if w.contentType == "" {
		w.contentType = "text/plain"
		ct := strings.SplitN(w.Header().Get("content-type"), ";", 2)
		if ct[0] != "" {
			w.contentType = ct[0]
		}
	}

	if w.needsOverride() {
		w.ResponseWriter.Header().Set("Content-Type", w.accept+"; charset=utf-8")
	}
}

// Write overrides the wrapped Write method to discard all contents and
// send its own response when it needs to.
func (w *responseWriterInterceptor) Write(c []byte) (int, error) {
	if !w.needsOverride() {
		return w.ResponseWriter.Write(c)
	}

	switch w.accept {
	case "application/json":
		b, _ := json.Marshal(Message{
			Status:  w.statusCode,
			Message: http.StatusText(w.statusCode),
		})
		return w.ResponseWriter.Write(b)
	case "text/html":
		ctx := TC{"Status": w.statusCode}
		tpl, ok := w.errorTemplates[w.statusCode]
		if !ok {
			tpl = "/error"
		}

		RenderTemplate(w.ResponseWriter, w.r, 0, tpl, ctx)
	default:
		return w.ResponseWriter.Write([]byte(http.StatusText(w.statusCode)))
	}

	return 0, nil
}

// WithCustomErrorTemplate registers a custom template for an error rendered as HTML.
// It must be set before any middleware that would trigger an HTTP error.
func WithCustomErrorTemplate(status int, template string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if w, ok := w.(*responseWriterInterceptor); ok {
				w.errorTemplates[status] = template
			}
			next.ServeHTTP(w, r)
		})
	}
}
