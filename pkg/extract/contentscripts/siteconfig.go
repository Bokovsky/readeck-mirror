// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
)

// SiteConfig holds the fivefilters configuration.
type SiteConfig struct {
	files []string

	TitleSelectors          []string          `json:"title_selectors"            js:"titleSelectors"`
	BodySelectors           []string          `json:"body_selectors"             js:"bodySelectors"`
	DateSelectors           []string          `json:"date_selectors"             js:"dateSelectors"`
	AuthorSelectors         []string          `json:"author_selectors"           js:"authorSelectors"`
	StripSelectors          []string          `json:"strip_selectors"            js:"stripSelectors"`
	StripIDOrClass          []string          `json:"strip_id_or_class"          js:"stripIdOrClass"`
	StripImageSrc           []string          `json:"strip_image_src"            js:"stripImageSrc"`
	NativeAdSelectors       []string          `json:"native_ad_selectors"`
	Tidy                    bool              `json:"tidy"`
	Prune                   bool              `json:"prune"`
	AutoDetectOnFailure     bool              `json:"autodetect_on_failure"`
	SinglePageLinkSelectors []string          `json:"single_page_link_selectors" js:"singlePageLinkSelectors"`
	NextPageLinkSelectors   []string          `json:"next_page_link_selectors"   js:"nextPageLinkSelectors"`
	ReplaceStrings          [][2]string       `json:"replace_strings"            js:"replaceStrings"`
	HTTPHeaders             map[string]string `json:"http_headers"               js:"httpHeaders"`
	Tests                   []FilterTest      `json:"tests"`
}

// FilterTest holds the values for a filter's test.
type FilterTest struct {
	URL      string   `json:"url"`
	Contains []string `json:"contains"`
}

// SiteConfigDiscovery is a wrapper around an fs.FS that provides
// a function to find site-config files based on a name.
type SiteConfigDiscovery struct {
	fs.FS
}

// NewSiteconfigDiscovery returns a new configuration discovery instance.
func NewSiteconfigDiscovery(root fs.FS) *SiteConfigDiscovery {
	return &SiteConfigDiscovery{FS: root}
}

// FindConfigHostFile finds the files matching the given name.
func (d *SiteConfigDiscovery) FindConfigHostFile(name string) []string {
	res := []string{}

	s, _ := fs.Stat(d, name+".json")
	if s != nil && !s.IsDir() {
		res = append(res, s.Name())
	}

	// Find wildcard files
	parts := strings.Split(name, ".")
	for i := range parts {
		n := fmt.Sprintf(".%s.json", strings.Join(parts[i:], "."))
		s, _ = fs.Stat(d, n)
		if s != nil && !s.IsDir() {
			res = append(res, s.Name())
		}
	}

	return res
}

// NewSiteConfig loads a configuration file from an io.Reader.
func NewSiteConfig(r io.Reader) (*SiteConfig, error) {
	cf := &SiteConfig{}
	cf.AutoDetectOnFailure = true
	dec := json.NewDecoder(r)
	if err := dec.Decode(cf); err != nil {
		return nil, err
	}

	return cf, nil
}

// NewConfigForURL loads site config configuration file(s) for
// a given URL.
func NewConfigForURL(discovery *SiteConfigDiscovery, src *url.URL) (*SiteConfig, error) {
	res := &SiteConfig{}
	res.HTTPHeaders = map[string]string{}
	res.AutoDetectOnFailure = true

	hostname := strings.TrimPrefix(src.Hostname(), "www.")
	hostname, _ = idna.ToASCII(hostname)

	files := discovery.FindConfigHostFile(hostname)
	files = append(files, discovery.FindConfigHostFile("global")...)

	for _, x := range files {
		fd, err := discovery.Open(x)
		if err != nil {
			return nil, err
		}
		cf, err := NewSiteConfig(fd)
		if err != nil {
			fd.Close() //nolint:errcheck
			return nil, err
		}
		if !res.AutoDetectOnFailure {
			fd.Close() //nolint:errcheck
			break
		}
		cf.files = []string{x}
		res.Merge(cf)
		fd.Close() //nolint:errcheck
	}

	return res, nil
}

// Merge merges a new configuration in the current one.
func (cf *SiteConfig) Merge(src *SiteConfig) {
	cf.files = append(cf.files, src.files...)
	cf.TitleSelectors = append(cf.TitleSelectors, src.TitleSelectors...)
	cf.BodySelectors = append(cf.BodySelectors, src.BodySelectors...)
	cf.DateSelectors = append(cf.DateSelectors, src.DateSelectors...)
	cf.AuthorSelectors = append(cf.AuthorSelectors, src.AuthorSelectors...)
	cf.StripSelectors = append(cf.StripSelectors, src.StripSelectors...)
	cf.StripIDOrClass = append(cf.StripIDOrClass, src.StripIDOrClass...)
	cf.StripImageSrc = append(cf.StripImageSrc, src.StripImageSrc...)
	cf.NativeAdSelectors = append(cf.NativeAdSelectors, src.NativeAdSelectors...)
	cf.Tidy = src.Tidy
	cf.Prune = src.Prune
	cf.AutoDetectOnFailure = src.AutoDetectOnFailure
	cf.SinglePageLinkSelectors = append(cf.SinglePageLinkSelectors, src.SinglePageLinkSelectors...)
	cf.NextPageLinkSelectors = append(cf.NextPageLinkSelectors, src.NextPageLinkSelectors...)
	cf.ReplaceStrings = append(cf.ReplaceStrings, src.ReplaceStrings...)
	cf.Tests = append(cf.Tests, src.Tests...)

	maps.Copy(cf.HTTPHeaders, src.HTTPHeaders)
}

// Files returns the files used to create the configuration.
func (cf *SiteConfig) Files() []string {
	return cf.files
}
