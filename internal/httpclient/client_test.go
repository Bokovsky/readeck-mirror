// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package httpclient_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/httpclient"
)

type echoResponse struct {
	URL    string
	Method string
	Header http.Header
}

func mockResponder(client *http.Client) func() {
	ot := client.Transport.(*httpclient.Transport).RoundTripper
	mt := httpmock.NewMockTransport()

	mt.RegisterResponder("GET", `=~.*`,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, echoResponse{
				URL:    req.URL.String(),
				Method: req.Method,
				Header: req.Header,
			})
		})

	client.Transport.(*httpclient.Transport).RoundTripper = mt

	return func() {
		client.Transport.(*httpclient.Transport).RoundTripper = ot
	}
}

func TestClient(t *testing.T) {
	t.Run("RoundTrip", func(t *testing.T) {
		t.Run("request", func(t *testing.T) {
			assert := require.New(t)

			client := httpclient.New()
			deactivate := mockResponder(client)
			defer deactivate()

			rsp, err := client.Get("https://example.net/")
			assert.NoError(err)
			defer rsp.Body.Close() //nolint:errcheck

			dec := json.NewDecoder(rsp.Body)
			var data echoResponse
			assert.NoError(dec.Decode(&data))

			assert.Equal("https://example.net/", data.URL)
			assert.Equal("GET", data.Method)
			assert.Contains(data.Header, "User-Agent")
			assert.Equal("none", data.Header.Get("Sec-Fetch-Site"))
		})

		t.Run("SetHeader", func(t *testing.T) {
			assert := require.New(t)

			client := httpclient.New()
			deactivate := mockResponder(client)
			defer deactivate()

			client.Transport.(*httpclient.Transport).SetHeader(func(h http.Header) {
				h.Set("x-test", "abc")
			})

			rsp, err := client.Get("https://example.net/")
			assert.NoError(err)
			defer rsp.Body.Close() //nolint:errcheck

			dec := json.NewDecoder(rsp.Body)
			var data echoResponse
			assert.NoError(dec.Decode(&data))

			assert.Equal("https://example.net/", data.URL)
			assert.Equal("abc", data.Header.Get("x-test"))
		})
	})
}
