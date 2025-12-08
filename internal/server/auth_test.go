// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server_test

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/pkg/base58"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestTokenAuth(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	unknownToken, err := configs.Keys.TokenKey().Encode(base58.NewUUID())
	require.NoError(t, err)

	t1 := &tokens.Token{
		UserID:      &(app.Users["user"].User.ID),
		Application: "test",
		IsEnabled:   true,
	}
	require.NoError(t, tokens.Tokens.Create(t1))
	t1Encoded, err := configs.Keys.TokenKey().Encode(t1.UID)
	require.NoError(t, err)

	exp := time.Now().UTC().Add(-time.Minute)
	t2 := &tokens.Token{
		UserID:      &(app.Users["user"].User.ID),
		Application: "test",
		IsEnabled:   true,
		Expires:     &exp,
	}
	require.NoError(t, tokens.Tokens.Create(t2))
	t2Encoded, err := configs.Keys.TokenKey().Encode(t2.UID)
	require.NoError(t, err)

	client := app.Client()

	tests := []struct {
		name   string
		req    func(r *http.Request)
		assert func(assert *require.Assertions, rsp *Response)
	}{
		{
			name: "no token",
			req:  func(*http.Request) {},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "empty bearer",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer")
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "space bearer",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer   ")
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "bearer",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+t1Encoded)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
			},
		},
		{
			name: "bearer unknown",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+unknownToken)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "bearer expired",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+t2Encoded)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "basic auth",
			req: func(r *http.Request) {
				r.SetBasicAuth("", t1Encoded)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
			},
		},
		{
			name: "basic auth any user",
			req: func(r *http.Request) {
				r.SetBasicAuth("alice and bob", t1Encoded)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
			},
		},
		{
			name: "basic auth empty",
			req: func(r *http.Request) {
				r.SetBasicAuth("", "")
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := client.NewRequest("GET", "/api/profile", nil)
			require.NoError(t, err)
			test.req(req)

			rsp := client.Request(t, req)
			test.assert(require.New(t), rsp)
		})
	}
}

func TestForwardedAuth(t *testing.T) {
	crt := &x509.Certificate{
		SerialNumber: big.NewInt(1234),
		Subject: pkix.Name{
			Organization: []string{"Test Client"},
		},
		EmailAddresses: []string{"contact@example.org"},
		NotBefore:      time.Now(),
		NotAfter:       time.Now().Add(time.Hour),
		KeyUsage:       x509.KeyUsageDigitalSignature,
	}

	cf := configs.Config.Auth.Forwarded

	restoreConf := func() {
		configs.Config.Server.ClientCAFile = ""
		configs.Config.Auth.Forwarded = cf
	}

	app := NewTestApp(t)
	defer app.Close(t)

	client := app.Client()

	tests := []struct {
		name   string
		req    func(r *http.Request)
		assert func(assert *require.Assertions, rsp *Response)
	}{
		{
			name: "disabled",
			req: func(r *http.Request) {
				configs.Config.Auth.Forwarded.Enabled = false
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "missing headers",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "user")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "authorized ip4",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
			},
		},
		{
			name: "authorized ip6",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "[::1]:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
			},
		},
		{
			name: "unauthorized ip4",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "1.2.3.4:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(403, rsp.StatusCode)
			},
		},
		{
			name: "unauthorized ip6",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "[22::]:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(403, rsp.StatusCode)
			},
		},
		{
			name: "invalid username",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "user@localhost")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(403, rsp.StatusCode)
			},
		},
		{
			name: "invalid email",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(403, rsp.StatusCode)
			},
		},
		{
			name: "invalid group",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "some-group")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(403, rsp.StatusCode)
			},
		},
		{
			name: "update user",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@example.org")
				r.Header.Set("Remote-Groups", "admin")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
				user, err := users.Users.GetOne(
					goqu.C("id").Eq(app.Users["user"].User.ID),
				)
				assert.NoError(err)
				assert.Equal("user@example.org", user.Email)
				assert.Equal("admin", user.Group)
			},
		},
		{
			name: "no provisioning",
			req: func(r *http.Request) {
				r.Header.Set("Remote-User", "new-user")
				r.Header.Set("Remote-Email", "new-user@example.org")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(403, rsp.StatusCode)
			},
		},
		{
			name: "provisioning",
			req: func(r *http.Request) {
				configs.Config.Auth.Forwarded.ProvisioningEnabled = true

				r.Header.Set("Remote-User", "new-user")
				r.Header.Set("Remote-Email", "new-user@example.org")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)

				user, err := users.Users.GetOne(
					goqu.C("username").Eq("new-user"),
				)
				assert.NoError(err)
				assert.Equal("new-user@example.org", user.Email)
				assert.Equal("user", user.Group)
			},
		},
		{
			name: "client cert no tls",
			req: func(r *http.Request) {
				configs.Config.Server.ClientCAFile = "/some-client.crt"
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "127.0.0.1:1234"
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(403, rsp.StatusCode)
			},
		},
		{
			name: "tls no client cert",
			req: func(r *http.Request) {
				configs.Config.Server.ClientCAFile = "/some-client.crt"
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "127.0.0.1:1234"
				r.TLS = &tls.ConnectionState{}
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(403, rsp.StatusCode)
			},
		},
		{
			// Note: we only need to add a peer certificate.
			// In real life, the client certificate validation is done by
			// the HTTP stack. Only valid certificates will be present in
			// PeerCertificates list (which is what we test).
			name: "client cert",
			req: func(r *http.Request) {
				configs.Config.Server.ClientCAFile = "/some-client.crt"
				r.Header.Set("Remote-User", "user")
				r.Header.Set("Remote-Email", "user@localhost")
				r.Header.Set("Remote-Groups", "user")
				r.RemoteAddr = "127.0.0.1:1234"
				r.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{crt},
				}
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configs.Config.Auth.Forwarded.Enabled = true
			configs.Config.Auth.Forwarded.ProvisioningEnabled = false
			defer restoreConf()

			req, err := client.NewRequest("GET", "/api/profile", nil)
			require.NoError(t, err)

			test.req(req)
			rsp := client.Request(t, req)
			test.assert(require.New(t), rsp)
		})
	}
	// })
}
