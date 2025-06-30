// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/araddon/dateparse"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type wallabagAdapter struct {
	Endpoint string `json:"url"`
	Token    string `json:"token"`
	articles *wallabagArticleList
}

type wallabagArticleList struct {
	Links struct {
		Next struct {
			Href string `json:"href"`
		} `json:"next"`
	} `json:"_links"`
	Embedded struct {
		Items []wallabagArticle `json:"items"`
	} `json:"_embedded"`
}

type wallabagArticle struct {
	IsArchived     int               `json:"is_archived"`
	IsStarred      int               `json:"is_starred"`
	Title          string            `json:"title"`
	ArticleURL     string            `json:"url"`
	Content        string            `json:"content"`
	CreatedAt      string            `json:"created_at"`
	PublishedAt    string            `json:"published_at"`
	PublishedBy    []string          `json:"published_by"`
	Language       string            `json:"language"`
	Tags           []wallabagTag     `json:"tags"`
	PreviewPicture string            `json:"preview_picture"`
	Headers        map[string]string `json:"headers"`
}

type wallabagTag struct {
	Label string `json:"label"`
}

func (wa *wallabagArticle) URL() string {
	return wa.ArticleURL
}

func (wa *wallabagArticle) Meta() (*BookmarkMeta, error) {
	res := &BookmarkMeta{
		Title:      wa.Title,
		Authors:    wa.PublishedBy,
		Lang:       wa.Language,
		Labels:     types.Strings{},
		IsArchived: wa.IsArchived > 0,
		IsMarked:   wa.IsStarred > 0,
	}

	res.Created, _ = dateparse.ParseAny(wa.CreatedAt)
	res.Published, _ = dateparse.ParseAny(wa.PublishedAt)

	for _, x := range wa.Tags {
		res.Labels = append(res.Labels, x.Label)
	}

	return res, nil
}

func (wa *wallabagArticle) EnableReadability() bool {
	return wa.Content == ""
}

func (wa *wallabagArticle) Resources() []tasks.MultipartResource {
	if wa.Content == "" {
		return nil
	}

	root, err := html.Parse(strings.NewReader(wa.Content))
	if err != nil {
		return nil
	}

	if wa.PreviewPicture != "" {
		node := dom.CreateElement("meta")
		dom.SetAttribute(node, "property", "og:image")
		dom.SetAttribute(node, "content", wa.PreviewPicture)
		dom.QuerySelector(root, "head").AppendChild(node)
	}

	buf := new(bytes.Buffer)
	html.Render(buf, root)

	if wa.Headers == nil {
		wa.Headers = map[string]string{"Content-Type": "text/html"}
	} else if v, ok := wa.Headers["Content-Type"]; ok {
		// Wallabag stores archived page content as UTF-8 regardless of the original encoding,
		// but for some reason will also store the original content-type of the page inclusive
		// of the `charset=...` declaration and include that in API responses. Readeck would
		// then think that the page is still in that encoding and transcode the already valid
		// content into garbage. This strips `charset=...` directives received from Wallabag.
		wa.Headers["Content-Type"], _, _ = strings.Cut(v, ";")
	}

	return []tasks.MultipartResource{
		{
			URL:     wa.ArticleURL,
			Headers: wa.Headers,
			Data:    buf.Bytes(),
		},
	}
}

func (adapter *wallabagAdapter) Name(_ forms.Translator) string {
	return "Wallabag"
}

func (adapter *wallabagAdapter) Form() forms.Binder {
	return forms.Must(
		context.Background(),
		forms.NewTextField("url",
			forms.Trim,
			forms.Required,
			forms.IsURL(allowedSchemes...),
		),
		forms.NewTextField("username", forms.Trim, forms.Required),
		forms.NewTextField("password", forms.Required),
		forms.NewTextField("client_id", forms.Trim, forms.Required),
		forms.NewTextField("client_secret", forms.Trim, forms.Required),
	)
}

func (adapter *wallabagAdapter) Params(f forms.Binder) ([]byte, error) {
	if !f.IsValid() {
		return nil, nil
	}

	endpoint, _ := url.Parse(f.Get("url").String())
	endpoint.Fragment = ""
	if endpoint.Path != "" {
		endpoint.Path = strings.TrimSuffix(path.Clean(endpoint.Path), "/")
	}
	adapter.Endpoint = endpoint.String()

	err := adapter.authenticate(f)
	if !f.IsValid() {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return json.Marshal(adapter)
}

func (adapter *wallabagAdapter) LoadData(data []byte) (err error) {
	if err = json.Unmarshal(data, adapter); err != nil {
		return
	}

	if adapter.Token == "" {
		err = errors.New("no token provided")
		return
	}

	// Initialize an empty article list with the first "next" URL to fetch
	adapter.articles = &wallabagArticleList{}
	adapter.articles.Links.Next.Href = adapter.Endpoint + "/api/entries?sort=created&order=desc&perPage=10&page=1"
	adapter.articles.Embedded.Items = []wallabagArticle{}
	return
}

func (adapter *wallabagAdapter) Next() (BookmarkImporter, error) {
	var err error

	if len(adapter.articles.Embedded.Items) == 0 {
		if adapter.articles.Links.Next.Href == "" {
			// No next link, we're done
			return nil, io.EOF
		}

		// Fetch next article list
		if err = adapter.fetchArticles(); err != nil {
			return nil, err
		}
	}

	// Pull the first item in the list
	item := adapter.articles.Embedded.Items[0]
	adapter.articles.Embedded.Items = adapter.articles.Embedded.Items[1:]

	// Cleanup the URL. This is done later by createBookmark() but
	// we want the URL to match anything that is sent by Resources() later.
	uri, err := url.Parse(item.ArticleURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIgnore, err)
	}

	// Set canonical header keys
	headers := make(map[string]string)
	for k, v := range item.Headers {
		headers[textproto.CanonicalMIMEHeaderKey(k)] = v
	}
	item.Headers = headers

	if !slices.Contains(allowedSchemes, uri.Scheme) {
		return nil, fmt.Errorf("%w: invalid scheme %s (%s)", ErrIgnore, uri.Scheme, uri)
	}

	item.ArticleURL = uri.String()

	return &item, nil
}

func (adapter *wallabagAdapter) authenticate(f forms.Binder) error {
	body := new(bytes.Buffer)
	enc := json.NewEncoder(body)
	_ = enc.Encode(map[string]string{
		"grant_type":    "password",
		"client_id":     f.Get("client_id").String(),
		"client_secret": f.Get("client_secret").String(),
		"username":      f.Get("username").String(),
		"password":      f.Get("password").String(),
	})

	req, _ := http.NewRequest(http.MethodPost, adapter.Endpoint+"/oauth/v2/token", body)
	req.Header.Set("Content-Type", "application/json")

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.AddErrors("", err)
		return nil
	}
	defer rsp.Body.Close() //nolint:errcheck

	if rsp.StatusCode == http.StatusNotFound {
		f.AddErrors("", forms.Gettext("Invalid URL"))
		return nil
	}

	if rsp.StatusCode != http.StatusOK {
		f.AddErrors("", forms.Gettext("Invalid credentials"))
		return nil
	}

	res := map[string]string{}
	dec := json.NewDecoder(rsp.Body)
	// we don't need to check for errors here, only that the access_token is present at the end
	_ = dec.Decode(&res)

	var ok bool
	if adapter.Token, ok = res["access_token"]; !ok {
		f.AddErrors("", forms.Gettext("No access token found"))
		return nil
	}

	return nil
}

func (adapter *wallabagAdapter) fetchArticles() error {
	req, _ := http.NewRequest(http.MethodGet, adapter.articles.Links.Next.Href, nil)
	req.Header.Set("Authorization", "Bearer "+adapter.Token)

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close() //nolint:errcheck

	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response %d", rsp.StatusCode)
	}

	// Always reset the next URL
	adapter.articles.Links.Next.Href = ""

	err = json.NewDecoder(rsp.Body).Decode(adapter.articles)
	if len(adapter.articles.Embedded.Items) == 0 {
		// Just in case, this will break the loop
		return io.EOF
	}
	return err
}
