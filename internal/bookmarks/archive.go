// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/go-shiori/dom"
	"github.com/google/uuid"

	"codeberg.org/readeck/readeck/pkg/archiver"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	"codeberg.org/readeck/readeck/pkg/img"
)

const (
	resourceDirName = "_resources"
)

var (
	// We can't process too many images at the same time if we don't want to overload
	// the system and freeze everything just because the image processing has way
	// too much work to do.
	imgSem = semaphore.NewWeighted(2)
	imgCtx = context.Background()

	archiveFlags = archiver.EnableImages | archiver.EnableBestImage | archiver.EnableDataAttributes
)

// ResourceDirName returns the resource folder name in an archive.
func ResourceDirName() string {
	return resourceDirName
}

// interface guards.
var (
	_ archiver.Collector        = &bookmarkCollector{}
	_ archiver.ConvertCollector = &bookmarkCollector{}
	_ archiver.LoggerCollector  = &bookmarkCollector{}
)

// for these formats, we copy them directly if they don't need resizing.
// Note: as tempting as allowing webp can be, it must be converted
// to either jpeg or png for maximum EPUB compatibility.
var directCopyFormats = map[string]struct{}{
	"image/gif":  {},
	"image/jpeg": {},
	"image/png":  {},
}

type bookmarkCollector struct {
	archiver.ZipCollector
	logger *slog.Logger
}

func (c *bookmarkCollector) Log() *slog.Logger {
	return c.logger
}

func (c *bookmarkCollector) Name(uri string) string {
	return "./" + path.Join(
		ResourceDirName(),
		base58.EncodeUUID(uuid.NewSHA1(uuid.NameSpaceURL, []byte(uri))),
	)
}

func (c *bookmarkCollector) Convert(ctx context.Context, res *archiver.Resource, r io.ReadCloser) (io.ReadCloser, error) {
	return ConvertCollectedImage(ctx, c, res, r)
}

// ConvertCollectedImage copies or convert a collected image.
// When an image fits in [extract.ImageSizeWide] and is in a compatible format, its
// reader is returned directly. Otherwise, it's converted to a suitable format.
func ConvertCollectedImage(ctx context.Context, c archiver.Collector, res *archiver.Resource, r io.ReadCloser) (io.ReadCloser, error) {
	if strings.Split(res.ContentType, "/")[0] != "image" {
		return r, nil
	}

	node := archiver.GetNodeContext(ctx)
	l := archiver.Logger(c).With(slog.Any("url", archiver.URLLogValue(res.URL())))
	readabilityEnabled, _ := contents.IsReadabilityEnabled(ctx)

	// First pass ignoring icons
	if readabilityEnabled && contents.IsIcon(node, res.Width, res.Height, 64) {
		l.Debug("remove icon image",
			slog.Int("w", res.Width),
			slog.Int("h", res.Height),
		)
		return nil, archiver.ErrRemoveSrc
	}

	// Direct copy of small enough images and in a compatible format.
	if _, ok := directCopyFormats[res.ContentType]; ok && res.Width <= extract.ImageSizeWide {
		l.Debug("copy image", slog.Group("resource",
			slog.String("type", res.ContentType),
			slog.Int("w", res.Width),
			slog.Int("h", res.Height),
		))
		return r, nil
	}

	err := imgSem.Acquire(imgCtx, 1)
	if err != nil {
		return nil, archiver.ErrRemoveSrc
	}
	defer imgSem.Release(1)

	im, err := img.New(res.ContentType, r)
	r.Close() //nolint:errcheck
	if err != nil {
		l.Warn("open image",
			slog.String("format", res.ContentType),
			slog.Any("err", err),
		)
		return nil, archiver.ErrRemoveSrc
	}
	defer func() {
		if err := im.Close(); err != nil {
			l.Warn("closing image", slog.Any("err", err))
		}
	}()

	// Second pass ignoring icons
	if readabilityEnabled && contents.IsIcon(node, int(im.Width()), int(im.Height()), 64) {
		l.Debug("remove icon image",
			slog.Int("w", int(im.Width())),
			slog.Int("h", int(im.Height())),
		)
		return nil, archiver.ErrRemoveSrc
	}

	if err = img.Pipeline(im,
		func(im img.Image) error { return im.Clean() },
		func(im img.Image) error { return im.SetQuality(75) },
		func(im img.Image) error { return im.SetCompression(img.CompressionBest) },
		func(im img.Image) error { return img.Fit(im, extract.ImageSizeWide, 0) },
	); err != nil {
		l.Warn("convert image", slog.Any("err", err))
		return nil, archiver.ErrRemoveSrc
	}

	if node != nil && dom.TagName(node) == "img" {
		dom.SetAttribute(node, "width", strconv.FormatUint(uint64(im.Width()), 10))
		dom.SetAttribute(node, "height", strconv.FormatUint(uint64(im.Height()), 10))
	}

	buf := new(bytes.Buffer)
	if err = im.Encode(buf); err != nil {
		l.Warn("encode image", slog.Any("err", err))
		return nil, archiver.ErrRemoveSrc
	}

	l.Debug("convert image",
		slog.Group("resource",
			slog.String("type", res.ContentType),
			slog.Int("w", int(im.Width())),
			slog.Int("h", int(im.Height())),
		),
		slog.Group("image",
			slog.String("format", im.Format()),
			slog.Int("w", int(im.Width())),
			slog.Int("h", int(im.Height())),
		),
	)

	res.ContentType = im.ContentType()
	res.Width = int(im.Width())
	res.Height = int(im.Height())
	res.Name = strings.TrimSuffix(res.Name, path.Ext(res.Name))
	res.Name += archiver.GetExtension(res.ContentType)

	return io.NopCloser(buf), nil
}

// ArchiveDocument runs the archiver and returns a the number of saved resources.
func ArchiveDocument(ctx context.Context, zw *zip.Writer, ex *extract.Extractor) (int, error) {
	collector := &bookmarkCollector{
		ZipCollector: *archiver.NewZipCollector(
			zw,
			ex.Client(),
			archiver.WithTimeout(50*time.Second),
		),
		logger: ex.Log(),
	}

	arc := archiver.New(
		archiver.WithFlags(archiveFlags),
		archiver.WithConcurrency(8),
		archiver.WithCollector(collector),
	)

	err := arc.ArchiveReader(ctx, bytes.NewReader(ex.HTML), ex.Drop().URL, "index.html")
	if err != nil {
		return 0, err
	}

	count := 0
	for res := range collector.Resources() {
		if res.Saved() {
			count++
		}
	}

	return count, nil
}
