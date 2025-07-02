// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package request provides two middlewares that will set the request's URL
// based on a fixed address and/or reverse proxy headers.
package request

import (
	"net"
	"net/http"
	"net/url"
	"slices"

	"codeberg.org/readeck/readeck/pkg/ctxr"
	"codeberg.org/readeck/readeck/pkg/http/forwarded"
)

type (
	ctxRemoteIPKey struct{}
	ctxRealIPKey   struct{}
	ctxURLKey      struct{}
)

var (
	// GetRemoteIP returns the request's [http.Request.RemoteAddr] as
	// a [net.IP] without its port.
	GetRemoteIP  = ctxr.Getter[net.IP](ctxRemoteIPKey{})
	withRemoteIP = ctxr.Setter[net.IP](ctxRemoteIPKey{})

	// GetRealIP returns the request's client real IP address
	// base on the "X-Real-IP" header. It fallbacks to [http.Request.RemoteAddr].
	GetRealIP  = ctxr.Getter[net.IP](ctxRealIPKey{})
	withRealIP = ctxr.Setter[net.IP](ctxRealIPKey{})

	// GetURL returns the request's [*url.URL].
	GetURL   = ctxr.Getter[*url.URL](ctxURLKey{})
	checkURL = ctxr.Checker[*url.URL](ctxURLKey{})
	withURL  = ctxr.Setter[*url.URL](ctxURLKey{})
)

// InitBaseURL sets the scheme and host taken from the given URL and adds
// it to the context.
// It's better used allongside (before) [InitRequest].
func InitBaseURL(bu *url.URL) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if bu == nil || !isHTTP(bu) {
				next.ServeHTTP(w, r)
				return
			}

			cu := &url.URL{}
			*cu = *r.URL
			cu.Scheme = bu.Scheme
			cu.Host = bu.Host

			next.ServeHTTP(w, r.WithContext(withURL(r.Context(), cu)))
		})
	}
}

// InitRequest adds the request's absolute URL (with scheme and host) to the context.
// It then sets the request URL itself. Host and scheme can be taken from X-Forwarded
// headers, only when the request's remoteAddr is in one of trustedProxies.
//
// When a request is seen as forwarded, its scheme defaults to https and is only
// downgraded to http when X-Forwarded-Proto is "http".
//
// The middleware also adds the remoteAddr (without port) and real remote IP (based on
// X-Forwarded-For).
func InitRequest(trustedProxies ...*net.IPNet) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Set remote IP
			remoteAddr, _, _ := net.SplitHostPort(r.RemoteAddr)
			remoteIP := net.ParseIP(remoteAddr)
			ctx = withRemoteIP(ctx, remoteIP)

			isTrusted := isTrustedProxy(trustedProxies, remoteIP)

			// Set the real remote IP
			if isTrusted {
				for _, ip := range forwarded.ParseXForwardedFor(r.Header) {
					if isTrustedProxy(trustedProxies, ip) {
						continue
					}
					remoteIP = ip
					break
				}
			}
			ctx = withRealIP(ctx, remoteIP)

			// Build request URL
			// Only if it's not already present in context.
			cu, ok := checkURL(r.Context())
			if !ok {
				cu = &url.URL{}
				*cu = *r.URL
				cu.Scheme = "http"
				cu.Host = r.Host

				isForwarded := forwarded.IsForwarded(r.Header)
				if isForwarded {
					// If forwarded, we first default to https.
					// It's the most common use case and it avoids a whole class of
					// configuration errors.
					cu.Scheme = "https"
				}

				if isTrusted && isForwarded {
					if forwarded.ParseXForwardedProto(r.Header) == "http" {
						cu.Scheme = "http"
					}

					if host := forwarded.ParseXForwardedHost(r.Header); host != "" {
						cu.Host = host
					}
				}
				ctx = withURL(ctx, cu)
			}

			// Set the request's URL
			*(r.URL) = *cu

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isHTTP(u *url.URL) bool {
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func isTrustedProxy(p []*net.IPNet, ip net.IP) bool {
	return slices.ContainsFunc(p, func(cidr *net.IPNet) bool {
		return cidr.Contains(ip)
	})
}
