// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bleach

import "strings"

var keepEmptyTags = map[string]struct{}{
	"area":        {},
	"base":        {},
	"br":          {},
	"col":         {},
	"command":     {},
	"embed":       {},
	"hr":          {},
	"img":         {},
	"input":       {},
	"keygen":      {},
	"link":        {},
	"menuitem":    {},
	"meta":        {},
	"mprescripts": {},
	"mrow":        {},
	"mspace":      {},
	"mtd":         {},
	"param":       {},
	"source":      {},
	"th":          {},
	"td":          {},
	"track":       {},
	"wbr":         {},
}

var blockTags = map[string]struct{}{
	"address":    {},
	"article":    {},
	"aside":      {},
	"blockquote": {},
	"canvas":     {},
	"dd":         {},
	"div":        {},
	"dl":         {},
	"dt":         {},
	"fieldset":   {},
	"figcaption": {},
	"figure":     {},
	"footer":     {},
	"form":       {},
	"h1":         {},
	"h2":         {},
	"h3":         {},
	"h4":         {},
	"h5":         {},
	"h6":         {},
	"header":     {},
	"hr":         {},
	"li":         {},
	"main":       {},
	"nav":        {},
	"noscript":   {},
	"ol":         {},
	"p":          {},
	"pre":        {},
	"section":    {},
	"table":      {},
	"tfoot":      {},
	"ul":         {},
	"video":      {},
}

// elementMap is the map of all known elements
// and what they can be transformed to.
// A value of "-" means the elements must be removed.
// As per https://developer.mozilla.org/en-US/docs/Web/HTML/Element
var elementMap = map[string]string{
	"a":          "",
	"abbr":       "",
	"acronym":    "",
	"address":    "",
	"applet":     "-", // remove
	"area":       "",
	"article":    "",
	"aside":      "",
	"audio":      "-", // remove
	"b":          "",
	"base":       "-", // remove
	"bdi":        "",
	"bdo":        "",
	"big":        "",
	"blockquote": "",
	"body":       "",
	"br":         "",
	"button":     "-", // remove
	"canvas":     "-", // remove
	"caption":    "",
	"center":     "",
	"cite":       "",
	"code":       "",
	"col":        "",
	"colgroup":   "",
	"data":       "",
	"datalist":   "",
	"dd":         "",
	"del":        "",
	"details":    "",
	"dfn":        "",
	"dialog":     "-", // remove
	"dir":        "",
	"div":        "",
	"dl":         "",
	"dt":         "",
	"em":         "",
	"embed":      "-", // remove
	"fieldset":   "div",
	"figcaption": "",
	"figure":     "",
	"font":       "span",
	"footer":     "",
	"form":       "div",
	"frame":      "-", // remove
	"frameset":   "-", // remove
	"h1":         "",
	"h2":         "",
	"h3":         "",
	"h4":         "",
	"h5":         "",
	"h6":         "",
	"head":       "-", // remove
	"header":     "",
	"hgroup":     "",
	"hr":         "",
	"html":       "",
	"i":          "",
	"iframe":     "-", // remove
	"image":      "",
	"img":        "",
	"input":      "-", // remove
	"ins":        "",
	"kbd":        "",
	"label":      "",
	"legend":     "",
	"li":         "",
	"link":       "-", // remove
	"main":       "",
	"map":        "",
	"mark":       "",
	"marquee":    "",
	"menu":       "",
	"menuitem":   "",
	"meta":       "-", // remove
	"meter":      "",
	"nav":        "",
	"nobr":       "",
	"noembed":    "div",
	"noframes":   "div",
	"noscript":   "div",
	"object":     "-", // remove
	"ol":         "",
	"optgroup":   "",
	"option":     "",
	"output":     "",
	"p":          "",
	"param":      "-", // remove
	"picture":    "",
	"plaintext":  "",
	"portal":     "-", // remove
	"pre":        "",
	"progress":   "",
	"q":          "",
	"rb":         "",
	"rp":         "",
	"rt":         "",
	"rtc":        "",
	"ruby":       "",
	"s":          "",
	"samp":       "",
	"script":     "-", // remove
	"search":     "",
	"section":    "",
	"select":     "-", // remove
	"slot":       "-", // remove
	"small":      "",
	"source":     "-", // remove
	"span":       "",
	"strike":     "",
	"strong":     "",
	"style":      "-", // remove
	"sub":        "",
	"summary":    "",
	"sup":        "",
	"table":      "",
	"tbody":      "",
	"td":         "",
	"template":   "-", // remove
	"textarea":   "-", // remove
	"tfoot":      "",
	"th":         "",
	"thead":      "",
	"time":       "",
	"title":      "-", // remove
	"tr":         "",
	"track":      "-", // remove
	"tt":         "",
	"u":          "",
	"ul":         "",
	"var":        "",
	"video":      "-", // remove
	"wbr":        "",
	"xmp":        "",

	// MathML Core elements
	// curl -fsSL https://www.w3.org/Math/RelaxNG/mathml4/mathml4-core.rnc | grep -oE 'element [a-z-]+' | cut -d' ' -f2 | grep -v '^none$' | sort
	"annotation":     "",
	"annotation-xml": "",
	"maction":        "",
	"math":           "",
	"merror":         "",
	"mfrac":          "",
	"mi":             "",
	"mmultiscripts":  "",
	"mn":             "",
	"mo":             "",
	"mover":          "",
	"mpadded":        "",
	"mphantom":       "",
	"mprescripts":    "",
	"mroot":          "",
	"mrow":           "",
	"ms":             "",
	"mspace":         "",
	"msqrt":          "",
	"mstyle":         "",
	"msub":           "",
	"msubsup":        "",
	"msup":           "",
	"mtable":         "",
	"mtd":            "",
	"mtext":          "",
	"mtr":            "",
	"munder":         "",
	"munderover":     "",
	"semantics":      "",
}

var excludedChars = [][2]int{
	// CO block, except 0x09 (tab), 0x0A (LF), 0x0D (CR)
	{0x00, 0x08},
	{0x0B, 0x0C},
	{0x0E, 0x1F},

	// C1 block, except 0x85 (next line)
	{0x7F, 0x84},
	{0x86, 0x9F},

	// Surrogates
	{0xFDD0, 0xFDDF},
	{0xFFFE, 0xFFFF},

	{0x1FFFE, 0x1FFFF},
	{0x2FFFE, 0x2FFFF},
	{0x3FFFE, 0x3FFFF},
	{0x4FFFE, 0x4FFFF},
	{0x5FFFE, 0x5FFFF},
	{0x6FFFE, 0x6FFFF},
	{0x7FFFE, 0x7FFFF},
	{0x8FFFE, 0x8FFFF},
	{0x9FFFE, 0x9FFFF},
	{0xAFFFE, 0xAFFFF},
	{0xBFFFE, 0xBFFFF},
	{0xCFFFE, 0xCFFFF},
	{0xDFFFE, 0xDFFFF},
	{0xEFFFE, 0xEFFFF},
	{0xFFFFE, 0xFFFFF},
	{0x10FFFE, 0x10FFFF},
}

// ctrlReplacer is a string replacer for all invalid XML characters.
var ctrlReplacer = func() *strings.Replacer {
	repl := []string{}
	for _, t := range excludedChars {
		for i := t[0]; i <= t[1]; i++ {
			repl = append(repl, string(rune(i)), "")
		}
	}

	return strings.NewReplacer(repl...)
}()
