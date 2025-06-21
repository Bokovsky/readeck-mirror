// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/http/csp"
	"codeberg.org/readeck/readeck/pkg/http/forwarded"
	"codeberg.org/readeck/readeck/pkg/http/permissionspolicy"
)

type (
	ctxCSPNonceKey     struct{}
	ctxCSPKey          struct{}
	ctxUnauthorizedKey struct{}
	ctxRemoteInfoKey   struct{}
)

const (
	unauthorizedDefault = iota
	unauthorizedRedir
)

type cspReport struct {
	Report map[string]any `json:"csp-report"`
}

// RemoteInfo contains the host/URL information as built by
// the configuration and/or proxy sent headers.
type RemoteInfo struct {
	IsForced    bool
	IsTrusted   bool
	IsForwarded bool
	ProxyAddr   net.IP
	Host        string
	Scheme      string
}

func newRemoteInfo(r *http.Request) *RemoteInfo {
	pi := &RemoteInfo{
		IsForwarded: forwarded.IsForwarded(r.Header),
		Host:        forwarded.ParseXForwardedHost(r.Header),
		Scheme:      forwarded.ParseXForwardedProto(r.Header),
	}

	// When we've got forwarded headers and an empty scheme, default to https
	if pi.IsForwarded && pi.Scheme == "" {
		pi.Scheme = "https"
	}

	return pi
}

func isTrustedIP(ip net.IP) bool {
	return slices.ContainsFunc(configs.TrustedProxies(), func(network *net.IPNet) bool {
		return network.Contains(ip)
	})
}

func checkHost(r *http.Request) error {
	// If allowed_hosts is not set, do not check the hostname.
	if len(configs.Config.Server.AllowedHosts) == 0 {
		return nil
	}

	host := r.Host
	port := r.URL.Port()
	if port != "" {
		host = strings.TrimSuffix(host, ":"+port)
	}
	host = strings.TrimSuffix(host, ".")

	if slices.Contains(configs.Config.Server.AllowedHosts, host) {
		return nil
	}
	return fmt.Errorf("host is not allowed: %s", host)
}

// GetRemoteInfo returns the [*RemoteInfo] instance stored in the request's context.
func GetRemoteInfo(r *http.Request) *RemoteInfo {
	if hi, ok := r.Context().Value(ctxRemoteInfoKey{}).(*RemoteInfo); ok {
		return hi
	}
	return &RemoteInfo{}
}

// InitRequest update the scheme and host on the incoming
// HTTP request URL (r.URL), based on provided headers and/or
// current environnement.
//
// It also checks the validity of the host header when the server
// is not running in dev mode.
func InitRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First, always remove the port from RenoteAddr
		r.RemoteAddr, _, _ = net.SplitHostPort(r.RemoteAddr)
		remoteIP := net.ParseIP(r.RemoteAddr)

		// Set default scheme
		r.URL.Scheme = "http"
		if r.TLS != nil {
			r.URL.Scheme = "https"
		}

		// Load remoteInfo headers
		remoteInfo := newRemoteInfo(r)
		if remoteInfo.IsForwarded {
			remoteInfo.ProxyAddr = remoteIP
			remoteInfo.IsTrusted = isTrustedIP(remoteIP)
		}

		if configs.Config.Server.BaseURL != nil && configs.Config.Server.BaseURL.IsHTTP() {
			// If a baseURL is set, set scheme and host from it.
			remoteInfo.IsForced = true
			remoteInfo.Host = configs.Config.Server.BaseURL.Host
			remoteInfo.Scheme = configs.Config.Server.BaseURL.Scheme
		}

		// Set host
		if remoteInfo.IsForced || remoteInfo.IsTrusted && remoteInfo.Host != "" {
			r.Host = remoteInfo.Host
		}
		r.URL.Host = r.Host

		// Set scheme
		if remoteInfo.IsForced || remoteInfo.IsForwarded && remoteInfo.Scheme != "" {
			r.URL.Scheme = remoteInfo.Scheme
		}

		// Set client IP
		if remoteInfo.IsTrusted {
			for _, ip := range forwarded.ParseXForwardedFor(r.Header) {
				if isTrustedIP(ip) {
					continue
				}
				r.RemoteAddr = ip.String()
				break
			}
		}

		*r = *r.WithContext(context.WithValue(r.Context(), ctxRemoteInfoKey{}, remoteInfo))

		// Check host
		if !configs.Config.Main.DevMode {
			if err := checkHost(r); err != nil {
				Log(r).Error("server error", slog.Any("err", err))
				Status(w, r, http.StatusBadRequest)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// getDefaultCSP returns the default Content Security Policy
// There are no definition on script-src and style-src because
// the SetSecurityHeaders middleware will set a nonce value
// for each of them.
func getDefaultCSP() csp.Policy {
	return csp.Policy{
		"base-uri":        {csp.None},
		"default-src":     {csp.Self},
		"font-src":        {csp.Self},
		"form-action":     {csp.Self},
		"frame-ancestors": {csp.None},
		"img-src":         {csp.Self, csp.Data},
		"media-src":       {csp.Self, csp.Data},
		"object-src":      {csp.None},
		"script-src":      {csp.ReportSample},
		"style-src":       {csp.ReportSample},
	}
}

// GetCSPHeader extracts the current CSPHeader from the request's context.
func GetCSPHeader(r *http.Request) csp.Policy {
	if c, ok := r.Context().Value(ctxCSPKey{}).(csp.Policy); ok {
		return c
	}
	return getDefaultCSP()
}

// SetSecurityHeaders adds some headers to improve client side security.
func SetSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var nonce string
		if nonce = r.Header.Get("x-turbo-nonce"); nonce == "" {
			nonce = csp.MakeNonce()
		}

		policy := getDefaultCSP()
		policy.Add("script-src", fmt.Sprintf("'nonce-%s'", nonce), csp.UnsafeInline)
		policy.Add("style-src", fmt.Sprintf("'nonce-%s'", nonce), csp.UnsafeInline)
		policy.Add("report-uri", urls.AbsoluteURL(r, "/logger/csp-report").String())

		policy.Write(w.Header())
		permissionspolicy.DefaultPolicy.Write(w.Header())
		w.Header().Set("Referrer-Policy", "same-origin, strict-origin")
		w.Header().Add("X-Frame-Options", "DENY")
		w.Header().Add("X-Content-Type-Options", "nosniff")
		w.Header().Add("X-XSS-Protection", "1; mode=block")
		w.Header().Add("X-Robots-Tag", "noindex, nofollow, noarchive")

		ctx := context.WithValue(r.Context(), ctxCSPNonceKey{}, nonce)
		ctx = context.WithValue(ctx, ctxCSPKey{}, policy)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func cspReportHandler(w http.ResponseWriter, r *http.Request) {
	report := cspReport{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&report); err != nil {
		Log(r).Error("server error", slog.Any("err", err))
		return
	}

	attrs := []slog.Attr{}
	for k, v := range report.Report {
		attrs = append(attrs, slog.Any(k, v))
	}
	Log(r).WithGroup("report").LogAttrs(
		context.Background(),
		slog.LevelWarn,
		"CSP violation",
		attrs...,
	)

	w.WriteHeader(http.StatusNoContent)
}

// unauthorizedHandler is a handler used by the session authentication provider.
// It sends different responses based on the context.
func unauthorizedHandler(w http.ResponseWriter, r *http.Request) {
	unauthorizedCtx, _ := r.Context().Value(ctxUnauthorizedKey{}).(int)

	switch unauthorizedCtx {
	case unauthorizedDefault:
		w.Header().Add("WWW-Authenticate", `Basic realm="Readeck Authentication"`)
		w.Header().Add("WWW-Authenticate", `Bearer realm="Bearer token"`)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, "Unauthorized")
	case unauthorizedRedir:
		if !configs.Config.Commissioned {
			Redirect(w, r, "/onboarding")
			return
		}

		redir := urls.AbsoluteURL(r, "/login")

		// Add the current path as a redirect query parameter
		// to the login route
		q := redir.Query()
		q.Add("r", urls.CurrentPath(r))
		redir.RawQuery = q.Encode()

		w.Header().Set("Location", redir.String())
		w.WriteHeader(http.StatusSeeOther)
	}
}

// WithRedirectLogin sets the unauthorized handler to redirect to the login page.
func WithRedirectLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxUnauthorizedKey{}, unauthorizedRedir)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
