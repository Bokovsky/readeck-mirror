// SPDX-FileCopyrightText: © 2020 Radhi Fadlillah
// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: MIT

package archiver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/sync/errgroup"

	"github.com/go-shiori/dom"
)

const (
	noScriptDataAttr  = "data-x-noscript"
	acceptImageHeader = "image/webp,image/svg+xml,image/*,*/*;q=0.8"
)

type (
	ctxNodeKey     struct{}
	ctxReferrerKey struct{}
)

func withNodeContext(ctx context.Context, node *html.Node) context.Context {
	return context.WithValue(ctx, ctxNodeKey{}, node)
}

// GetNodeContext returns an [html.Node] stored in context.
func GetNodeContext(ctx context.Context) *html.Node {
	if n, ok := ctx.Value(ctxNodeKey{}).(*html.Node); ok {
		return n
	}
	return nil
}

func withReferrerContext(ctx context.Context, uri string) context.Context {
	return context.WithValue(ctx, ctxReferrerKey{}, uri)
}

func getReferrerContext(ctx context.Context) string {
	s, _ := ctx.Value(ctxReferrerKey{}).(string)
	return s
}

var (
	rxLazyImageSrc    = regexp.MustCompile(`(?i)^\s*\S+\.(gif|jpg|jpeg|png|webp)\S*\s*$`)
	rxLazyImageSrcset = regexp.MustCompile(`(?i)\.(gif|jpg|jpeg|png|webp)\s+\d`)
	rxImgExtensions   = regexp.MustCompile(`(?i)\.(gif|jpg|jpeg|png|webp)`)
	rxSrcsetURL       = regexp.MustCompile(`(?i)(\S+)(\s+[\d.]+[xw])?(\s*(?:,|$))`)
	rxB64DataURL      = regexp.MustCompile(`(?i)^data:\s*([^\s;,]+)\s*;\s*base64\s*`)
)

// Note: we'll need this when implementing the HTML saving
// func (arc *Archiver) processHTMLReader(ctx context.Context, r io.Reader, baseURL *url.URL) error {
// 	doc, err := html.Parse(r)
// 	if err != nil {
// 		return err
// 	}

// 	return arc.processHTML(ctx, doc, baseURL)
// }

func (arc *Archiver) processHTML(ctx context.Context, doc *html.Node, baseURL *url.URL) error {
	arc.log().Info("start page archive", slog.String("url", baseURL.String()))

	ctx = withReferrerContext(ctx, baseURL.String())

	// Prepare document
	arc.setCharset(ctx, doc)
	arc.setContentSecurityPolicy(ctx, doc)
	arc.applyConfiguration(ctx, doc)
	arc.convertNoScriptToDiv(ctx, doc, true)
	arc.removePrefetch(ctx, doc)
	arc.removeComments(ctx, doc)
	arc.convertLazyImageAttrs(ctx, doc)
	arc.convertRelativeURLs(ctx, doc, baseURL)
	arc.removeIntegrityAttr(ctx, doc)

	if arc.flags&EnableBestImage > 0 {
		arc.processBestImages(ctx, doc)
	}

	// Find all nodes which might have subresource.
	// A node might has subresource if it fulfills one of these criteria :
	// - It has inline style;
	// - It's link for icon or stylesheets;
	// - It's tag name is either style, img, picture, figure, video, audio, source, iframe or object;
	resourceNodes := make(map[*html.Node]struct{})
	for _, node := range dom.GetElementsByTagName(doc, "*") {
		if style := dom.GetAttribute(node, "style"); strings.TrimSpace(style) != "" {
			resourceNodes[node] = struct{}{}
			continue
		}

		switch dom.TagName(node) {
		case "link":
			rel := dom.GetAttribute(node, "rel")
			if strings.Contains(rel, "icon") || strings.Contains(rel, "stylesheet") {
				resourceNodes[node] = struct{}{}
			}

		case "iframe", "embed", "object", "style", "script",
			"img", "use", "video", "audio", "source":
			resourceNodes[node] = struct{}{}
		}
	}

	// Process each node concurrently
	g, ctx := errgroup.WithContext(ctx)
	for node := range resourceNodes {
		g.Go(func() error {
			// Update style attribute
			if dom.HasAttribute(node, "style") {
				err := arc.processStyleAttr(ctx, node, baseURL)
				if err != nil {
					return err
				}
			}

			// Update node depending on its tag name
			switch dom.TagName(node) {
			case "style":
				return arc.processStyleNode(ctx, node, baseURL)
			case "link":
				return arc.processLinkNode(ctx, node)
			case "script":
				return arc.processScriptNode(ctx, node)
			case "object", "embed", "iframe":
				return arc.processEmbedNode(ctx, node)
			case "img", "use", "video", "audio", "source":
				return arc.processMediaNode(ctx, node)
			default:
				return nil
			}
		})
	}

	// Wait until all resources processed
	if err := g.Wait(); err != nil {
		return err
	}

	// Revert the converted noscripts
	arc.revertConvertedNoScript(ctx, doc)

	// Remove data attributes
	if arc.flags&EnableDataAttributes == 0 {
		arc.removeDataAttributes(ctx, doc)
	}

	arc.setLazyImages(ctx, doc)

	return nil
}

func (arc *Archiver) saveHTML(ctx context.Context, doc *html.Node, uri, name string) error {
	uri = requestURI(uri)
	if _, ok := arc.collector.Get(uri); ok {
		return nil
	}

	buf := bytes.NewBuffer(nil)
	buf.WriteString("<!DOCTYPE html>\n")
	html.Render(buf, doc)

	info := &Resource{
		url:         uri,
		Name:        name,
		ContentType: "text/html",
		Size:        int64(buf.Len()),
	}

	if info.Name == "" {
		info.Name = arc.collector.Name(uri) + ".html"
	}

	_, err := arc.saveResource(ctx, io.NopCloser(buf), info)
	return err
}

func (arc *Archiver) processURLNode(ctx context.Context, node *html.Node, attrName string) error {
	if !dom.HasAttribute(node, attrName) {
		return nil
	}

	uri := dom.GetAttribute(node, attrName)
	_, frag, _ := strings.Cut(uri, "#")
	headers := http.Header{}

	switch node.Data {
	case "img", "picture", "source", "use":
		headers.Set("accept", acceptImageHeader)
	}

	res, err := arc.processURL(withNodeContext(ctx, node), uri, processOptions{headers: headers})
	if err != nil {
		if errors.Is(err, errSkippedURL) {
			return nil
		}
		return err
	}

	newURI := res.Value()
	if frag != "" {
		newURI += "#" + frag
	}

	dom.SetAttribute(node, attrName, newURI)
	return nil
}

func (arc *Archiver) processStyleAttr(ctx context.Context, node *html.Node, baseURL *url.URL) error {
	style := dom.GetAttribute(node, "style")
	buf, err := arc.processCSS(
		withNodeContext(ctx, node), strings.NewReader(style),
		&Resource{url: baseURL.String()},
	)
	if err == nil {
		dom.SetAttribute(node, "style", buf.String())
	}

	return err
}

func (arc *Archiver) processStyleNode(ctx context.Context, node *html.Node, baseURL *url.URL) error {
	style := dom.TextContent(node)
	buf, err := arc.processCSS(
		withNodeContext(ctx, node), strings.NewReader(style),
		&Resource{url: baseURL.String()},
	)
	if err == nil {
		dom.SetTextContent(node, buf.String())
	}

	return err
}

func (arc *Archiver) processLinkNode(ctx context.Context, node *html.Node) error {
	if !dom.HasAttribute(node, "href") {
		return nil
	}

	rel := dom.GetAttribute(node, "rel")
	if strings.Contains(rel, "icon") || rel == "stylesheet" {
		return arc.processURLNode(withNodeContext(ctx, node), node, "href")
	}

	return nil
}

func (arc *Archiver) processScriptNode(ctx context.Context, node *html.Node) error {
	if !dom.HasAttribute(node, "src") {
		return nil
	}

	uri := dom.GetAttribute(node, "src")
	info, err := arc.processURL(withNodeContext(ctx, node), uri, processOptions{})
	if err != nil {
		if errors.Is(err, errSkippedURL) {
			return nil
		}
		return err
	}
	dom.SetAttribute(node, "src", info.Value())
	return nil
}

func (arc *Archiver) processEmbedNode(ctx context.Context, node *html.Node) error {
	attrName := "src"
	if dom.TagName(node) == "object" {
		attrName = "data"
	}

	if !dom.HasAttribute(node, attrName) {
		return nil
	}

	uri := dom.GetAttribute(node, attrName)
	info, err := arc.processURL(withNodeContext(ctx, node), uri, processOptions{})
	if err != nil {
		if errors.Is(err, errSkippedURL) {
			return nil
		}
		return err
	}

	dom.SetAttribute(node, attrName, info.Value())
	return nil
}

func (arc *Archiver) processMediaNode(ctx context.Context, node *html.Node) error {
	err := arc.processURLNode(ctx, node, "src")
	if err != nil {
		return err
	}

	if node.Data == "use" {
		return arc.processURLNode(ctx, node, "href")
	}

	err = arc.processURLNode(ctx, node, "poster")
	if err != nil {
		return err
	}

	if !dom.HasAttribute(node, "srcset") {
		return nil
	}

	var newSets []string
	srcset := dom.GetAttribute(node, "srcset")
	for _, parts := range rxSrcsetURL.FindAllStringSubmatch(srcset, -1) {
		res, err := arc.processURL(withNodeContext(ctx, node), parts[1], processOptions{
			headers: http.Header{"Accept": {"image/webp,image/svg+xml,image/*,*/*;q=0.8"}},
		})
		if err == nil {
			newSets = append(newSets, res.Value()+parts[2])
		}
	}

	dom.SetAttribute(node, "srcset", strings.Join(newSets, ","))
	return nil
}

func getHead(doc *html.Node) *html.Node {
	nodes := dom.GetElementsByTagName(doc, "head")
	if len(nodes) == 0 {
		head := dom.CreateElement("head")
		dom.AppendChild(doc, head)
		return head
	}
	return nodes[0]
}

func (arc *Archiver) setCharset(_ context.Context, doc *html.Node) {
	head := getHead(doc)

	// Change existing charset if any
	for _, meta := range dom.GetElementsByTagName(head, "meta") {
		if dom.GetAttribute(meta, "charset") != "" {
			dom.SetAttribute(meta, "charset", "utf-8")
			return
		}
	}

	// None found, create one
	meta := dom.CreateElement("meta")
	dom.SetAttribute(meta, "charset", "utf-8")
	dom.AppendChild(head, meta)
}

func (arc *Archiver) setContentSecurityPolicy(_ context.Context, doc *html.Node) {
	// Remove existing policy
	dom.RemoveNodes(dom.GetElementsByTagName(doc, "meta"), func(n *html.Node) bool {
		return strings.EqualFold(
			dom.GetAttribute(n, "http-equiv"),
			"content-security-policy",
		)
	})

	csp := []string{
		"default-src 'self' 'unsafe-inline' data:",
		"connect-src 'none'",
	}
	if arc.flags&EnableJS == 0 {
		csp = append(csp, "script-src 'none'")
	}

	if arc.flags&EnableCSS == 0 {
		csp = append(csp, "style-src 'none'")
	}

	if arc.flags&EnableEmbeds == 0 {
		csp = append(csp, "frame-src 'none'", "child-src 'none'")
	}

	if arc.flags&EnableImages == 0 {
		csp = append(csp, "image-src 'none'")
	}

	if arc.flags&EnableMedia == 0 {
		csp = append(csp, "media-src 'none'")
	}

	head := getHead(doc)
	meta := dom.CreateElement("meta")
	dom.SetAttribute(meta, "http-equiv", "Content-Security-Policy")
	dom.SetAttribute(meta, "content", strings.Join(csp, ";"))
	dom.PrependChild(head, meta)
}

func (arc *Archiver) applyConfiguration(ctx context.Context, doc *html.Node) {
	if arc.flags&EnableJS == 0 {
		// Remove script tags
		dom.RemoveNodes(dom.GetAllNodesWithTag(doc, "script"), nil)

		// Remove links with javascript scheme
		dom.ForEachNode(dom.GetElementsByTagName(doc, "a"), func(n *html.Node, _ int) {
			if strings.HasPrefix(dom.GetAttribute(n, "href"), "javascript") {
				dom.SetAttribute(n, "href", "#")
			}
		})

		// Remove all on* attributes
		dom.ForEachNode(dom.GetElementsByTagName(doc, "*"), func(n *html.Node, _ int) {
			n.Attr = slices.DeleteFunc(n.Attr, func(a html.Attribute) bool {
				return len(a.Key) >= 2 && strings.EqualFold(a.Key[0:2], "on")
			})
		})

		arc.convertNoScriptToDiv(ctx, doc, false)
	}

	if arc.flags&EnableCSS == 0 {
		// Remove style tags
		dom.RemoveNodes(dom.GetAllNodesWithTag(doc, "style"), nil)

		// Remove inline style
		dom.ForEachNode(dom.GetElementsByTagName(doc, "*"), func(n *html.Node, _ int) {
			dom.RemoveAttribute(n, "style")
		})

		// Remove style links
		dom.RemoveNodes(dom.GetAllNodesWithTag(doc, "link"), func(n *html.Node) bool {
			return dom.GetAttribute(n, "rel") == "stylesheet"
		})
	}

	if arc.flags&EnableEmbeds == 0 {
		dom.RemoveNodes(dom.GetAllNodesWithTag(doc, "object", "embed", "iframe"), nil)
	}

	if arc.flags&EnableImages == 0 {
		dom.RemoveNodes(dom.GetAllNodesWithTag(doc, "img", "picture"), nil)
		dom.RemoveNodes(dom.GetElementsByTagName(doc, "link"), func(n *html.Node) bool {
			return strings.Contains(dom.GetAttribute(n, "rel"), "icon")
		})
	}

	if arc.flags&EnableMedia == 0 {
		dom.RemoveNodes(dom.GetAllNodesWithTag(doc, "video", "audio"), nil)
	}
}

func (arc *Archiver) removePrefetch(_ context.Context, doc *html.Node) {
	dom.RemoveNodes(dom.GetAllNodesWithTag(doc, "link"), func(n *html.Node) bool {
		rel := dom.GetAttribute(n, "rel")
		return rel == "preload" || rel == "preconnect" || rel == "dns-prefetch"
	})
}

// fixRelativeURIs normalizes every href, src, srcset and poster
// URI values.
// It uses <base href> when present.
func (arc *Archiver) convertRelativeURLs(_ context.Context, doc *html.Node, base *url.URL) {
	attrs := []string{"href", "src", "poster"}
	baseURL := &url.URL{}
	*baseURL = *base

	// <base href> exists, we resolve its URL and set the new baseURL.
	if baseMeta := dom.QuerySelector(doc, "base[href]"); baseMeta != nil {
		b := dom.GetAttribute(baseMeta, "href")
		if b != "" {
			if buri, err := url.Parse(b); err == nil {
				baseURL = baseURL.ResolveReference(buri)
			}
		}
	}

	// walk through anything with href, src, poster attribute.
	for _, attr := range attrs {
		dom.ForEachNode(dom.QuerySelectorAll(doc, "["+attr+"]"), func(n *html.Node, _ int) {
			newURI := toAbsoluteURI(dom.GetAttribute(n, attr), baseURL)
			dom.SetAttribute(n, attr, newURI)
		})
	}

	// srcset handler
	dom.ForEachNode(dom.QuerySelectorAll(doc, "[srcset]"), func(n *html.Node, _ int) {
		srcset := dom.GetAttribute(n, "srcset")
		if srcset == "" {
			return
		}
		newSrcSet := rxSrcsetURL.ReplaceAllStringFunc(srcset, func(s string) string {
			p := rxSrcsetURL.FindStringSubmatch(s)
			return toAbsoluteURI(p[1], baseURL) + p[2] + p[3]
		})
		dom.SetAttribute(n, "srcset", newSrcSet)
	})

	// make fragments to the same document relative
	dom.ForEachNode(dom.QuerySelectorAll(doc, "a[href]"), func(n *html.Node, _ int) {
		attr := dom.GetAttribute(n, "href")
		if attr == "" {
			return
		}
		if strings.HasPrefix(attr, "#") {
			return
		}
		uri, err := url.Parse(dom.GetAttribute(n, "href"))
		if err != nil {
			return
		}
		if fragment := uri.Fragment; fragment != "" {
			uri.Fragment = ""
			tmp := new(url.URL)
			*tmp = *baseURL
			tmp.Fragment = ""
			if uri.String() == tmp.String() {
				dom.SetAttribute(n, "href", "#"+fragment)
			}
		}
	})
}

func (arc *Archiver) removeIntegrityAttr(_ context.Context, doc *html.Node) {
	for _, n := range dom.QuerySelectorAll(doc, "link,script") {
		dom.RemoveAttribute(n, "integrity")
	}
}

func (arc *Archiver) removeDataAttributes(_ context.Context, doc *html.Node) {
	dom.ForEachNode(dom.GetAllNodesWithTag(doc, "*"), func(n *html.Node, _ int) {
		keys := []string{}
		for _, attr := range n.Attr {
			if strings.HasPrefix(attr.Key, "data-") {
				keys = append(keys, attr.Key)
			}
		}
		for _, key := range keys {
			dom.RemoveAttribute(n, key)
		}
	})
}

func (arc *Archiver) setLazyImages(_ context.Context, doc *html.Node) {
	dom.ForEachNode(dom.GetElementsByTagName(doc, "img"), func(n *html.Node, _ int) {
		// "loading" should be the first attribute
		dom.RemoveAttribute(n, "loading")
		n.Attr = append([]html.Attribute{{Key: "loading", Val: "lazy"}}, n.Attr...)
	})
}

func (arc *Archiver) convertNoScriptToDiv(_ context.Context, doc *html.Node, restorable bool) {
	dom.ForEachNode(dom.GetElementsByTagName(doc, "noscript"), func(n *html.Node, _ int) {
		content := dom.TextContent(n)
		doc, err := html.Parse(strings.NewReader(content))
		if err != nil {
			return
		}
		body := dom.GetElementsByTagName(doc, "body")[0]

		div := dom.CreateElement("div")
		for _, child := range dom.ChildNodes(body) {
			dom.AppendChild(div, child)
		}

		if restorable {
			dom.SetAttribute(div, noScriptDataAttr, "true")
		}

		dom.ReplaceChild(n.Parent, div, n)
	})
}

func (arc *Archiver) removeComments(_ context.Context, doc *html.Node) {
	var comments []*html.Node
	var finder func(*html.Node)

	finder = func(node *html.Node) {
		if node.Type == html.CommentNode {
			comments = append(comments, node)
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			finder(child)
		}
	}

	for child := doc.FirstChild; child != nil; child = child.NextSibling {
		finder(child)
	}

	dom.RemoveNodes(comments, nil)
}

func (arc *Archiver) convertLazyImageAttrs(_ context.Context, doc *html.Node) {
	imageNodes := dom.GetAllNodesWithTag(doc, "img", "picture", "figure")
	dom.ForEachNode(imageNodes, func(elem *html.Node, _ int) {
		src := dom.GetAttribute(elem, "src")
		srcset := dom.GetAttribute(elem, "srcset")
		nodeTag := dom.TagName(elem)
		nodeClass := dom.ClassName(elem)

		// In some sites (e.g. Kotaku), they put 1px square image as data uri in
		// the src attribute. So, here we check if the data uri is too short,
		// just might as well remove it.
		if src != "" && rxB64DataURL.MatchString(src) {
			// Make sure it's not SVG, because SVG can have a meaningful image
			// in under 133 bytes.
			parts := rxB64DataURL.FindStringSubmatch(src)
			if parts[1] == "image/svg+xml" {
				return
			}

			// Make sure this element has other attributes which contains
			// image. If it doesn't, then this src is important and
			// shouldn't be removed.
			srcCouldBeRemoved := false
			for _, attr := range elem.Attr {
				if attr.Key == "src" {
					continue
				}

				if rxImgExtensions.MatchString(attr.Val) && isValidURL(attr.Val) {
					srcCouldBeRemoved = true
					break
				}
			}

			// Here we assume if image is less than 100 bytes (or 133B
			// after encoded to base64) it will be too small, therefore
			// it might be placeholder image.
			if srcCouldBeRemoved {
				b64starts := strings.Index(src, "base64") + 7
				b64length := len(src) - b64starts
				if b64length < 133 {
					src = ""
					dom.RemoveAttribute(elem, "src")
				}
			}
		}

		if (src != "" || srcset != "") && !strings.Contains(strings.ToLower(nodeClass), "lazy") {
			return
		}

		for i := 0; i < len(elem.Attr); i++ {
			attr := elem.Attr[i]
			if attr.Key == "src" || attr.Key == "srcset" {
				continue
			}

			copyTo := ""
			if rxLazyImageSrcset.MatchString(attr.Val) {
				copyTo = "srcset"
			} else if rxLazyImageSrc.MatchString(attr.Val) {
				copyTo = "src"
			}

			if copyTo == "" || !isValidURL(attr.Val) {
				continue
			}

			if nodeTag == "img" || nodeTag == "picture" {
				// if this is an img or picture, set the attribute directly
				dom.SetAttribute(elem, copyTo, attr.Val)
			} else if nodeTag == "figure" && len(dom.GetAllNodesWithTag(elem, "img", "picture")) == 0 {
				// if the item is a <figure> that does not contain an image or picture,
				// create one and place it inside the figure see the nytimes-3
				// testcase for an example
				img := dom.CreateElement("img")
				dom.SetAttribute(img, copyTo, attr.Val)
				dom.AppendChild(elem, img)
			}

			// Since the attribute already copied, just remove it
			dom.RemoveAttribute(elem, attr.Key)
		}
	})
}

func (arc *Archiver) revertConvertedNoScript(_ context.Context, doc *html.Node) {
	dom.ForEachNode(dom.GetElementsByTagName(doc, "div"), func(n *html.Node, _ int) {
		if dom.GetAttribute(n, noScriptDataAttr) != "true" {
			return
		}

		e := dom.CreateElement("noscript")
		dom.SetTextContent(e, dom.InnerHTML(n))
		dom.ReplaceChild(n.Parent, e, n)
	})
}
