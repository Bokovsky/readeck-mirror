// SPDX-FileCopyrightText: © 2020 Radhi Fadlillah
// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: MIT

package archiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

var errSkippedURL = errors.New("skip processing url")

type processOptions struct {
	headers http.Header
}

func (arc *Archiver) processURL(ctx context.Context, uri string, options processOptions) (*Resource, error) {
	// Make sure this URL is not empty, data or hash. If yes, just skip it.
	uri = strings.TrimSpace(uri)
	if uri == "" || strings.HasPrefix(uri, "#") {
		return nil, fmt.Errorf("%w: %s", errSkippedURL, uri)
	}

	uri = requestURI(uri)

	// Resource exists and is saved, done.
	if res, ok := arc.collector.Get(uri); ok && res.Saved() {
		return res, nil
	}

	// Fetch resource
	log := arc.log().With(slog.Any("url", URLLogValue(uri)))
	if n := GetNodeContext(ctx); n != nil {
		log = log.With(slog.Any("node", NodeLogValue(n)))
	}

	body, res, err := arc.fetch(ctx, uri, options.headers)
	if err != nil || res.status/100 != 2 {
		log.Warn("failed to fetch resource",
			slog.Int("status", res.status),
			slog.Any("err", err),
		)
		return nil, errors.Join(errSkippedURL, err)
	}
	defer body.Close() //nolint:errcheck

	switch res.ContentType {
	case "text/html":
		// TODO: process embedded (iframe, object) HTML
		if n := GetNodeContext(ctx); n != nil {
			log.Warn("HTML")
		}
	case "text/css":
		// process css
		buf, err := arc.processCSS(ctx, body, res)
		if err != nil {
			return nil, err
		}
		if res, err = arc.saveResource(ctx, io.NopCloser(buf), res); err != nil {
			return nil, err
		}
	default:
		if res, err = arc.saveResource(ctx, body, res); err != nil {
			return nil, err
		}
	}

	return res, nil
}
