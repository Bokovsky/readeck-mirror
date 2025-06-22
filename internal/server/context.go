// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"net/http"

	"codeberg.org/readeck/readeck/pkg/ctxr"
)

type ctxRequestKey struct{}

// WithRequest returns a context with the given [*http.Request].
var WithRequest = ctxr.Setter[*http.Request](ctxRequestKey{})

// GetRequest returns the [*http.Request] from the context.
var GetRequest = ctxr.Getter[*http.Request](ctxRequestKey{})
