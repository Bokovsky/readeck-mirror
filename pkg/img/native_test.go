// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package img_test

import (
	"bytes"
	"image"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/img"
)

func TestNativeImageProps(t *testing.T) {
	assert := require.New(t)
	r := bytes.NewReader(newImage(200, 100))
	im, err := img.NewNativeImage(r)
	assert.NoError(err)

	assert.Equal("png", im.Format())
	assert.Equal("image/png", im.ContentType())
	assert.Equal(uint(200), im.Width())
	assert.Equal(uint(100), im.Height())

	m := im.Image()
	assert.Equal(200, m.Bounds().Dx())
	assert.Equal(100, m.Bounds().Dy())
}

func TestNativeImageLoadError(t *testing.T) {
	assert := require.New(t)
	im, err := img.NewNativeImage(bytes.NewReader([]byte{}))
	assert.Nil(im)
	assert.Error(err)
	assert.EqualError(err, "image: unknown format")
}

func TestNativeImageTooBig(t *testing.T) {
	assert := require.New(t)
	r := bytes.NewReader(newImage(6001, 5000))
	im, err := img.NewNativeImage(r)
	assert.Nil(im)
	assert.Error(err)
	assert.EqualError(err, "image is too big")
}

func TestNativeImageResize(t *testing.T) {
	assert := require.New(t)
	r := bytes.NewReader(newImage(200, 100))
	im, err := img.NewNativeImage(r)
	assert.NoError(err)
	assert.NoError(im.Resize(400, 200))

	buf := new(bytes.Buffer)
	ct, err := im.Encode(buf)
	assert.NoError(err)
	assert.Equal("image/png", ct)

	p, _, err := image.Decode(buf)
	assert.NoError(err)
	assert.Equal(400, p.Bounds().Dx())
	assert.Equal(200, p.Bounds().Dy())
}

func TestNativeImageEncode(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{"png", "image/png"},
		{"", "image/png"},
		{"jpeg", "image/jpeg"},
		{"gif", "image/gif"},
		{"bmp", "image/jpeg"},
	}

	data := newImage(200, 100)
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert := require.New(t)
			im, err := img.NewNativeImage(bytes.NewReader(data))
			assert.NoError(err)

			assert.NoError(im.SetFormat(test.format))

			buf := new(bytes.Buffer)
			ct, err := im.Encode(buf)
			assert.NoError(err)
			assert.Equal(test.expected, ct)

			im, err = img.NewNativeImage(buf)
			assert.NoError(err)
			assert.Equal(test.expected, im.ContentType())
		})
	}
}

func TestWebpLossless(t *testing.T) {
	tests := []struct {
		file     string
		expected string
	}{
		{"fixtures/lossless.webp", "image/png"},
		{"fixtures/lossy.webp", "image/jpeg"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert := require.New(t)
			r, err := os.Open(test.file)
			assert.NoError(err)
			im, err := img.NewNativeImage(r)
			assert.NoError(err)

			buf := new(bytes.Buffer)
			ct, err := im.Encode(buf)
			assert.NoError(err)
			assert.Equal(test.expected, ct)

			im, err = img.NewNativeImage(buf)
			assert.NoError(err)
			assert.Equal(test.expected, im.ContentType())
		})
	}
}

func TestNativeImageOperations(t *testing.T) {
	tests := []func(*img.NativeImage) error{
		func(im *img.NativeImage) error { return im.Clean() },
		func(im *img.NativeImage) error { return im.SetCompression(img.CompressionBest) },
		func(im *img.NativeImage) error { return im.SetQuality(1) },
	}

	data := newImage(200, 100)
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert := require.New(t)
			im, err := img.NewNativeImage(bytes.NewReader(data))
			assert.NoError(err)
			assert.NoError(test(im))
		})
	}
}
