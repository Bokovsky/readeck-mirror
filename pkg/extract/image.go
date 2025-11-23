// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"codeberg.org/readeck/readeck/pkg/img"
	"github.com/gabriel-vasile/mimetype"
)

const (
	// ImageSizeThumbnail is the width of a regular thumbnail image.
	ImageSizeThumbnail = 800

	// ImageSizeWide is the width of a bigger image.
	// Its dimension matches 48rem (on a 16px basis)
	// on an HDPI screen: 48 × 16px × 2.
	ImageSizeWide = 1536
)

// NewRemoteImage loads an image and returns a new img.Image instance.
// If the image is a GIF, it returns its first frame only.
func NewRemoteImage(ctx context.Context, client *http.Client, src string) (img.Image, error) {
	if client == nil {
		client = http.DefaultClient
	}

	if src == "" {
		return nil, errors.New("No image URL")
	}

	// Send the request with a specific Accept header
	header, _ := CheckRequestHeader(ctx)
	if header == nil {
		header = http.Header{}
	}
	header.Set("Accept", "image/webp,image/svg+xml,image/*,*/*;q=0.8")
	ctx = WithRequestHeader(ctx, header)
	ctx = WithRequestType(ctx, ImageRequest)

	rsp, err := Fetch(ctx, client, src)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close() //nolint:errcheck

	if rsp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Invalid response status (%d)", rsp.StatusCode)
	}

	buf := new(bytes.Buffer)
	mtype, err := mimetype.DetectReader(io.TeeReader(rsp.Body, buf))
	if err != nil {
		return nil, err
	}

	im, err := img.New(mtype.String(), io.MultiReader(buf, rsp.Body))
	if err != nil {
		return nil, err
	}

	// Image is a GIF, use its first frame only.
	if x, ok := im.(img.MultiFrameImage); ok {
		im = x.FirstFrame()
	}
	return im, nil
}

// Picture is a remote picture.
type Picture struct {
	Href   string
	Type   string
	Size   [2]int
	format string
	bytes  []byte
}

// NewPicture returns a new Picture instance from a given
// URL and its base.
func NewPicture(src string, base *url.URL) (*Picture, error) {
	href, err := base.Parse(src)
	if err != nil {
		return nil, err
	}

	return &Picture{
		Href: href.String(),
	}, nil
}

// Load loads the image remotely and fit it into the given
// boundaries size.
func (p *Picture) Load(ctx context.Context, client *http.Client, size uint, toFormat string) error {
	ri, err := NewRemoteImage(ctx, client, p.Href)
	if err != nil {
		return err
	}
	defer ri.Close() //nolint:errcheck

	err = img.Pipeline(ri, pClean, pComp, pQual, pFit(size, 0), pFormat(toFormat))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	ct, err := ri.Encode(&buf)
	if err != nil {
		return err
	}

	p.bytes = buf.Bytes()
	p.Size = [2]int{int(ri.Width()), int(ri.Height())}
	p.Type = ct
	p.format = ri.Format()
	return nil
}

// Copy returns a resized copy of the image, as a new Picture instance.
func (p *Picture) Copy(size uint, toFormat string) (*Picture, error) {
	ri, err := img.New(p.Type, bytes.NewReader(p.bytes))
	if err != nil {
		return nil, err
	}
	defer ri.Close() //nolint:errcheck

	res := &Picture{Href: p.Href}
	err = img.Pipeline(ri, pClean, pComp, pQual, pFit(size, 0), pFormat(toFormat))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	ct, err := ri.Encode(&buf)
	if err != nil {
		return nil, err
	}

	res.bytes = buf.Bytes()
	res.Size = [2]int{int(ri.Width()), int(ri.Height())}
	res.Type = ct
	res.format = ri.Format()
	return res, nil
}

// Name returns the given name of the picture with the correct
// extension.
func (p *Picture) Name(name string) string {
	return fmt.Sprintf("%s.%s", name, p.format)
}

// Bytes returns the image data.
func (p *Picture) Bytes() []byte {
	return p.bytes
}

// Encoded returns a base64 encoded string of the image.
func (p *Picture) Encoded() string {
	if len(p.bytes) == 0 {
		return ""
	}

	return base64.StdEncoding.EncodeToString(p.bytes)
}

func pFormat(f string) img.ImageFilter {
	return func(im img.Image) error {
		return im.SetFormat(f)
	}
}

func pFit(w, h uint) img.ImageFilter {
	return func(im img.Image) error {
		return img.Fit(im, w, h)
	}
}

func pComp(im img.Image) error {
	return im.SetCompression(img.CompressionBest)
}

func pClean(im img.Image) error {
	return im.Clean()
}

func pQual(im img.Image) error {
	return im.SetQuality(75)
}
