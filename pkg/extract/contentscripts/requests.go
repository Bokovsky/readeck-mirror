// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dop251/goja"

	"codeberg.org/readeck/readeck/pkg/extract"
)

type httpClient struct {
	*http.Client
	vm *Runtime
}

type httpResponse struct {
	*http.Response
	vm   *Runtime
	body *bytes.Buffer
}

// NewHTTPClient returns a new (very) simple HTTP client for the JS runtime.
func NewHTTPClient(vm *Runtime, client *http.Client) (*goja.Object, error) {
	c := &httpClient{vm: vm, Client: client}

	obj := vm.NewObject()
	if err := obj.Set("get", c.get); err != nil {
		return nil, err
	}
	if err := obj.Set("post", c.post); err != nil {
		return nil, err
	}

	return obj, nil
}

func (c *httpClient) Do(req *http.Request, args ...goja.Value) (*goja.Object, error) {
	c.vm.GetLogger().Debug("request",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
	)

	if m := c.vm.getProcessMessage(); m != nil {
		u := m.Extractor.Drop().URL
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", u.Scheme, u.Hostname()))
	}

	if len(args) > 0 {
		var headers map[string]string
		if err := c.vm.ExportTo(args[0], &headers); err != nil {
			return nil, err
		}
		for k, v := range headers {
			k = http.CanonicalHeaderKey(k)
			if k == "Content-Length" {
				continue
			}
			if v != "" {
				req.Header.Set(k, v)
			} else {
				req.Header.Del(k)
			}
		}
	}

	ctx := extract.WithRequestType(req.Context(), extract.ContentScriptRequest)
	r, err := c.Client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	return newHTTPResponse(c.vm, r)
}

func (c *httpClient) get(url string, args ...goja.Value) (*goja.Object, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return c.Do(req, args...)
}

func (c *httpClient) post(url string, data []byte, args ...goja.Value) (*goja.Object, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	return c.Do(req, args...)
}

func newHTTPResponse(vm *Runtime, rsp *http.Response) (*goja.Object, error) {
	defer rsp.Body.Close() //nolint:errcheck

	r := &httpResponse{vm: vm, Response: rsp, body: new(bytes.Buffer)}
	if _, err := io.Copy(r.body, rsp.Body); err != nil {
		return nil, err
	}

	headers := map[string]string{}
	for k, v := range r.Header {
		headers[k] = strings.Join(v, " ")
	}

	obj := vm.NewObject()
	if err := obj.Set("status", r.StatusCode); err != nil {
		return nil, err
	}
	if err := obj.Set("headers", headers); err != nil {
		return nil, err
	}
	if err := obj.Set("raiseForStatus", r.raiseForStatus); err != nil {
		return nil, err
	}
	if err := obj.Set("json", r.json); err != nil {
		return nil, err
	}
	if err := obj.Set("text", r.text); err != nil {
		return nil, err
	}

	return obj, nil
}

func (r *httpResponse) raiseForStatus() error {
	if r.StatusCode/100 != 2 {
		return fmt.Errorf("invalid status code %d", r.StatusCode)
	}
	return nil
}

func (r *httpResponse) json() (goja.Value, error) {
	var res any
	if err := json.Unmarshal(r.body.Bytes(), &res); err != nil {
		return nil, err
	}
	return r.vm.ToValue(res), nil
}

func (r *httpResponse) text() string {
	return r.body.String()
}
