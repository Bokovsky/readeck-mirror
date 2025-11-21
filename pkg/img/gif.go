// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package img

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/gif"
	"image/png"
	"io"

	"github.com/anthonynsimon/bild/transform"
)

var _ MultiFrameImage = &GIFImage{}

func init() {
	AddImageHandler(
		func(r io.Reader) (Image, error) {
			return NewGIFImage(r)
		},
		"image/gif",
	)
}

// GIFImage is the Image implementation for GIF files only.
type GIFImage struct {
	*gif.GIF
	compression ImageCompression
}

// NewGIFImage returns a [GIFImage] instance that decodes all frames
// from a GIF image.
func NewGIFImage(r io.Reader) (*GIFImage, error) {
	buf := new(bytes.Buffer)
	c, err := gif.DecodeConfig(io.TeeReader(r, buf))
	if err != nil {
		return nil, err
	}

	// Limit image size to 30Mpx
	if c.Width*c.Height > 30000000 {
		return nil, errors.New("image is too big")
	}

	g, err := gif.DecodeAll(io.MultiReader(buf, r))
	if err != nil {
		return nil, err
	}

	return &GIFImage{
		GIF: g,
	}, nil
}

// Frames returns the image's number of frames.
func (im *GIFImage) Frames() uint {
	return uint(len(im.Image))
}

// FirstFrame returns the first GIF's image as an [Image].
func (im *GIFImage) FirstFrame() Image {
	return &GIFImage{
		GIF: &gif.GIF{
			Image: []*image.Paletted{im.Image[0]},
		},
		compression: im.compression,
	}
}

// Close frees the resources used by the image and must be called
// when you're done processing it.
func (im *GIFImage) Close() error {
	im.GIF = nil
	return nil
}

// Format returns the image format.
func (im *GIFImage) Format() string {
	return "gif"
}

// ContentType returns the image mimetype.
func (im *GIFImage) ContentType() string {
	return "image/gif"
}

// Width returns the image width.
func (im *GIFImage) Width() uint {
	return uint(im.GIF.Image[0].Bounds().Dx())
}

// Height returns the image height.
func (im *GIFImage) Height() uint {
	return uint(im.GIF.Image[0].Bounds().Dy())
}

// SetFormat is a noop function for this image family.
func (im *GIFImage) SetFormat(_ string) error {
	return nil
}

// SetCompression sets the compression level of PNG endoding.
func (im *GIFImage) SetCompression(c ImageCompression) error {
	im.compression = c
	return nil
}

// SetQuality is a noop function for this image family.
func (im *GIFImage) SetQuality(_ uint8) error {
	return nil
}

// Clean is a noop function for this image family.
func (im *GIFImage) Clean() error {
	return nil
}

// Resize resizes the image to the given width and height.
func (im *GIFImage) Resize(w, h uint) error {
	fb := im.Image[0].Bounds()
	img := image.NewRGBA(image.Rect(0, 0, fb.Dx(), fb.Dy()))
	im.Config.Width = int(w)
	im.Config.Height = int(h)

	for i, frame := range im.Image {
		b := frame.Bounds()
		draw.Draw(img, b, frame, b.Min, draw.Over)
		x := transform.Resize(img, int(w), int(h), transform.NearestNeighbor)

		b = x.Bounds()
		pm := image.NewPaletted(b, frame.Palette)
		draw.FloydSteinberg.Draw(pm, b, x, image.Point{})
		im.Image[i] = pm
	}

	return nil
}

// Encode encodes the image to an io.Writer.
func (im *GIFImage) Encode(w io.Writer) error {
	if len(im.Image) == 1 {
		// A GIF made of one image is converted to PNG
		encoder := &png.Encoder{CompressionLevel: png.BestSpeed}
		if im.compression == CompressionBest {
			encoder.CompressionLevel = png.BestCompression
		}
		return encoder.Encode(w, im.Image[0])
	}

	return gif.EncodeAll(w, im.GIF)
}
