// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package cookbook

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"time"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/pkg/archiver"
	"codeberg.org/readeck/readeck/pkg/extract"
)

var (
	_ archiver.Collector          = &cookbookCollector{}
	_ archiver.ConvertCollector   = &cookbookCollector{}
	_ archiver.PostWriteCollector = &cookbookCollector{}
	_ archiver.LoggerCollector    = &cookbookCollector{}
)

type cookbookCollector struct {
	archiver.SingleFileCollector
	logger *slog.Logger
}

func (c *cookbookCollector) Log() *slog.Logger {
	return c.logger
}

func (c *cookbookCollector) Convert(ctx context.Context, res *archiver.Resource, r io.ReadCloser) (io.ReadCloser, error) {
	return bookmarks.ConvertCollectedImage(ctx, c, res, r)
}

func archiveProcessor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepPostProcess {
		return next
	}

	if len(m.Extractor.HTML) == 0 {
		return next
	}
	if !m.Extractor.Drop().IsHTML() {
		return next
	}
	if !m.Extractor.Drop().IsHTML() {
		return next
	}

	m.Log().Debug("create archive")

	buf := new(bytes.Buffer)
	collector := &cookbookCollector{
		SingleFileCollector: *archiver.NewSingleFileCollector(
			buf,
			m.Extractor.Client(),
			archiver.WithTimeout(time.Second*50),
		),
		logger: m.Log(),
	}

	arc := archiver.New(
		archiver.WithConcurrency(12),
		archiver.WithCollector(collector),
		archiver.WithFlags(
			archiver.EnableImages|archiver.EnableBestImage|archiver.EnableDataAttributes,
		),
	)

	err := arc.ArchiveReader(m.Extractor.Context, bytes.NewReader(m.Extractor.HTML), m.Extractor.URL, "index.html")
	if err != nil {
		panic(err)
	}

	m.Extractor.HTML = buf.Bytes()

	return next
}
