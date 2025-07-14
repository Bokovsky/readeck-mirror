// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"archive/zip"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	rxHTMLStart = regexp.MustCompile(`^(?s)(.*?)<body>`)
	rxHTMLEnd   = regexp.MustCompile(`</body>\s*</html>\s*$`)
)

// BookmarkContainer is a wrapper around zip.ReadCloser
// to handle a bookmark's zipfile.
type BookmarkContainer struct {
	*zip.ReadCloser
	articleFilename string
	articleContent  *strings.Builder
}

// OpenContainer opens the bookmark's zipfile and returns a new
// bookmarkContainer instance.
func (b *Bookmark) OpenContainer() (*BookmarkContainer, error) { //revive:disable:unexported-return
	p := b.GetFilePath()
	if p == "" {
		return nil, os.ErrNotExist
	}

	z, err := zip.OpenReader(p)
	if err != nil {
		return nil, err
	}

	res := &BookmarkContainer{
		ReadCloser:     z,
		articleContent: new(strings.Builder),
	}
	if a, ok := b.Files["article"]; ok {
		res.articleFilename = a.Name
	}

	return res, nil
}

// Lookup return a [*zip.File] with the given name, when it exists.
func (c *BookmarkContainer) Lookup(name string) (*zip.File, bool) {
	for _, entry := range c.File {
		if entry.Name == name {
			return entry, true
		}
	}
	return nil, false
}

// ListResources returns a list of files located under "_resources/".
func (c *BookmarkContainer) ListResources() []*zip.File {
	res := []*zip.File{}
	for _, entry := range c.File {
		if !strings.HasSuffix(entry.Name, "/") && strings.HasPrefix(entry.Name, resourceDirName) {
			res = append(res, entry)
		}
	}

	return res
}

// LoadArticle loads the bookmark´s article when it exists.
func (c *BookmarkContainer) LoadArticle() error {
	if c.articleFilename == "" {
		return os.ErrNotExist
	}

	fp, err := c.Open(c.articleFilename)
	if err != nil {
		return err
	}

	_, err = io.Copy(c.articleContent, fp)

	return err
}

// ReplaceLinks replaces all the link to _resources/* in the article content.
func (c *BookmarkContainer) ReplaceLinks(fn func(name string) string) (err error) {
	args := []string{}
	for _, x := range c.ListResources() {
		args = append(args, "./"+x.Name, fn("./"+x.Name))
	}

	replacer := strings.NewReplacer(args...)
	res := replacer.Replace(c.articleContent.String())
	c.articleContent.Reset()
	_, err = c.articleContent.WriteString(res)
	return
}

// ExtractBody extract the content of the article's HTML body.
func (c *BookmarkContainer) ExtractBody() (err error) {
	res := ExtractHTMLBody(c.articleContent.String())
	c.articleContent.Reset()
	_, err = c.articleContent.WriteString(res)
	return
}

// GetArticle returns a string of the article's HTML.
func (c *BookmarkContainer) GetArticle() string {
	return c.articleContent.String()
}

// GetFile returns a file's content.
func (c *BookmarkContainer) GetFile(name string) ([]byte, error) {
	fd, err := c.Open(name)
	if err != nil {
		return nil, err
	}
	defer fd.Close() //nolint:errcheck
	return io.ReadAll(fd)
}

// ExtractHTMLBody returns the given string's content that's inside
// the body element.
func ExtractHTMLBody(text string) string {
	res := rxHTMLStart.ReplaceAllString(text, "")
	res = rxHTMLEnd.ReplaceAllString(res, "")
	return res
}
