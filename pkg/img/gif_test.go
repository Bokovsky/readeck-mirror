// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package img_test

import (
	"bytes"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/img"
)

func TestAnimatedGIF(t *testing.T) {
	assert := require.New(t)
	r, err := os.Open("fixtures/animated.gif")
	assert.NoError(err)

	im, err := img.New("image/gif", r)
	assert.NoError(err)
	assert.IsType((*img.GIFImage)(nil), im)
	assert.Implements((*img.MultiFrameImage)(nil), im)

	assert.Equal(uint(131), im.Width())
	assert.Equal(uint(121), im.Height())
	assert.Equal("gif", im.Format())
	assert.NoError(im.SetFormat("gif"))
	assert.NoError(im.SetCompression(img.CompressionBest))
	assert.NoError(im.SetQuality(0))
	assert.NoError(im.Clean())

	assert.Equal(uint(22), im.(img.MultiFrameImage).Frames())

	assert.NoError(im.Resize(100, 100))

	w := new(bytes.Buffer)
	ct, err := im.Encode(w)
	assert.NoError(err)
	assert.Equal("image/gif", ct)

	c, err := gif.DecodeConfig(w)
	assert.NoError(err)
	assert.Equal(100, c.Width)
	assert.Equal(100, c.Height)

	ff := im.(img.MultiFrameImage).FirstFrame()
	assert.IsType((*img.GIFImage)(nil), ff)
	assert.Equal("gif", ff.Format())

	w.Reset()
	ct, err = ff.Encode(w)
	assert.NoError(err)
	assert.Equal("image/png", ct)

	c, err = png.DecodeConfig(w)
	assert.NoError(err)
	assert.IsType((color.Palette)(nil), c.ColorModel)

	assert.NoError(im.Close())
	assert.NoError(ff.Close())
}

func TestSingleFrameGIF(t *testing.T) {
	assert := require.New(t)
	r, err := os.Open("fixtures/image.gif")
	assert.NoError(err)

	im, err := img.New("image/gif", r)
	assert.NoError(err)
	assert.IsType((*img.GIFImage)(nil), im)
	assert.Implements((*img.MultiFrameImage)(nil), im)

	assert.Equal(uint(250), im.Width())
	assert.Equal(uint(297), im.Height())
	assert.Equal(uint(1), im.(img.MultiFrameImage).Frames())

	assert.NoError(im.Resize(125, 148))

	w := new(bytes.Buffer)
	assert.NoError(im.SetCompression(img.CompressionBest))
	ct, err := im.Encode(w)
	assert.NoError(err)
	assert.Equal("image/png", ct)

	c, err := png.DecodeConfig(w)
	assert.NoError(err)
	assert.Equal(125, c.Width)
	assert.Equal(148, c.Height)
	assert.IsType((color.Palette)(nil), c.ColorModel)
}
