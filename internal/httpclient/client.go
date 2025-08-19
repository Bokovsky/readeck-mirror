// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package httpclient is Readeck's own HTTP client.
// It provides an [http.RoundTripper] with sensible defaults
// that can make outgoing requests look like they come from a browser.
package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"maps"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/glob"
)

type ctxProxyURLKey struct{}

const uaString = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.3"

// defaultDialer is our own default net.Dialer with shorter timeout and keepalive.
var defaultDialer = net.Dialer{
	Timeout:   15 * time.Second,
	KeepAlive: 30 * time.Second,
}

var cipherSuites = []uint16{
	// Chrome like cipher suite
	tls.TLS_AES_128_GCM_SHA256,
	tls.TLS_AES_256_GCM_SHA384,
	tls.TLS_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
}

var greaseCiphers = []uint16{
	0x0a0a, 0x1a1a, 0x2a2a, 0x3a3a, 0x4a4a, 0x5a5a, 0x6a6a, 0x7a7a,
	0x8a8a, 0x9a9a, 0xaaaa, 0xbaba, 0xcaca, 0xdada, 0xeaea, 0xfafa,
}

// defaultTransport is our http.RoundTripper with some custom settings.
var defaultTransport = &http.Transport{
	DialContext: defaultDialer.DialContext,
	Proxy:       proxyMatcher,
	TLSClientConfig: &tls.Config{
		// Note: although some ciphers and TLS version are disabled by default for good reasons,
		// we need to enable them for some websites :/
		CipherSuites: cipherSuites,
		MinVersion:   tls.VersionTLS12,
	},
	ForceAttemptHTTP2:     true,
	DisableCompression:    false,
	DisableKeepAlives:     false,
	MaxIdleConns:          50,
	MaxIdleConnsPerHost:   2,
	IdleConnTimeout:       30 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// defaultHeaders are the HTTP headers that are sent with every new request.
// They're attached to the transport and can be overridden and/or modified
// while using the associated client.
var defaultHeaders = http.Header{
	"User-Agent":                []string{uaString},
	"Accept":                    []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/jpeg,image/png,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
	"Accept-Language":           []string{"en-US,en;q=0.8"},
	"Cache-Control":             []string{"max-age=0"},
	"Upgrade-Insecure-Requests": []string{"1"},
	"Sec-CH-UA":                 []string{`"Google Chrome";v="137", "Chromium";v="137"`},
	"Sec-CH-UA-mobile":          []string{"?0"},
	"Sec-CH-UA-platform":        []string{`"Windows"`},
	"Sec-Fetch-Site":            []string{"none"},
}

func proxyMatcher(req *http.Request) (u *url.URL, err error) {
	for _, p := range configs.Config.Extractor.ProxyMatch {
		if glob.Glob(p.Host(), req.URL.Host) {
			u = p.URL()
			break
		}
	}

	if u == nil {
		u, err = http.ProxyFromEnvironment(req)
	}

	if u != nil {
		*req = *(req.WithContext(context.WithValue(req.Context(), ctxProxyURLKey{}, u)))
	}

	return
}

// Transport wraps an [http.RoundTripper].
type Transport struct {
	http.RoundTripper
	header http.Header
	logger *slog.Logger
}

// RoundTrip implements [http.RoundTripper].
// It checks if the destination IP is allowed, adds default headers and
// logs (debug-10 level) every request.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.checkDestIP(r); err != nil {
		return nil, err
	}

	// A RoundTripper should not modify the request. Since we only want to add
	// headers, we can work with a shallow copy.
	req := new(http.Request)
	*req = *r
	req.Header = req.Header.Clone()

	// Add the client's default headers that don't exist in the
	// current request.
	for k, values := range t.header {
		if _, ok := r.Header[textproto.CanonicalMIMEHeaderKey(k)]; !ok {
			req.Header[k] = values
		}
	}

	attrs := []slog.Attr{
		slog.Group("request",
			slog.String("url", req.URL.String()),
			slog.String("method", req.Method),
			slog.Any("headers", req.Header),
		),
	}

	t.setTLSGrease()
	now := time.Now()
	rsp, err := t.RoundTripper.RoundTrip(req)

	if p, ok := req.Context().Value(ctxProxyURLKey{}).(*url.URL); ok {
		t.Log().Debug("using proxy",
			slog.Any("domain", req.URL.Host),
			slog.Any("proxy", p.String()),
		)
	}

	if err != nil {
		attrs = append(attrs, slog.Group("response",
			slog.Any("err", err),
		))
	} else {
		attrs = append(attrs, slog.Group("response",
			slog.Int("status", rsp.StatusCode),
			slog.Any("headers", rsp.Header),
		))
	}
	attrs = append(attrs, slog.Duration("time", time.Since(now)))
	t.Log().LogAttrs(context.Background(), slog.LevelDebug-10, "request", attrs...)

	return rsp, err
}

// setTLSGrease adds a random GREASE cipher to the cipher suite.
// see https://www.rfc-editor.org/rfc/rfc8701
func (t *Transport) setTLSGrease() {
	if tr, ok := t.RoundTripper.(*http.Transport); ok {
		tr.TLSClientConfig.CipherSuites = append([]uint16{
			greaseCiphers[rand.IntN(len(greaseCiphers))], //nolint:gosec
		}, cipherSuites...)
	}
}

func (t *Transport) checkDestIP(r *http.Request) error {
	if len(configs.Config.Extractor.DeniedIPs) == 0 {
		// An empty list disables the IP check.
		return nil
	}

	hostname := r.URL.Hostname()
	host, err := idna.ToASCII(hostname)
	if err != nil {
		return fmt.Errorf("invalid hostname %s", hostname)
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve %s", host)
	}

	for _, cidr := range configs.Config.Extractor.DeniedIPs {
		for _, ip := range ips {
			if cidr.Contains(ip) {
				return fmt.Errorf("ip %s is blocked by rule %s", ip, cidr)
			}
		}
	}

	return nil
}

// Log returns the transport's logger.
func (t *Transport) Log() *slog.Logger {
	return t.logger
}

// SetLogger sets the transport's logger.
func (t *Transport) SetLogger(l *slog.Logger) {
	t.logger = l
}

// SetHeader receives a function that can manipulate the
// transport's default headers.
func (t *Transport) SetHeader(fn func(h http.Header)) {
	fn(t.header)
}

// New returns a new client with an empty cookie storage and a [Transport] instance.
func New() *http.Client {
	cookies, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	return &http.Client{
		Transport: &Transport{
			RoundTripper: defaultTransport.Clone(),
			header:       maps.Clone(defaultHeaders),
			logger:       slog.Default(),
		},
		Timeout: 10 * time.Second,
		Jar:     cookies,
	}
}
