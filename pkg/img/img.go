// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/*
Package img provides a unified image loader and manipulation pipeline.
It can load images using the native go implementations for the major
web image types, perform some manipulation like resizing and write
the result to an io.Writer.
*/
package img

import (
	"fmt"
	"io"
)

// ImageCompression is the compression level used for PNG images.
type ImageCompression uint8

const (
	// CompressionFast is a fast method.
	CompressionFast ImageCompression = iota

	// CompressionBest is the best space saving method.
	CompressionBest
)

// Image describes the interface of an image manipulation type.
type Image interface {
	Close() error
	Format() string
	ContentType() string
	Width() uint
	Height() uint
	SetFormat(string) error
	Resize(uint, uint) error
	Encode(io.Writer) (string, error)
	SetCompression(ImageCompression) error
	SetQuality(uint8) error
	Clean() error
}

// MultiFrameImage describes the interface of an image with multiple frames,
// such as animated GIFs.
type MultiFrameImage interface {
	Image
	Frames() uint
	FirstFrame() Image
}

// ImageFilter is a filter application function used by the
// Pipeline method of an Image instance.
type ImageFilter func(Image) error

// Handler is a function that returns an Image type instance from an io.Reader.
type Handler func(io.Reader) (Image, error)

type handlersList map[string]Handler

// handlers holds the available image handlers.
var handlers = make(handlersList)

// AddImageHandler adds (or replaces) a handler for the given types.
func AddImageHandler(f Handler, types ...string) {
	for _, t := range types {
		handlers[t] = f
	}
}

// New creates a new Image instance, using the available handlers.
func New(contentType string, r io.Reader) (Image, error) {
	handler, ok := handlers[contentType]
	if !ok {
		return nil, fmt.Errorf("no img handler for %s", contentType)
	}

	return handler(r)
}

// Pipeline apply all the given ImageFilter functions to the image.
func Pipeline(im Image, filters ...ImageFilter) error {
	for _, fn := range filters {
		err := fn(im)
		if err != nil {
			return err
		}
	}
	return nil
}

// Fit resizes the image keeping the aspect ratio and staying within
// the given width and height.
func Fit(im Image, w, h uint) error {
	if w == 0 && h == 0 {
		return nil
	}

	// original dimension
	ow := float64(im.Width())
	oh := float64(im.Height())
	aspectRatio := ow / oh

	// target dimension
	tw := float64(w)
	th := float64(h)

	// if width or height is zero, adjust it based on the other dimension
	// and aspect ratio.
	if w == 0 {
		tw = th * aspectRatio
	}
	if h == 0 {
		th = tw / aspectRatio
	}

	// When the target ens up being bigger, don't resize it.
	if tw > ow && th > oh {
		return nil
	}

	if aspectRatio > tw/th {
		th = tw / aspectRatio
	} else {
		tw = th * aspectRatio
	}

	return im.Resize(uint(tw), uint(th))
}
