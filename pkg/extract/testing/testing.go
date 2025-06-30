// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package testing provides some tools for fixture loading as HTTP mock responses.
package testing

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/jarcoal/httpmock"
)

// NewFileResponder returns a mock response for a file in test-fixtures.
func NewFileResponder(name string) httpmock.Responder {
	fd, err := os.Open(path.Join("test-fixtures", name))
	if err != nil {
		panic(err)
	}
	defer fd.Close() //nolint:errcheck

	data, err := io.ReadAll(fd)
	if err != nil {
		panic(err)
	}

	return httpmock.NewBytesResponder(200, data)
}

// NewContentResponder returns a mock response for a file, with extra headers.
func NewContentResponder(status int, headers map[string]string, name string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		fd, err := os.Open(path.Join("test-fixtures", name))
		if err != nil {
			panic(err)
		}
		defer fd.Close() //nolint:errcheck

		data, err := io.ReadAll(fd)
		if err != nil {
			panic(err)
		}

		rsp := httpmock.NewBytesResponse(status, data)
		for k, v := range headers {
			rsp.Header.Set(k, v)
		}
		rsp.Request = req
		return rsp, nil
	}
}

// NewHTMLResponder returns a mock response with an HTML content-type.
func NewHTMLResponder(status int, name string) httpmock.Responder {
	return NewContentResponder(
		status,
		map[string]string{"content-type": "text/html"},
		name)
}

type errReader int

func (errReader) Read([]byte) (n int, err error) {
	return 0, errors.New("read error")
}

func (errReader) Close() error {
	return nil
}

// NewIOErrorResponder returns a mock response with a faulty body.
func NewIOErrorResponder(status int, headers map[string]string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		rsp := httpmock.NewBytesResponse(status, []byte{})
		for k, v := range headers {
			rsp.Header.Set(k, v)
		}
		rsp.Request = req
		rsp.Body = errReader(0)
		return rsp, nil
	}
}
