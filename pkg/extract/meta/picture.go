// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"context"
	"log/slog"

	"codeberg.org/readeck/readeck/pkg/extract"
)

// ExtractPicture is a processor that extracts the picture from the document
// metadata. It has to come after ExtractMeta.
func ExtractPicture(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepFinish || m.Position() > 0 {
		return next
	}

	d := m.Extractor.Drop()
	if d.Meta == nil {
		return next
	}

	href := d.Meta.LookupGet(
		"x.picture_url",
		"graph.image",
		"twitter.image",
		"oembed.thumbnail_url",
		"ld.image",
	)

	if href == "" {
		return next
	}

	size := uint(extract.ImageSizeThumbnail)
	if d.DocumentType == "photo" {
		size = extract.ImageSizeWide
	}

	m.Log().Debug("loading picture", slog.String("href", href))

	picture, err := extract.NewPicture(href, d.URL)
	if err != nil {
		m.Log().Warn("", slog.Any("err", err))
		return next
	}

	ctx := extract.WithReferrer(context.Background(), m.Extractor.Drop().URL)

	if err = picture.Load(ctx, m.Extractor.Client(), size, ""); err != nil {
		m.Log().Warn("cannot load picture",
			slog.Any("err", err),
			slog.String("url", href),
		)
		return next
	}

	d.Pictures["image"] = picture
	m.Log().Debug("picture loaded", slog.Any("size", picture.Size[:]))

	thumbnail, err := picture.Copy(380, "")
	if err != nil {
		m.Log().Warn("", slog.Any("err", err))
		return next
	}
	d.Pictures["thumbnail"] = thumbnail

	return next
}
