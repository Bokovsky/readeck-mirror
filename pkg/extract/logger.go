// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

type groupOrAttrs struct {
	group string      // group name if non-empty
	attrs []slog.Attr // attrs if non-empty
}

type logRecorder struct {
	slog.Handler
	level     slog.Level
	extractor *Extractor
	mu        *sync.Mutex
	goas      []groupOrAttrs
}

func newLogRecorder(handler slog.Handler, level slog.Level, extractor *Extractor) *logRecorder {
	return &logRecorder{
		Handler:   handler,
		level:     level,
		extractor: extractor,
		mu:        &sync.Mutex{},
		goas:      []groupOrAttrs{},
	}
}

func (h *logRecorder) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *logRecorder) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	b := new(strings.Builder)
	lName := r.Level.String()
	if r.Level < slog.LevelDebug {
		lName = "TRACE"
	}

	fmt.Fprintf(b, "[%s] ", lName[0:4])
	fmt.Fprintf(b, "%s ", strings.TrimSpace(r.Message))

	goas := h.goas
	if r.NumAttrs() == 0 {
		// If the record has no Attrs, remove groups at the end of the list; they are empty.
		for len(goas) > 0 && goas[len(goas)-1].group != "" {
			goas = goas[:len(goas)-1]
		}
	}

	prefix := ""
	for _, goa := range goas {
		if goa.group != "" {
			prefix += goa.group + "."
		}

		for _, a := range goa.attrs {
			h.renderAttr(b, prefix, a)
		}
	}

	r.Attrs(func(a slog.Attr) bool {
		h.renderAttr(b, prefix, a)
		return true
	})

	h.extractor.Logs = append(h.extractor.Logs, strings.TrimSpace(b.String()))

	if r.Level >= slog.LevelError {
		h.extractor.errors = append(h.extractor.errors, errors.New(r.Message))
	}

	if h.Handler.Enabled(ctx, r.Level) {
		return h.Handler.Handle(ctx, r)
	}
	return nil
}

func (h *logRecorder) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := h.withGroupOrAttrs(groupOrAttrs{attrs: attrs})
	h2.Handler = h2.Handler.WithAttrs(attrs)
	return h2
}

func (h *logRecorder) WithGroup(name string) slog.Handler {
	h2 := h.withGroupOrAttrs(groupOrAttrs{group: name})
	h2.Handler = h2.Handler.WithGroup(name)
	return h2
}

func (h *logRecorder) withGroupOrAttrs(goa groupOrAttrs) *logRecorder {
	h2 := *h
	h2.goas = make([]groupOrAttrs, len(h.goas)+1)
	copy(h2.goas, h.goas)
	h2.goas[len(h2.goas)-1] = goa
	return &h2
}

func (h *logRecorder) renderAttr(w *strings.Builder, prefix string, attr slog.Attr) {
	if attr.Value.Kind() == slog.KindGroup {
		for _, a := range attr.Value.Group() {
			h.renderAttr(w, prefix+attr.Key+".", a)
		}
		return
	}
	fmt.Fprintf(w, `%s%s="%v" `, prefix, attr.Key, attr.Value)
}
