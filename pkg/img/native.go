// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package img

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"io"

	"image/gif"  // GIF decoder and encoder
	"image/jpeg" // JPEG decoder and encoder
	"image/png"  // PNG decoder and encoder

	_ "codeberg.org/readeck/readeck/pkg/img/ico" // ICO decoder
	_ "golang.org/x/image/bmp"                   // BMP decoder
	_ "golang.org/x/image/tiff"                  // TIFF decoder
	_ "golang.org/x/image/webp"                  // WEBP decoder

	"github.com/anthonynsimon/bild/effect"
	"github.com/anthonynsimon/bild/transform"
)

func init() {
	AddImageHandler(
		func(r io.Reader) (Image, error) {
			return NewNativeImage(r)
		},
		"image/bmp",
		"image/gif",
		"image/x-icon",
		"image/vnd.microsoft.icon",
		"image/jpeg",
		"image/png",
		"image/tiff",
		"image/webp",
	)
}

// NativeImage is the Image implementation using Go native
// image tools.
type NativeImage struct {
	m           image.Image
	format      string
	lossless    bool
	encFormat   string
	compression ImageCompression
	quality     uint8
}

// NewNativeImage returns a NativeImage instance using Go native image tools.
func NewNativeImage(r io.Reader) (*NativeImage, error) {
	buf := new(bytes.Buffer)
	c, format, err := image.DecodeConfig(io.TeeReader(r, buf))
	if err != nil {
		return nil, err
	}

	// Limit image size to 30Mpx
	if c.Width*c.Height > 30000000 {
		return nil, errors.New("image is too big")
	}

	// Check for lossless webp
	lossless := false
	if format == "webp" {
		// DecodeConfig already read 4096 bytes, so we can read
		// the 16th directly from it.
		lossless = buf.Bytes()[15] == 'L'
	}

	m, _, err := image.Decode(io.MultiReader(buf, r))
	if err != nil {
		return nil, err
	}

	return &NativeImage{
		m:           m,
		format:      format,
		lossless:    lossless,
		compression: CompressionFast,
		quality:     80,
	}, nil
}

// Image returns the wrapped image instance.
func (im *NativeImage) Image() image.Image {
	return im.m
}

// Close frees the resources used by the image and must be called
// when you're done processing it.
func (im *NativeImage) Close() error {
	return nil
}

// Format returns the image format.
func (im *NativeImage) Format() string {
	return im.format
}

// ContentType returns the image mimetype.
func (im *NativeImage) ContentType() string {
	return "image/" + im.format
}

// Width returns the image width.
func (im *NativeImage) Width() uint {
	return uint(im.m.Bounds().Dx())
}

// Height returns the image height.
func (im *NativeImage) Height() uint {
	return uint(im.m.Bounds().Dy())
}

// SetFormat sets the encoding format. When none
// is set, it will use the original format or fallback
// to JPEG.
func (im *NativeImage) SetFormat(f string) error {
	im.encFormat = f
	return nil
}

// SetCompression sets the compression level of PNG encoding.
func (im *NativeImage) SetCompression(c ImageCompression) error {
	im.compression = c
	return nil
}

// SetQuality sets the quality of JEPG encoding.
func (im *NativeImage) SetQuality(q uint8) error {
	im.quality = q
	return nil
}

// Resize resizes the image to the given width and height.
func (im *NativeImage) Resize(w, h uint) error {
	im.m = transform.Resize(im.m, int(w), int(h), transform.Box)
	return nil
}

// Grayscale transforms the image to a grayscale version.
func (im *NativeImage) Grayscale() error {
	im.m = effect.Grayscale(im.m)
	return nil
}

// Gray16 transforms the image to a 16 gray levels palette,
// applying a Floyd Steinberg dithering. It is better to
// convert the image to grayscale before this operation.
func (im *NativeImage) Gray16() error {
	b := im.m.Bounds()

	pm := image.NewPaletted(b, Gray16Palette)
	draw.FloydSteinberg.Draw(pm, b, im.m, b.Min)
	im.m = pm
	return nil
}

// Clean is a noop function for this image family.
func (im *NativeImage) Clean() error {
	return nil
}

// Encode encodes the image to an io.Writer.
func (im *NativeImage) Encode(w io.Writer) error {
	if im.encFormat == "" {
		im.encFormat = im.format
	}

	switch {
	case im.encFormat == "gif":
		m, ok := im.m.(*image.Paletted)
		numColors := 256
		if ok {
			numColors = len(m.Palette)
		}
		im.format = "gif"
		return gif.Encode(w, im.m, &gif.Options{
			NumColors: numColors,
		})
	case im.encFormat == "png" || im.format == "ico" || im.lossless:
		c := png.BestSpeed
		if im.compression == CompressionBest {
			c = png.BestCompression
		}
		encoder := &png.Encoder{CompressionLevel: c}
		im.format = "png"
		return encoder.Encode(w, im.m)
	}

	// Default to jpeg encoding
	im.format = "jpeg"
	return jpeg.Encode(w, im.m, &jpeg.Options{Quality: int(im.quality)})
}
