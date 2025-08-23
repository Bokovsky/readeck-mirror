// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package httpclient

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

// CacheTransport is a wrapper around [Transport] that adds a cache layer.
type CacheTransport struct {
	*Transport
	sync.RWMutex

	entries   map[string]*cacheResource
	checkFunc func(*http.Request) bool
}

type cacheResource struct {
	header http.Header
	body   []byte
}

// RoundTrip implements [http.RoundTripper].
// When an entry is found in the cache, it sends a response made out of it. Otherwise,
// it calls the wrapped RoundTrip method.
func (t *CacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	entry := t.getEntry(req)
	if entry != nil {
		t.Log().Debug("cache hit", slog.String("url", req.URL.String()))

		rsp := &http.Response{
			Status:        http.StatusText(http.StatusOK),
			StatusCode:    http.StatusOK,
			Header:        entry.header,
			Request:       req,
			ContentLength: 0,
		}
		if req.Method == http.MethodGet {
			b := bytes.NewReader(entry.body)
			rsp.Body = io.NopCloser(b)
			rsp.ContentLength = b.Size()
		}

		return rsp, nil
	}

	return t.Transport.RoundTrip(req)
}

// addEntry adds an entry into the cache.
func (t *CacheTransport) addEntry(url string, header http.Header, body []byte) {
	t.Lock()
	defer t.Unlock()

	t.entries[url] = &cacheResource{
		header: header,
		body:   body,
	}
}

// hasEntry returns true when the given url exists in the cache.
func (t *CacheTransport) hasEntry(url string) bool {
	t.RLock()
	defer t.RUnlock()

	_, ok := t.entries[url]
	return ok
}

// getEntry returns a [cacheResource] when it exists.
// If the transport has a "checkFunc", it must return true for
// the entry to be returned.
func (t *CacheTransport) getEntry(req *http.Request) *cacheResource {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return nil
	}

	t.RLock()
	defer t.RUnlock()

	url := req.URL.String()
	entry, ok := t.entries[url]
	if !ok {
		return nil
	}

	if t.checkFunc != nil && !t.checkFunc(req) {
		return nil
	}
	return entry
}

// NewCacheClient returns a new [http.Client] with a [CacheTransport] round tripper.
// The "check" function, when not null, can be used to exclude a request from
// any cache request.
func NewCacheClient(check func(*http.Request) bool) *http.Client {
	client := New()
	client.Transport = &CacheTransport{
		Transport: client.Transport.(*Transport),
		entries:   map[string]*cacheResource{},
		checkFunc: check,
	}

	return client
}

// AddToCache adds a URL, headers and body to an [http.Client] cache.
// If the client's transport is not a [CacheTransport] instance, it does nothing.
func AddToCache(client *http.Client, url string, headers http.Header, body []byte) {
	if t, ok := client.Transport.(*CacheTransport); ok {
		t.addEntry(url, headers, body)
	}
}

// IsInCache returns true if a URL exists in an [http.Client] cache.
// If the client's transport is not a [CacheTransport] instance, it does nothing.
func IsInCache(client *http.Client, url string) bool {
	if t, ok := client.Transport.(*CacheTransport); ok {
		return t.hasEntry(url)
	}
	return false
}
