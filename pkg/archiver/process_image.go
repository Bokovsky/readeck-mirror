// SPDX-FileCopyrightText: © 2020 Radhi Fadlillah
// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: MIT

package archiver

import (
	"cmp"
	"context"
	"iter"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/sync/errgroup"

	"github.com/go-shiori/dom"
)

const (
	replacedImgDataAttr = "data-x-replaced-img"

	// We won't use any image greater than 30Mpx.
	maxImageSize = 30 << 20
)

var imagePriority = []string{
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
	"image/vnd.microsoft.icon",
	"image/x-icon",
	"image/bmp",
	"image/tiff",
}

func (arc *Archiver) processBestImages(ctx context.Context, doc *html.Node) {
	dom.ForEachNode(dom.GetAllNodesWithTag(doc, "picture"), func(n *html.Node, _ int) {
		arc.processPictureNode(ctx, n)
	})

	dom.ForEachNode(dom.GetAllNodesWithTag(doc, "img"), func(n *html.Node, _ int) {
		if dom.GetAttribute(n, replacedImgDataAttr) == "true" {
			return
		}
		arc.processImgNode(ctx, n)
	})

	dom.ForEachNode(dom.GetAllNodesWithTag(doc, "img"), func(n *html.Node, _ int) {
		dom.RemoveAttribute(n, replacedImgDataAttr)
	})
}

func (arc *Archiver) processPictureNode(ctx context.Context, node *html.Node) {
	set := make(map[string]struct{})
	dom.ForEachNode(dom.GetAllNodesWithTag(node, "img", "source"), func(n *html.Node, _ int) {
		if dom.TagName(n) == "source" && dom.HasAttribute(n, "type") {
			// Discard types we know we don't support
			mt := strings.TrimSpace(strings.Split(dom.GetAttribute(n, "type"), ";")[0])
			if slices.Index(imagePriority, mt) == -1 {
				return
			}
		}

		maps.Insert(set, maps.All(getSrcset(dom.GetAttribute(n, "srcset"))))
		if n.Data == "img" && dom.HasAttribute(n, "src") {
			set[dom.GetAttribute(n, "src")] = struct{}{}
		}
	})

	log := arc.log().With(slog.Any("node", NodeLogValue(node)))

	bestImg := arc.findBestImage(withNodeContext(ctx, node), maps.Keys(set))
	if bestImg == nil {
		// no candidate, remove the node
		log.Warn("couldn't find image candidate for node",
			slog.String("node", node.Data),
		)
		node.Parent.RemoveChild(node)
		return
	}

	// Keep or create the img element.
	img := dom.QuerySelector(node, "img")
	if img == nil {
		img = dom.CreateElement("img")
		node.AppendChild(img)
	}

	// Remove the source elements
	dom.RemoveNodes(dom.GetElementsByTagName(node, "source"), nil)

	dom.RemoveAttribute(img, "srcset")
	dom.SetAttribute(img, "src", bestImg.URL())
	dom.SetAttribute(img, replacedImgDataAttr, "true")
	setImgSize(img, bestImg)

	log.Log(ctx, levelTrace, "replaced img",
		slog.Any("url", URLLogValue(bestImg.url)),
	)
}

func (arc *Archiver) processImgNode(ctx context.Context, node *html.Node) {
	set := make(map[string]struct{})
	maps.Insert(set, maps.All(getSrcset(dom.GetAttribute(node, "srcset"))))
	if src := dom.GetAttribute(node, "src"); src != "" {
		set[src] = struct{}{}
	}

	log := arc.log().With(slog.Any("node", NodeLogValue(node)))
	bestImg := arc.findBestImage(withNodeContext(ctx, node), maps.Keys(set))
	if bestImg == nil {
		// no candidate, remove the node
		log.Warn("couldn't find image candidate")
		node.Parent.RemoveChild(node)
		return
	}

	dom.RemoveAttribute(node, "srcset")
	dom.SetAttribute(node, "src", bestImg.URL())
	setImgSize(node, bestImg)

	log.Log(ctx, levelTrace, "replaced img",
		slog.Any("url", URLLogValue(bestImg.url)),
	)
}

func getSrcset(srcset string) (set map[string]struct{}) {
	set = make(map[string]struct{})

	for _, parts := range rxSrcsetURL.FindAllStringSubmatch(srcset, -1) {
		set[parts[1]] = struct{}{}
	}

	return
}

func setImgSize(node *html.Node, res *Resource) {
	keepSize := res.ContentType == "image/svg+xml" && dom.HasAttribute(node, "width") && dom.HasAttribute(node, "height")

	if !keepSize && res.Width > 0 && res.Height > 0 {
		dom.SetAttribute(node, "width", strconv.Itoa(res.Width))
		dom.SetAttribute(node, "height", strconv.Itoa(res.Height))
	}
}

func (arc *Archiver) findBestImage(ctx context.Context, seq iter.Seq[string]) *Resource {
	candidates := []*Resource{}
	mutext := sync.Mutex{}

	log := arc.log()
	if n := GetNodeContext(ctx); n != nil {
		log = log.With(slog.Any("node", NodeLogValue(n)))
	}
	g, ctx := errgroup.WithContext(ctx)
	for src := range seq {
		g.Go(func() error {
			log := log.With(slog.Any("url", URLLogValue(src)))

			res, err := arc.fetchInfo(ctx, src, http.Header{
				"Accept": {acceptImageHeader},
			})
			if err != nil {
				log.Warn("cannot load image", slog.Any("err", err))
				return nil
			}

			if res.ContentType == "image/svg+xml" {
				mutext.Lock()
				candidates = append(candidates, res)
				mutext.Unlock()
				return nil
			}

			r := res.Width * res.Height
			if r == 0 || r > maxImageSize {
				log.Warn("invalid image size",
					slog.Int("w", res.Width),
					slog.Int("h", res.Height),
				)
				return nil
			}

			mutext.Lock()
			candidates = append(candidates, res)
			mutext.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Error("error loading images", slog.Any("err", err))
		return nil
	}

	slices.SortStableFunc(candidates, func(a, b *Resource) int {
		return cmp.Or(
			cmp.Compare(b.Width, a.Width),
			-cmp.Compare(slices.Index(imagePriority, b.ContentType), slices.Index(imagePriority, a.ContentType)),
		)
	})

	if len(candidates) > 0 {
		return candidates[0]
	}

	return nil
}
