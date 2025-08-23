// SPDX-FileCopyrightText: © 2020 Radhi Fadlillah
// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: MIT

package archiver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

func ignoreFontFace(lexer *css.Lexer) {
	// Consume the rule until a right brace
	for {
		token, _ := lexer.Next()
		if token == css.ErrorToken || token == css.RightBraceToken {
			break
		}
	}
}

func (arc *Archiver) processCSS(ctx context.Context, r io.Reader, parent *Resource) (*bytes.Buffer, error) {
	// Scan CSS and find all URLs
	contents := new(bytes.Buffer)
	if _, err := io.Copy(contents, r); err != nil {
		return nil, err
	}

	urls := make(map[string]struct{})
	lexer := css.NewLexer(parse.NewInputBytes(contents.Bytes()))

	buf := new(bytes.Buffer)
	for {
		token, bt := lexer.Next()
		if token == css.ErrorToken {
			break
		}

		if arc.flags&EnableFonts == 0 && token == css.TokenType(css.AtKeywordToken) && bytes.Equal(bt, []byte("@font-face")) {
			ignoreFontFace(lexer)
			continue
		}

		if token == css.URLToken {
			urls[string(bt)] = struct{}{}
		}
		buf.Write(bt)
	}

	// Download all resources and prepare the replacement with the new
	// name.
	mutex := sync.RWMutex{}
	replacer := []string{}

	parentURL, err := url.Parse(parent.url)
	if err != nil {
		return nil, errSkippedURL
	}

	ctx = withReferrerContext(ctx, parentURL)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(5)
	for token := range urls {
		g.Go(func() error {
			cssURL := sanitizeStyleURL(token)
			if strings.HasPrefix(cssURL, "#") {
				return nil
			}

			cssURL = toAbsoluteURI(cssURL, parentURL)
			res, err := arc.processURL(ctx, cssURL, processOptions{})
			if err != nil {
				if errors.Is(err, errSkippedURL) {
					return nil
				}
				return err
			}

			// Make the path relative to its parent
			newURL := res.Value()
			if res.Contents == nil {
				newURL = res.Name[len(commonPrefix(res.Name, parent.Name)):]
			}

			mutex.Lock()
			replacer = append(replacer,
				token, fmt.Sprintf(`url("%s")`, newURL),
			)
			mutex.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Replace original URLs and return the new buffer.
	result := new(bytes.Buffer)
	repl := strings.NewReplacer(replacer...)
	if _, err := repl.WriteString(result, buf.String()); err != nil {
		return nil, err
	}

	return result, nil
}
