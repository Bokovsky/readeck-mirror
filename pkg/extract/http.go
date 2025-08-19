// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"context"
	"log/slog"
	"net/http"

	"codeberg.org/readeck/readeck/pkg/ctxr"
)

type logSetter interface {
	SetLogger(*slog.Logger)
}

type (
	// FetchType is the type of request the extractor and related tools can make.
	FetchType           uint8
	ctxRequestTypeKey   struct{}
	ctxRequestHeaderKey struct{}
)

var (
	// WithRequestType returns a new context that contains the given [FetchType].
	WithRequestType = ctxr.Setter[FetchType](ctxRequestTypeKey{})
	// CheckRequestType returns the [FetchType] of a given context.
	CheckRequestType = ctxr.Checker[FetchType](ctxRequestTypeKey{})

	// WithRequestHeader returns a new context that contains the given [http.Header].
	WithRequestHeader = ctxr.Setter[http.Header](ctxRequestHeaderKey{})
	// CheckRequestHeader returns the [http.Header] of a given context.
	CheckRequestHeader = ctxr.Checker[http.Header](ctxRequestHeaderKey{})
)

const (
	// PageRequest is a page request type.
	PageRequest FetchType = iota + 1
	// ImageRequest is an image request type.
	ImageRequest
	// ResourceRequest is a resource request type.
	ResourceRequest
	// ContentScriptRequest identifies a request made from a content-script.
	ContentScriptRequest
)

// Fetch builds and performs a GET requests to a given URL.
// It uses [FetchOptions] to add the request type to the request's context
// and headers, if any.
func Fetch(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if header, ok := ctx.Value(ctxRequestHeaderKey{}).(http.Header); ok {
		req.Header = header
	}

	return client.Do(req)
}
