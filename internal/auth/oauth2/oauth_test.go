// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oauth2_test

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func registerClient(t *testing.T, client *Client) string {
	rsp := client.RequestJSON(http.MethodPost, "/api/oauth/client", map[string]any{
		"client_name":      "Test App",
		"client_uri":       "https://example.net/",
		"software_id":      uuid.NewString(),
		"software_version": "1.0.2",
		"redirect_uris":    []string{"http://[::1]:4000/callback"},
	})
	require.Equal(t, 201, rsp.StatusCode)
	return rsp.JSON.(map[string]any)["client_id"].(string)
}

func TestServerMetadata(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := NewClient(t, app)

	RunRequestSequence(t, client, "", RequestTest{
		Target:       "/.well-known/oauth-authorization-server",
		ExpectStatus: http.StatusOK,
		ExpectJSON: `{
			"issuer":"http://readeck.example.org/",
			"authorization_endpoint":"http://readeck.example.org/authorize",
			"token_endpoint":"http://readeck.example.org/api/oauth/token",
			"device_authorization_endpoint":"http://readeck.example.org/api/oauth/device",
			"registration_endpoint":"http://readeck.example.org/api/oauth/client",
			"revocation_endpoint": "http://readeck.example.org/api/oauth/revoke",
			"grant_types_supported":["authorization_code", "urn:ietf:params:oauth:grant-type:device_code"],
			"response_types_supported":["code"],
			"scopes_supported":["bookmarks:read","bookmarks:write","profile:read"],
			"code_challenge_methods_supported":["S256"],
			"token_endpoint_auth_methods_supported": ["none", "bearer"]
		}`,
	})
}

func TestAuthorizationCodeFlow(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := NewClient(t, app)

	t.Run("authorization form", func(t *testing.T) {
		clientID := registerClient(t, client)

		codeVerifier := "210e967c91ae52d32bf414f1439769fb0eda1f828fc8d49c78e18ac5"
		h := sha256.New()
		h.Write([]byte(codeVerifier))

		codeChallenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
		params := url.Values{}
		params.Set("client_id", clientID)
		params.Set("redirect_uri", "http://[::1]:4000/callback")
		params.Set("scope", "bookmarks:read bookmarks:write")
		params.Set("state", "random.state")
		params.Set("code_challenge", codeChallenge)
		params.Set("code_challenge_method", "S256")

		RunRequestSequence(t, client, "", RequestTest{
			Target:       "/authorize",
			ExpectStatus: 303,
		})

		RunRequestSequence(t, client, "user",
			RequestTest{
				Name:         "authorize ko",
				Target:       "/authorize",
				ExpectStatus: 401,
				ExpectJSON:   `{"error": "invalid_client"}`,
			},
			RequestTest{
				Name:           "authorize form",
				Target:         "/authorize?" + params.Encode(),
				ExpectContains: `Authorize</button>`,
			},
			RequestTest{
				Name:   "authorize ok",
				Target: "{{ (index .History 0).URL }}",
				Method: http.MethodPost,
				Form: url.Values{
					"granted": []string{"1"},
				},
				ExpectStatus: http.StatusFound,
				Assert: func(t *testing.T, r *Response) {
					assert := require.New(t)
					location := r.Header.Get("location")
					assert.NotEmpty(location)
					u, err := url.Parse(location)
					assert.NoError(err)

					query := u.Query()
					assert.Contains(query, "code")
					assert.NotEmpty(query.Get("code"))
					assert.Equal("random.state", query.Get("state"))
				},
			},
			RequestTest{
				Name:   "authorize deny",
				Target: "{{ (index .History 0).URL }}",
				Method: http.MethodPost,
				Form: url.Values{
					"granted": []string{"0"},
				},
				ExpectStatus: http.StatusFound,
				Assert: func(t *testing.T, r *Response) {
					assert := require.New(t)
					location := r.Header.Get("location")
					assert.NotEmpty(location)
					u, err := url.Parse(location)
					assert.NoError(err)

					query := u.Query()
					assert.Equal("access_denied", query.Get("error"))
					assert.Equal("access denied", query.Get("error_description"))
					assert.Contains(query, "code")
					assert.Empty(query.Get("code"))
				},
			},
			RequestTest{
				Name:         "client gone after deny",
				Target:       "{{ (index .History 0).URL }}",
				Method:       http.MethodGet,
				ExpectStatus: http.StatusUnauthorized,
			},
		)
	})

	t.Run("token", func(t *testing.T) {
		clientID := registerClient(t, client)

		codeVerifier := "210e967c91ae52d32bf414f1439769fb0eda1f828fc8d49c78e18ac5"
		h := sha256.New()
		h.Write([]byte(codeVerifier))

		codeChallenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
		params := url.Values{}
		params.Set("client_id", clientID)
		params.Set("redirect_uri", "http://[::1]:4000/callback")
		params.Set("scope", "bookmarks:read bookmarks:write")
		params.Set("state", "random.state")
		params.Set("code_challenge", codeChallenge)
		params.Set("code_challenge_method", "S256")

		tokenCode := ""
		accessToken := ""

		RunRequestSequence(t, client, "user",
			RequestTest{
				Name:           "authorize form",
				Target:         "/authorize?" + params.Encode(),
				ExpectContains: `Authorize</button>`,
			},
			RequestTest{
				Name:   "authorize ok",
				Target: "{{ (index .History 0).URL }}",
				Method: http.MethodPost,
				Form: url.Values{
					"granted": []string{"1"},
				},
				ExpectStatus: http.StatusFound,
				Assert: func(t *testing.T, r *Response) {
					u, err := url.Parse(r.Header.Get("location"))
					require.NoError(t, err)

					query := u.Query()
					tokenCode = query.Get("code")
				},
			},
		)

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:         "token challenge ko",
				Target:       "/api/oauth/token",
				Method:       http.MethodPost,
				Form:         url.Values{},
				ExpectStatus: 400,
				ExpectJSON: `{
					"error":"invalid_request",
					"error_description":"error on field \"grant_type\": field is required"
				}`,
			},
			RequestTest{
				Name:   "token challenge ko",
				Target: "/api/oauth/token",
				Method: http.MethodPost,
				Form: url.Values{
					"grant_type":    []string{"authorization_code"},
					"code":          []string{"pnIEw47"},
					"code_verifier": []string{"FaRhB"},
				},
				ExpectStatus: 400,
				ExpectJSON: `{
					"error":"invalid_grant",
					"error_description":"code is not valid"
				}`,
			},
			RequestTest{
				Name:   "token challenge ko",
				Target: "/api/oauth/token",
				Method: http.MethodPost,
				Form: url.Values{
					"grant_type":    []string{"authorization_code"},
					"code":          []string{tokenCode},
					"code_verifier": []string{"8GrkY4Fk1XpnIEw47w71XDEoMFaRhB8SvLhs9ZLCCNI"},
				},
				ExpectStatus: 400,
				ExpectJSON: `{
					"error":"invalid_grant",
					"error_description":"code is not valid"
				}`,
			},
			RequestTest{
				Name:   "token ok",
				Target: "/api/oauth/token",
				Method: http.MethodPost,
				Form: url.Values{
					"grant_type":    []string{"authorization_code"},
					"code":          []string{tokenCode},
					"code_verifier": []string{codeVerifier},
				},
				ExpectStatus: http.StatusCreated,
				ExpectJSON: `{
					"access_token": "<<PRESENCE>>",
					"id": "<<PRESENCE>>",
					"scope": "bookmarks:read bookmarks:write",
					"token_type": "Bearer"
				}`,
				Assert: func(t *testing.T, r *Response) {
					accessToken = r.JSON.(map[string]any)["access_token"].(string)
					t.Log(accessToken)
					require.Empty(t, Store().Get("oauth:client:"+clientID))
				},
			},
		)

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:   "profile with new token",
				Target: "/api/profile",
				Headers: map[string]string{
					"Authorization": "Bearer " + accessToken,
				},
				ExpectStatus: 200,
			},
		)

		RunRequestSequence(t, client, "user",
			RequestTest{
				Name:   "revoke token with session auth",
				Target: "/api/oauth/revoke",
				Method: http.MethodPost,
				JSON: map[string]string{
					"token": accessToken,
				},
				ExpectStatus: 400,
				ExpectJSON: `{
					"error":"access_denied"
				}`,
			},
		)

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:   "revoke token",
				Target: "/api/oauth/revoke",
				Method: http.MethodPost,
				Headers: map[string]string{
					"Authorization": "Bearer " + accessToken,
				},
				JSON: map[string]string{
					"token": accessToken,
				},
				ExpectStatus: 200,
			},
			RequestTest{
				Name:   "profile with revoked token",
				Target: "/api/profile",
				Headers: map[string]string{
					"Authorization": "Bearer " + accessToken,
				},
				ExpectStatus: 401,
			},
			RequestTest{
				Name:   "revoke token already revoked",
				Target: "/api/oauth/revoke",
				Method: http.MethodPost,
				Headers: map[string]string{
					"Authorization": "Bearer " + accessToken,
				},
				JSON: map[string]string{
					"token": accessToken,
				},
				ExpectStatus: 401,
			},
		)
	})
}

func TestDeviceCodeFlow(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := NewClient(t, app)

	t.Run("granted access", func(t *testing.T) {
		clientID := registerClient(t, client)
		deviceCode := ""
		userCode := ""

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:   "device code request",
				Target: "/api/oauth/device",
				Method: http.MethodPost,
				Form: url.Values{
					"client_id": {clientID},
					"scope":     {"bookmarks:read"},
				},
				ExpectStatus: 200,
				ExpectJSON: `{
					"device_code": "<<PRESENCE>>",
					"user_code": "<<PRESENCE>>",
					"verification_uri": "http://readeck.example.org/device",
					"verification_uri_complete": "<<PRESENCE>>",
					"expires_in": 300,
					"interval": 5
				}`,
				Assert: func(t *testing.T, r *Response) {
					deviceCode = r.JSON.(map[string]any)["device_code"].(string)
					userCode = r.JSON.(map[string]any)["user_code"].(string)
					require.NotEqual(t, deviceCode, userCode)

					fullURI := r.JSON.(map[string]any)["verification_uri_complete"].(string)
					require.Equal(t,
						"http://readeck.example.org/device?user_code="+userCode,
						fullURI,
					)

					require.NotEmpty(t, Store().Get("oauth:device-code:"+userCode))
				},
			},
			RequestTest{
				Name:   "pending token",
				Target: "/api/oauth/token",
				Method: http.MethodPost,
				JSON: map[string]any{
					"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
					"device_code": "{{ (index .History 0).JSON.device_code }}",
					"client_id":   clientID,
				},
				ExpectStatus: 400,
				ExpectJSON:   `{"error": "authorization_pending"}`,
			},
			RequestTest{
				Name:   "slow down",
				Target: "/api/oauth/token",
				Method: http.MethodPost,
				JSON: map[string]any{
					"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
					"device_code": "{{ (index .History 1).JSON.device_code }}",
					"client_id":   clientID,
				},
				ExpectStatus: 400,
				ExpectJSON:   `{"error": "slow_down"}`,
			},
		)

		RunRequestSequence(t, client, "user",
			RequestTest{
				Name:           "authorization page",
				Target:         "/device",
				Method:         http.MethodGet,
				ExpectContains: "Enter the code displayed on your device",
			},
			RequestTest{
				Name:           "authorization page",
				Target:         "/device?user_code=" + userCode,
				Method:         http.MethodGet,
				ExpectStatus:   200,
				ExpectContains: "would like permission to access your account",
			},
			RequestTest{
				Name:           "authorization page",
				Target:         "/device?user_code=abc",
				Method:         http.MethodGet,
				ExpectStatus:   400,
				ExpectContains: "This code has expired or is not valid",
			},
			RequestTest{
				Name:   "grant authorization",
				Target: "/device",
				Method: http.MethodPost,
				Form: url.Values{
					"user_code": {userCode},
					"granted":   {"1"},
				},
				ExpectStatus: http.StatusSeeOther,
				Assert: func(t *testing.T, r *Response) {
					require.Equal(t,
						"http://readeck.example.org/device?user_code="+userCode,
						r.Header.Get("Location"),
					)
				},
			},
			RequestTest{
				Name:           "authorization granted",
				Target:         "{{ (index .History 0).Redirect }}",
				Method:         http.MethodGet,
				ExpectStatus:   200,
				ExpectContains: "Authorization granted. Connecting device...",
				Assert: func(t *testing.T, r *Response) {
					require.Equal(t, "6", r.Header.Get("Refresh"))
				},
			},
		)

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:   "retrieve token",
				Target: "/api/oauth/token",
				Method: http.MethodPost,
				JSON: map[string]any{
					"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
					"device_code": deviceCode,
					"client_id":   clientID,
				},
				ExpectStatus: 201,
				ExpectJSON: `{
					"id": "<<PRESENCE>>",
					"access_token": "<<PRESENCE>>",
					"token_type": "Bearer",
					"scope": "bookmarks:read"
				}`,
			},
			RequestTest{
				Name:   "profile with new token",
				Target: "/api/profile",
				Headers: map[string]string{
					"Authorization": "Bearer {{ (index .History 0).JSON.access_token }}",
				},
				ExpectStatus: 200,
			},
		)
	})

	t.Run("denied access", func(t *testing.T) {
		clientID := registerClient(t, client)
		deviceCode := ""
		userCode := ""

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:   "device code request",
				Target: "/api/oauth/device",
				Method: http.MethodPost,
				Form: url.Values{
					"client_id": {clientID},
					"scope":     {"bookmarks:read"},
				},
				ExpectStatus: 200,
				Assert: func(_ *testing.T, r *Response) {
					deviceCode = r.JSON.(map[string]any)["device_code"].(string)
					userCode = r.JSON.(map[string]any)["user_code"].(string)
				},
			},
		)
		RunRequestSequence(t, client, "user",
			RequestTest{
				Name:   "deny authorization",
				Target: "/device",
				Method: http.MethodPost,
				Form: url.Values{
					"user_code": {userCode},
					"granted":   {"0"},
				},
				ExpectStatus: http.StatusSeeOther,
				Assert: func(t *testing.T, r *Response) {
					require.Equal(t,
						"http://readeck.example.org/device?user_code="+userCode,
						r.Header.Get("Location"),
					)
				},
			},
			RequestTest{
				Name:           "authorization denied",
				Target:         "{{ (index .History 0).Redirect }}",
				Method:         http.MethodGet,
				ExpectStatus:   200,
				ExpectContains: "Authorization request was denied.",
				Assert: func(t *testing.T, r *Response) {
					require.Empty(t, r.Header.Get("Refresh"))
				},
			},
		)

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:   "token denied",
				Target: "/api/oauth/token",
				Method: http.MethodPost,
				JSON: map[string]any{
					"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
					"device_code": deviceCode,
					"client_id":   clientID,
				},
				ExpectStatus: 400,
				ExpectJSON:   `{"error": "access_denied"}`,
			},
		)
	})

	t.Run("expired code", func(t *testing.T) {
		clientID := registerClient(t, client)
		deviceCode := ""
		userCode := ""

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:   "device code request",
				Target: "/api/oauth/device",
				Method: http.MethodPost,
				Form: url.Values{
					"client_id": {clientID},
					"scope":     {"bookmarks:read"},
				},
				ExpectStatus: 200,
				Assert: func(_ *testing.T, r *Response) {
					deviceCode = r.JSON.(map[string]any)["device_code"].(string)
					userCode = r.JSON.(map[string]any)["user_code"].(string)
				},
			},
		)

		// Code has expired
		require.NoError(t, Store().Del("oauth:device-code:"+userCode))

		RunRequestSequence(t, client, "user",
			RequestTest{
				Name:   "expired authorization",
				Target: "/device",
				Method: http.MethodPost,
				Form: url.Values{
					"user_code": {userCode},
					"granted":   {"0"},
				},
				ExpectStatus:   http.StatusBadRequest,
				ExpectContains: "This code has expired or is not valid",
			},
		)

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:   "token denied",
				Target: "/api/oauth/token",
				Method: http.MethodPost,
				JSON: map[string]any{
					"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
					"device_code": deviceCode,
					"client_id":   clientID,
				},
				ExpectStatus: 400,
				ExpectJSON:   `{"error": "expired_token"}`,
			},
		)
	})
}

func TestClientRegistration(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := NewClient(t, app)

	t.Run("invalid_redirect_uri", func(t *testing.T) {
		tests := []string{
			"http://test.localhost/",
			"http://localhost/",
			"http://example.org/",
			"https://",
		}

		seq := []RequestTest{}
		for _, test := range tests {
			seq = append(seq, RequestTest{
				Target: "/api/oauth/client",
				Method: "POST",
				JSON: map[string]any{
					"redirect_uris": []string{
						test,
					},
				},
				ExpectJSON: `{"error":"invalid_redirect_uri","error_description":"invalid URL"}`,
			})
		}
		RunRequestSequence(t, client, "", seq...)
	})

	t.Run("invalid client_uri", func(t *testing.T) {
		tests := []string{
			"http://example.com/",
			"https://192.168.0.1:4000/test",
			"https://[fc00::1]/",
			"https://localhost/",
			"https://test.localhost/",
		}

		seq := []RequestTest{}
		for _, test := range tests {
			seq = append(seq, RequestTest{
				Name:   "no client URI",
				Target: "/api/oauth/client",
				Method: "POST",
				JSON: map[string]any{
					"client_name":   "test",
					"client_uri":    test,
					"redirect_uris": []string{"https://example.org/callback"},
				},
				ExpectJSON: `{
				"error": "invalid_client_metadata",
				"error_description": "error on field \"client_uri\": invalid client URI"
			}`,
			})
		}
		RunRequestSequence(t, client, "", seq...)
	})

	RunRequestSequence(t, client, "",
		RequestTest{
			Name:   "no client name",
			Target: "/api/oauth/client",
			Method: "POST",
			JSON: map[string]any{
				"redirect_uris": []string{"https://example.org/callback"},
			},
			ExpectJSON: `{
				"error": "invalid_client_metadata",
				"error_description": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			Name:   "no client URI",
			Target: "/api/oauth/client",
			Method: "POST",
			JSON: map[string]any{
				"client_name":   "test",
				"redirect_uris": []string{"https://example.org/callback"},
			},
			ExpectJSON: `{
				"error": "invalid_client_metadata",
				"error_description": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			Name:   "create client",
			Target: "/api/oauth/client",
			Method: "POST",
			JSON: map[string]any{
				"client_name":      "test",
				"client_uri":       "https://example.org/",
				"logo_uri":         "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+P+/HgAFhAJ/wlseKgAAAABJRU5ErkJggg==",
				"software_id":      "10098d11-8b3f-4ebb-b519-cab2301975fa",
				"software_version": "1.0.0",
				"redirect_uris": []string{
					"https://example.org/callback",
					"https://example.org/callback2",
					"net.myapp:oauth-callback",
					"net.myapp:///oauth-callback",
					"http://127.0.0.8:8000/callback",
					"http://[::1]:8000/callback",
				},
			},
			ExpectStatus: 201,
			ExpectJSON: `{
				"client_id":"<<PRESENCE>>",
				"client_name": "test",
				"client_uri": "https://example.org/",
				"logo_uri": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+P+/HgAFhAJ/wlseKgAAAABJRU5ErkJggg==",
				"redirect_uris":[
					"https://example.org/callback",
					"https://example.org/callback2",
					"net.myapp:oauth-callback",
					"net.myapp:///oauth-callback",
					"http://127.0.0.8:8000/callback",
					"http://[::1]:8000/callback"
				],
				"software_id":"10098d11-8b3f-4ebb-b519-cab2301975fa",
				"software_version":"1.0.0",
				"token_endpoint_auth_method":"none",
				"grant_types":["authorization_code","urn:ietf:params:oauth:grant-type:device_code"],
				"response_types":["code"]
			}`,
		},
	)
}
