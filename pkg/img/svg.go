// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package img

import (
	"fmt"
	"image"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
)

func init() {
	AddImageHandler(
		func(r io.Reader) (Image, error) {
			return NewSvgImage(r)
		},
		"image/svg+xml",
	)
}

var reViewBox = regexp.MustCompile(`[\s,]+`)

// SvgImage is the Image implementation for SVG images.
type SvgImage struct {
	node   *xmlquery.Node
	bounds image.Rectangle
}

// NewSvgImage returns an SvgImage instance.
func NewSvgImage(r io.Reader) (*SvgImage, error) {
	node, err := xmlquery.Parse(r)
	if err != nil {
		return nil, err
	}

	return &SvgImage{
		node:   node,
		bounds: getSVGDimensions(node),
	}, nil
}

// Close is a noop here.
func (im *SvgImage) Close() error {
	return nil
}

// Format returns the image format.
func (im *SvgImage) Format() string {
	return "svg"
}

// ContentType returns the image mimetype.
func (im *SvgImage) ContentType() string {
	return "image/svg+xml"
}

// Width returns the image width.
func (im *SvgImage) Width() uint {
	return uint(im.bounds.Dx())
}

// Height returns the image height.
func (im *SvgImage) Height() uint {
	return uint(im.bounds.Dy())
}

// SetFormat is a noop here.
func (im *SvgImage) SetFormat(string) error {
	return nil
}

// SetCompression is a noop here.
func (im *SvgImage) SetCompression(ImageCompression) error {
	return nil
}

// SetQuality is a noop here.
func (im *SvgImage) SetQuality(uint8) error {
	return nil
}

// Grayscale is a noop here.
func (im *SvgImage) Grayscale() error {
	return nil
}

// Gray16 is a noop here.
func (im *SvgImage) Gray16() error {
	return nil
}

// Resize resizes the image to the given width and height.
func (im *SvgImage) Resize(w, h uint) error {
	im.bounds.Max.X = int(w)
	im.bounds.Max.Y = int(h)

	node := xmlquery.FindOne(im.node, "/svg")
	if node != nil {
		node.SetAttr("width", strconv.Itoa(int(im.Width())))
		node.SetAttr("height", strconv.Itoa(int(im.Height())))
	}

	return nil
}

// Encode encodes the image to an io.Writer.
func (im *SvgImage) Encode(w io.Writer) (string, error) {
	_, err := w.Write([]byte(im.node.OutputXMLWithOptions(
		xmlquery.WithOutputSelf(),
		xmlquery.WithEmptyTagSupport(),
	)))
	return "image/svg+xml", err
}

// Clean sanitizes the SVG image by keeping only a specific set of tags and attributes.
func (im *SvgImage) Clean() error {
	for _, node := range xmlquery.Find(im.node, "//*") {
		if node.Type != xmlquery.ElementNode {
			continue
		}

		f, ok := allowedSVGTags[xmlName{node.NamespaceURI, node.Data}]
		if !ok || !f(node) {
			xmlquery.RemoveFromTree(node)
			continue
		}
	}

	return nil
}

const (
	xmlNS   = "http://www.w3.org/XML/1998/namespace"
	svgNS   = "http://www.w3.org/2000/svg"
	xlinkNS = "http://www.w3.org/1999/xlink"
)

type xmlName struct {
	NS, Local string
}

func getSVGDimensions(top *xmlquery.Node) (rect image.Rectangle) {
	rect = image.Rectangle{}

	// Default values when no dimension information can be found
	rect.Min.X = 0
	rect.Min.Y = 0
	rect.Max.X = 300
	rect.Max.Y = 150

	node := xmlquery.FindOne(top, "/svg")
	if node == nil {
		return
	}

	// We have a width and height, it gets priority
	w := cssDimensionToPixels(node.SelectAttr("width"))
	h := cssDimensionToPixels(node.SelectAttr("height"))
	if w > 0 && h > 0 {
		rect.Max.X = w
		rect.Max.Y = h
		return
	}

	viewBox := node.SelectAttr("viewBox")
	parts := reViewBox.Split(viewBox, -1)
	if len(parts) != 4 {
		return
	}

	vbw := svgNumberToInt(parts[2])
	vbh := svgNumberToInt(parts[3])
	if vbw == 0 || vbh == 0 {
		return
	}

	// Fixed width and viewbox
	if w > 0 {
		rect.Max.X = w
		rect.Max.Y = w / vbw * vbh
		return
	}

	// Fixed height and viewbox
	if h > 0 {
		rect.Max.X = h / vbh * vbw
		rect.Max.Y = h
		return
	}

	rect.Max.X = vbw
	rect.Max.Y = vbh

	return
}

const (
	// The default font size in web browsers is 16px.
	cssEmDefaultSize = 16
	// Size of ex is roughly 50% of em.
	cssExDefaultSize = 8
)

func cssDimensionToPixels(v string) int {
	if strings.HasSuffix(v, "%") {
		px, _ := strconv.Atoi(strings.TrimSuffix(v, "%"))
		return px
	}

	// https://developer.mozilla.org/en-US/docs/Web/CSS/length
	multiplier := 1
	switch {
	case strings.HasSuffix(v, "rem"):
		v = strings.TrimSuffix(v, "rem")
		multiplier = cssEmDefaultSize
	case strings.HasSuffix(v, "rex"):
		v = strings.TrimSuffix(v, "rex")
		multiplier = cssExDefaultSize
	case strings.HasSuffix(v, "em"):
		v = strings.TrimSuffix(v, "em")
		multiplier = cssEmDefaultSize
	case strings.HasSuffix(v, "ex"):
		v = strings.TrimSuffix(v, "ex")
		multiplier = cssExDefaultSize
	case strings.HasSuffix(v, "px"):
		v = strings.TrimSuffix(v, "px")

	}

	vv, _ := strconv.ParseFloat(v, 32)
	return int(math.Round(vv * float64(multiplier)))
}

// https://developer.mozilla.org/en-US/docs/Web/SVG/Guides/Content_type#number
func svgNumberToInt(v string) int {
	vv, _ := strconv.ParseFloat(v, 32)
	return int(math.Floor(vv))
}

type svgTagCleaner func(*xmlquery.Node) bool

func svgTagCleanup(node *xmlquery.Node) bool {
	for _, x := range node.Attr {
		if _, ok := allowedSVGAttributes[xmlName{x.NamespaceURI, x.Name.Local}]; !ok {
			node.RemoveAttr(fmt.Sprintf("%s:%s", x.Name.Space, x.Name.Local))
		}
	}
	return true
}

func svgUseTagCleanup(node *xmlquery.Node) bool {
	svgTagCleanup(node)

	attrs := []xmlquery.Attr{}
	hasValidHref := false

	for _, x := range node.Attr {
		if x.Name.Local == "href" && (x.Name.Space == "" || x.NamespaceURI == xlinkNS) {
			// Remove href and xlink:href attributes that don't refer to internal IDs
			if strings.HasPrefix(x.Value, "#") {
				attrs = append(attrs, x)
				hasValidHref = true
			}
			continue
		}
		attrs = append(attrs, x)
	}

	node.Attr = attrs

	return hasValidHref
}

var allowedSVGTags = map[xmlName]svgTagCleaner{
	{svgNS, "svg"}:                 svgTagCleanup,
	{svgNS, "a"}:                   svgTagCleanup,
	{svgNS, "animateMotion"}:       svgTagCleanup,
	{svgNS, "animateTransform"}:    svgTagCleanup,
	{svgNS, "circle"}:              svgTagCleanup,
	{svgNS, "clipPath"}:            svgTagCleanup,
	{svgNS, "defs"}:                svgTagCleanup,
	{svgNS, "desc"}:                svgTagCleanup,
	{svgNS, "ellipse"}:             svgTagCleanup,
	{svgNS, "feBlend"}:             svgTagCleanup,
	{svgNS, "feColorMatrix"}:       svgTagCleanup,
	{svgNS, "feComponentTransfer"}: svgTagCleanup,
	{svgNS, "feComposite"}:         svgTagCleanup,
	{svgNS, "feConvolveMatrix"}:    svgTagCleanup,
	{svgNS, "feDiffuseLighting"}:   svgTagCleanup,
	{svgNS, "feDisplacementMap"}:   svgTagCleanup,
	{svgNS, "feDistantLight"}:      svgTagCleanup,
	{svgNS, "feDropShadow"}:        svgTagCleanup,
	{svgNS, "feFlood"}:             svgTagCleanup,
	{svgNS, "feFuncA"}:             svgTagCleanup,
	{svgNS, "feFuncB"}:             svgTagCleanup,
	{svgNS, "feFuncG"}:             svgTagCleanup,
	{svgNS, "feFuncR"}:             svgTagCleanup,
	{svgNS, "feGaussianBlur"}:      svgTagCleanup,
	{svgNS, "feImage"}:             svgTagCleanup,
	{svgNS, "feMerge"}:             svgTagCleanup,
	{svgNS, "feMergeNode"}:         svgTagCleanup,
	{svgNS, "feMorphology"}:        svgTagCleanup,
	{svgNS, "feOffset"}:            svgTagCleanup,
	{svgNS, "fePointLight"}:        svgTagCleanup,
	{svgNS, "feSpecularLighting"}:  svgTagCleanup,
	{svgNS, "feSpotLight"}:         svgTagCleanup,
	{svgNS, "feTile"}:              svgTagCleanup,
	{svgNS, "feTurbulence"}:        svgTagCleanup,
	{svgNS, "filter"}:              svgTagCleanup,
	{svgNS, "font"}:                svgTagCleanup,
	{svgNS, "g"}:                   svgTagCleanup,
	{svgNS, "glyph"}:               svgTagCleanup,
	{svgNS, "glyphRef"}:            svgTagCleanup,
	{svgNS, "hkern"}:               svgTagCleanup,
	{svgNS, "image"}:               svgTagCleanup,
	{svgNS, "line"}:                svgTagCleanup,
	{svgNS, "linearGradient"}:      svgTagCleanup,
	{svgNS, "marker"}:              svgTagCleanup,
	{svgNS, "mask"}:                svgTagCleanup,
	{svgNS, "metadata"}:            svgTagCleanup,
	{svgNS, "mpath"}:               svgTagCleanup,
	{svgNS, "path"}:                svgTagCleanup,
	{svgNS, "pattern"}:             svgTagCleanup,
	{svgNS, "polygon"}:             svgTagCleanup,
	{svgNS, "polyline"}:            svgTagCleanup,
	{svgNS, "radialGradient"}:      svgTagCleanup,
	{svgNS, "rect"}:                svgTagCleanup,
	{svgNS, "stop"}:                svgTagCleanup,
	{svgNS, "style"}:               svgTagCleanup,
	{svgNS, "switch"}:              svgTagCleanup,
	{svgNS, "symbol"}:              svgTagCleanup,
	{svgNS, "text"}:                svgTagCleanup,
	{svgNS, "textPath"}:            svgTagCleanup,
	{svgNS, "title"}:               svgTagCleanup,
	{svgNS, "tref"}:                svgTagCleanup,
	{svgNS, "tspan"}:               svgTagCleanup,
	{svgNS, "use"}:                 svgUseTagCleanup,
	{svgNS, "view"}:                svgTagCleanup,
	{svgNS, "vkern"}:               svgTagCleanup,
}

var allowedSVGAttributes = map[xmlName]struct{}{
	{"xmlns", "svg"}:   {},
	{"xmlns", "xlink"}: {},

	{svgNS, "accent-height"}:               {},
	{svgNS, "accumulate"}:                  {},
	{svgNS, "additive"}:                    {},
	{svgNS, "alignment-baseline"}:          {},
	{svgNS, "ascent"}:                      {},
	{svgNS, "attributeName"}:               {},
	{svgNS, "attributeType"}:               {},
	{svgNS, "azimuth"}:                     {},
	{svgNS, "baseFrequency"}:               {},
	{svgNS, "baseline-shift"}:              {},
	{svgNS, "begin"}:                       {},
	{svgNS, "bias"}:                        {},
	{svgNS, "by"}:                          {},
	{svgNS, "class"}:                       {},
	{svgNS, "clip"}:                        {},
	{svgNS, "clip-path"}:                   {},
	{svgNS, "clip-rule"}:                   {},
	{svgNS, "clipPathUnits"}:               {},
	{svgNS, "color"}:                       {},
	{svgNS, "color-interpolation"}:         {},
	{svgNS, "color-interpolation-filters"}: {},
	{svgNS, "color-profile"}:               {},
	{svgNS, "color-rendering"}:             {},
	{svgNS, "cx"}:                          {},
	{svgNS, "cy"}:                          {},
	{svgNS, "d"}:                           {},
	{svgNS, "dx"}:                          {},
	{svgNS, "dy"}:                          {},
	{svgNS, "diffuseConstant"}:             {},
	{svgNS, "direction"}:                   {},
	{svgNS, "display"}:                     {},
	{svgNS, "divisor"}:                     {},
	{svgNS, "dur"}:                         {},
	{svgNS, "edgeMode"}:                    {},
	{svgNS, "elevation"}:                   {},
	{svgNS, "end"}:                         {},
	{svgNS, "fill"}:                        {},
	{svgNS, "fill-opacity"}:                {},
	{svgNS, "fill-rule"}:                   {},
	{svgNS, "filter"}:                      {},
	{svgNS, "filterUnits"}:                 {},
	{svgNS, "flood-color"}:                 {},
	{svgNS, "flood-opacity"}:               {},
	{svgNS, "font-family"}:                 {},
	{svgNS, "font-size"}:                   {},
	{svgNS, "font-size-adjust"}:            {},
	{svgNS, "font-stretch"}:                {},
	{svgNS, "font-style"}:                  {},
	{svgNS, "font-variant"}:                {},
	{svgNS, "font-weight"}:                 {},
	{svgNS, "fx"}:                          {},
	{svgNS, "fy"}:                          {},
	{svgNS, "g1"}:                          {},
	{svgNS, "g2"}:                          {},
	{svgNS, "glyph-name"}:                  {},
	{svgNS, "glyphRef"}:                    {},
	{svgNS, "gradientUnits"}:               {},
	{svgNS, "gradientTransform"}:           {},
	{svgNS, "height"}:                      {},
	{svgNS, "href"}:                        {},
	{svgNS, "id"}:                          {},
	{svgNS, "image-rendering"}:             {},
	{svgNS, "in"}:                          {},
	{svgNS, "in2"}:                         {},
	{svgNS, "k"}:                           {},
	{svgNS, "k1"}:                          {},
	{svgNS, "k2"}:                          {},
	{svgNS, "k3"}:                          {},
	{svgNS, "k4"}:                          {},
	{svgNS, "kerning"}:                     {},
	{svgNS, "keyPoints"}:                   {},
	{svgNS, "keySplines"}:                  {},
	{svgNS, "keyTimes"}:                    {},
	{svgNS, "lang"}:                        {},
	{svgNS, "lengthAdjust"}:                {},
	{svgNS, "letter-spacing"}:              {},
	{svgNS, "kernelMatrix"}:                {},
	{svgNS, "kernelUnitLength"}:            {},
	{svgNS, "lighting-color"}:              {},
	{svgNS, "local"}:                       {},
	{svgNS, "marker-end"}:                  {},
	{svgNS, "marker-mid"}:                  {},
	{svgNS, "marker-start"}:                {},
	{svgNS, "markerHeight"}:                {},
	{svgNS, "markerUnits"}:                 {},
	{svgNS, "markerWidth"}:                 {},
	{svgNS, "maskContentUnits"}:            {},
	{svgNS, "maskUnits"}:                   {},
	{svgNS, "max"}:                         {},
	{svgNS, "mask"}:                        {},
	{svgNS, "media"}:                       {},
	{svgNS, "method"}:                      {},
	{svgNS, "mode"}:                        {},
	{svgNS, "min"}:                         {},
	{svgNS, "name"}:                        {},
	{svgNS, "numOctaves"}:                  {},
	{svgNS, "offset"}:                      {},
	{svgNS, "operator"}:                    {},
	{svgNS, "opacity"}:                     {},
	{svgNS, "order"}:                       {},
	{svgNS, "orient"}:                      {},
	{svgNS, "orientation"}:                 {},
	{svgNS, "origin"}:                      {},
	{svgNS, "overflow"}:                    {},
	{svgNS, "paint-order"}:                 {},
	{svgNS, "path"}:                        {},
	{svgNS, "pathLength"}:                  {},
	{svgNS, "patternContentUnits"}:         {},
	{svgNS, "patternTransform"}:            {},
	{svgNS, "patternUnits"}:                {},
	{svgNS, "points"}:                      {},
	{svgNS, "preserveAlpha"}:               {},
	{svgNS, "preserveAspectRatio"}:         {},
	{svgNS, "primitiveUnits"}:              {},
	{svgNS, "r"}:                           {},
	{svgNS, "rx"}:                          {},
	{svgNS, "ry"}:                          {},
	{svgNS, "radius"}:                      {},
	{svgNS, "refX"}:                        {},
	{svgNS, "refY"}:                        {},
	{svgNS, "repeatCount"}:                 {},
	{svgNS, "repeatDur"}:                   {},
	{svgNS, "restart"}:                     {},
	{svgNS, "result"}:                      {},
	{svgNS, "rotate"}:                      {},
	{svgNS, "scale"}:                       {},
	{svgNS, "seed"}:                        {},
	{svgNS, "shape-rendering"}:             {},
	{svgNS, "specularConstant"}:            {},
	{svgNS, "specularExponent"}:            {},
	{svgNS, "spreadMethod"}:                {},
	{svgNS, "startOffset"}:                 {},
	{svgNS, "stdDeviation"}:                {},
	{svgNS, "stitchTiles"}:                 {},
	{svgNS, "stop-color"}:                  {},
	{svgNS, "stop-opacity"}:                {},
	{svgNS, "stroke-dasharray"}:            {},
	{svgNS, "stroke-dashoffset"}:           {},
	{svgNS, "stroke-linecap"}:              {},
	{svgNS, "stroke-linejoin"}:             {},
	{svgNS, "stroke-miterlimit"}:           {},
	{svgNS, "stroke-opacity"}:              {},
	{svgNS, "stroke"}:                      {},
	{svgNS, "stroke-width"}:                {},
	{svgNS, "style"}:                       {},
	{svgNS, "surfaceScale"}:                {},
	{svgNS, "systemLanguage"}:              {},
	{svgNS, "tabindex"}:                    {},
	{svgNS, "targetX"}:                     {},
	{svgNS, "targetY"}:                     {},
	{svgNS, "transform"}:                   {},
	{svgNS, "transform-origin"}:            {},
	{svgNS, "text-anchor"}:                 {},
	{svgNS, "text-decoration"}:             {},
	{svgNS, "text-rendering"}:              {},
	{svgNS, "textLength"}:                  {},
	{svgNS, "type"}:                        {},
	{svgNS, "u1"}:                          {},
	{svgNS, "u2"}:                          {},
	{svgNS, "unicode"}:                     {},
	{svgNS, "values"}:                      {},
	{svgNS, "viewBox"}:                     {},
	{svgNS, "visibility"}:                  {},
	{svgNS, "version"}:                     {},
	{svgNS, "vert-adv-y"}:                  {},
	{svgNS, "vert-origin-x"}:               {},
	{svgNS, "vert-origin-y"}:               {},
	{svgNS, "width"}:                       {},
	{svgNS, "word-spacing"}:                {},
	{svgNS, "writing-mode"}:                {},
	{svgNS, "xChannelSelector"}:            {},
	{svgNS, "yChannelSelector"}:            {},
	{svgNS, "x"}:                           {},
	{svgNS, "x1"}:                          {},
	{svgNS, "x2"}:                          {},
	{svgNS, "y"}:                           {},
	{svgNS, "y1"}:                          {},
	{svgNS, "y2"}:                          {},
	{svgNS, "z"}:                           {},
	{svgNS, "zoomAndPan"}:                  {},

	{xmlNS, "id"}:    {},
	{xmlNS, "space"}: {},

	{xlinkNS, "href"}:  {},
	{xlinkNS, "title"}: {},
}
