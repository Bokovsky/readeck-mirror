// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// tools/docs is a script that builds an HTML documentation based on markdown files
// in a source directory.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/crc64"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	figure "github.com/mangoumbrella/goldmark-figure"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var ctxTitleKey = parser.NewContextKey()

// File is a documentation file.
type File struct {
	Route      string         `json:"route"`
	Aliases    []string       `json:"aliases"`
	File       string         `json:"file"`
	Etag       string         `json:"etag"`
	IsDocument bool           `json:"is_document"`
	Title      string         `json:"title"`
	Meta       map[string]any `json:"meta"`
}

// Section is a documentation section (locale).
type Section struct {
	Files map[string]*File `json:"files"`
	TOC   [][2]string      `json:"toc"`
}

// Manifest contains a list of all files and sections.
type Manifest struct {
	Files    map[string]*File    `json:"files"`
	Sections map[string]*Section `json:"sections"`
}

type linkTransform struct{}

func (t *linkTransform) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if n, ok := node.(*ast.Link); ok {
				href, err := url.Parse(string(n.Destination))
				if err != nil {
					log.Fatal(err)
				}

				if href.Scheme == "" {
					href.Path = strings.TrimSuffix(href.Path, ".md")
					if href.Path == "index" {
						href.Path = "./"
					}
					if strings.HasSuffix(href.Path, "/index") {
						href.Path = strings.TrimSuffix(href.Path, "index")
					}
					n.Destination = []byte(href.String())
				}
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

type imgTransform struct {
	src string
}

func (t *imgTransform) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if n, ok := node.(*ast.Image); ok {
				// Check for image source. If the file does not exist,
				// change the source to the en-US one.
				d := string(n.Destination)
				p := filepath.Join(filepath.Dir(t.src), d)
				if _, err := os.Stat(p); err != nil && os.IsNotExist(err) {
					d, _ = filepath.Rel(filepath.Dir(t.src), p)
					if strings.HasPrefix(d, "img/") {
						n.Destination = []byte("../en-US/" + d)
					}

				}
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

type titleExtract struct{}

func (t *titleExtract) Transform(doc *ast.Document, reader text.Reader, ctx parser.Context) {
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkStop, nil
		}
		n, ok := node.(*ast.Heading)
		if !ok || n.Level != 1 {
			return ast.WalkContinue, nil
		}
		ctx.Set(ctxTitleKey, string(n.Lines().Value(reader.Source())))

		return ast.WalkStop, nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func mdToHTML(source, dest string) (*File, error) {
	src, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer src.Close() //nolint:errcheck
	md, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	p := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.DefinitionList,
			figure.Figure,
			meta.Meta,
		),
		goldmark.WithParserOptions(
			parser.WithHeadingAttribute(),
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&linkTransform{}, 900),
				util.Prioritized(&imgTransform{src: source}, 901),
				util.Prioritized(&titleExtract{}, 902),
			),
		),
	)
	ctx := parser.NewContext()
	if err = p.Convert(md, buf, parser.WithContext(ctx)); err != nil {
		return nil, err
	}

	dst, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	defer dst.Close() //nolint:errcheck

	if _, err = dst.Write(buf.Bytes()); err != nil {
		return nil, err
	}

	title, _ := ctx.Get(ctxTitleKey).(string)
	if t, ok := meta.Get(ctx)["Title"]; ok {
		title = t.(string)
	}

	src.Seek(0, 0) //nolint:errcheck

	return &File{
		File:       dst.Name(),
		IsDocument: true,
		Title:      title,
		Meta:       meta.Get(ctx),
		Etag:       getEtag(dst.Name(), src, buf),
	}, nil
}

func copyFile(source, dest string) (*File, error) {
	src, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer src.Close() //nolint:errcheck

	dst, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	defer dst.Close() //nolint:errcheck

	_, err = io.Copy(dst, src)
	if err != nil {
		return nil, err
	}

	src.Seek(0, 0) //nolint:errcheck

	return &File{
		File: dst.Name(),
		Etag: getEtag(dst.Name(), src),
	}, nil
}

func getEtag(name string, src ...io.Reader) string {
	h := crc64.New(crc64.MakeTable(crc64.ISO))
	_, err := io.Copy(h, io.MultiReader(append(src, strings.NewReader(name))...))
	if err != nil {
		log.Fatal(err)
	}
	return strconv.FormatUint(h.Sum64(), 16)
}

func newManifest(fileList []*File) (*Manifest, error) {
	res := &Manifest{
		Files:    make(map[string]*File),
		Sections: make(map[string]*Section),
	}

	// Build file lists
	for _, file := range fileList {
		if !file.IsDocument {
			res.Files[file.Route] = file
			continue
		}

		topLevel := strings.SplitN(file.Route, "/", 2)[0]
		section, ok := res.Sections[topLevel]
		if !ok {
			section = &Section{
				TOC:   [][2]string{},
				Files: make(map[string]*File),
			}
			res.Sections[topLevel] = section
		}
		section.Files[file.Route] = file
	}

	// Build each section's TOC
	for name, section := range res.Sections {
		index, ok := section.Files[name+"/"]
		if !ok {
			return nil, fmt.Errorf("no index file in section %s", name)
		}
		toc, ok := index.Meta["TOC"]
		if !ok {
			return nil, fmt.Errorf("no toc entry in %s/index.md", name)
		}
		for _, x := range toc.([]any) {
			route := name + "/" + x.(string)
			f, ok := section.Files[route]
			if !ok {
				return nil, fmt.Errorf("TOC entry for %s/%s does not exist", name, route)
			}

			section.TOC = append(section.TOC, [2]string{route, f.Title})
		}
	}

	return res, nil
}

func main() {
	flag.Parse()

	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	if len(flag.Args()) < 2 {
		log.Fatalf("Usage: build_help <src> <dest>")
	}

	srcDir, _ := filepath.Abs(flag.Arg(0))
	destDir, _ := filepath.Abs(flag.Arg(1))

	info, err := os.Stat(srcDir)
	if err != nil {
		log.Fatal(err)
	}
	if !info.IsDir() {
		log.Fatalf("%s is not a directory", srcDir)
	}

	info, err = os.Stat(destDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if e := os.MkdirAll(destDir, 0o750); e != nil {
				log.Fatal(e)
			}
		} else {
			log.Fatal(err)
		}
	}
	if info != nil && !info.IsDir() {
		log.Fatalf("%s is not a directory", destDir)
	}

	log.Printf("Reading help files from %s", srcDir)
	log.Printf("Destination: %s", destDir)

	fileList := []*File{}

	err = filepath.Walk(srcDir, func(src string, info fs.FileInfo, _ error) error {
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(srcDir, src)
		if err != nil {
			return err
		}

		dst := filepath.Join(destDir, rel)

		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return err
		}

		var file *File
		route := filepath.ToSlash(rel)
		aliases := []string{}

		if path.Ext(src) == ".md" {
			dst = strings.TrimSuffix(dst, ".md") + ".html"

			file, err = mdToHTML(src, dst)
			route = strings.TrimSuffix(route, ".md")
			if strings.HasSuffix(route, "/index") {
				route = strings.TrimSuffix(route, "index")
				aliases = append(aliases, strings.TrimSuffix(route, "/"), route+"index")
			}
		} else {
			file, err = copyFile(src, dst)
		}

		if err != nil {
			log.Fatal(err)
		}

		file.File, _ = filepath.Rel(destDir, dst)
		file.Route = route
		file.Aliases = aliases

		fileList = append(fileList, file)

		_src, _ := filepath.Rel(filepath.Dir(srcDir), src)
		_dst, _ := filepath.Rel(filepath.Dir(destDir), dst)
		log.Printf("- %s -> %s", _src, _dst)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	manifest, err := newManifest(fileList)
	if err != nil {
		log.Fatal(err)
	}

	// Copy changelog
	cf, err := mdToHTML(
		filepath.Join(srcDir, "../../CHANGELOG.md"),
		filepath.Join(destDir, "changelog.html"),
	)
	if err != nil {
		log.Fatal(err)
	}
	cf.File, _ = filepath.Rel(destDir, cf.File)
	cf.Route = "changelog"
	manifest.Files["changelog"] = cf

	// Create manifest
	fd, err := os.Create(path.Join(destDir, "manifest.json"))
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(fd)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		log.Fatal(err)
	}
}
