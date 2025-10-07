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

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/oauth2"
	"codeberg.org/readeck/readeck/internal/db/types"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

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
			"registration_endpoint":"http://readeck.example.org/api/oauth/client",
			"revocation_endpoint": "http://readeck.example.org/api/oauth/revoke",
			"grant_types_supported":["authorization_code"],
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

	appClient := &oauth2.Client{
		Name:            "Test App",
		Website:         "https://example.net",
		RedirectURIs:    types.Strings{"http://[::1]:4000/callback"},
		SoftwareID:      uuid.NewString(),
		SoftwareVersion: "1.0.2",
	}
	require.NoError(t, oauth2.Clients.Create(appClient))
	appToken, err := configs.Keys.OauthClientTokenKey().Encode(appClient.UID)
	require.NoError(t, err)

	codeVerifier := "210e967c91ae52d32bf414f1439769fb0eda1f828fc8d49c78e18ac5"
	h := sha256.New()
	h.Write([]byte(codeVerifier))

	codeChallenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	params := url.Values{}
	params.Set("client_id", appClient.UID)
	params.Set("redirect_uri", "http://[::1]:4000/callback")
	params.Set("scope", "bookmarks:read bookmarks:write")
	params.Set("state", "random.state")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	tokenCode := ""
	accessToken := ""
	_ = tokenCode
	_ = accessToken

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
				tokenCode = query.Get("code")
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

	RunRequestSequence(t, client, "",
		RequestTest{
			Name:   "revoke token",
			Target: "/api/oauth/revoke",
			Method: http.MethodPost,
			Headers: map[string]string{
				"Authorization": "Bearer " + appToken,
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
				"Authorization": "Bearer " + appToken,
			},
			JSON: map[string]string{
				"token": accessToken,
			},
			ExpectStatus: 200,
		},
	)
}

func TestClientRegistration(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := NewClient(t, app)

	appClient1 := &oauth2.Client{
		Name:            "Test App 1",
		Website:         "https://example.net",
		RedirectURIs:    types.Strings{"http://[::1]:4000/callback"},
		SoftwareID:      uuid.NewString(),
		SoftwareVersion: "1.0.2",
	}
	appClient2 := &oauth2.Client{
		Name:            "Test App 2",
		Website:         "https://example.net",
		RedirectURIs:    types.Strings{"http://[::1]:4000/callback"},
		SoftwareID:      uuid.NewString(),
		SoftwareVersion: "1.0.2",
	}
	require.NoError(t, oauth2.Clients.Create(appClient1))
	require.NoError(t, oauth2.Clients.Create(appClient2))

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

	t.Run("client bearer", func(t *testing.T) {
		t1, err := configs.Keys.OauthClientTokenKey().Encode(appClient1.UID)
		require.NoError(t, err)

		t2, err := configs.Keys.OauthClientTokenKey().Encode(appClient2.UID)
		require.NoError(t, err)

		RunRequestSequence(t, client, "",
			RequestTest{
				Name:   "ok",
				Target: "/api/oauth/client/" + appClient1.UID,
				Headers: map[string]string{
					"Authorization": "Bearer " + t1,
				},
				ExpectStatus: 200,
			},
			RequestTest{
				Name:   "mismatch id and token",
				Target: "/api/oauth/client/" + appClient1.UID,
				Headers: map[string]string{
					"Authorization": "Bearer " + t2,
				},
				ExpectStatus: 401,
			},
		)
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
			Name:   "no client name",
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
				"registration_access_token": "<<PRESENCE>>",
				"registration_client_uri": "<<PRESENCE>>",
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
				"grant_types":["authorization_code"],
				"response_types":["code"]
			}`,
		},
		RequestTest{
			Name:         "client info",
			Target:       "/api/oauth/client/{{ (index .History 0).JSON.client_id }}",
			ExpectStatus: 401,
		},
		RequestTest{
			Name:   "client info",
			Target: "/api/oauth/client/{{ (index .History 1).JSON.client_id }}",
			Headers: map[string]string{
				"Authorization": "Bearer {{ (index .History 1).JSON.registration_access_token }}",
			},
			ExpectStatus: 200,
			ExpectJSON: `{
				"client_id":"<<PRESENCE>>",
				"registration_access_token": "<<PRESENCE>>",
				"registration_client_uri": "<<PRESENCE>>",
				"client_name": "test",
				"client_uri": "https://example.org/",
				"logo_uri": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+P+/HgAFhAJ/wlseKgAAAABJRU5ErkJggg==",
				"redirect_uris": [
					"https://example.org/callback",
					"https://example.org/callback2",
					"net.myapp:oauth-callback",
					"net.myapp:///oauth-callback",
					"http://127.0.0.8:8000/callback",
					"http://[::1]:8000/callback"
				],
				"software_id": "10098d11-8b3f-4ebb-b519-cab2301975fa",
				"software_version": "1.0.0",
				"token_endpoint_auth_method": "none",
				"grant_types": ["authorization_code"],
				"response_types": ["code"]
			}`,
		},
		RequestTest{
			Name:   "client update",
			Target: "/api/oauth/client/{{ (index .History 0).JSON.client_id }}",
			Method: "PUT",
			Headers: map[string]string{
				"Authorization": "Bearer {{ (index .History 0).JSON.registration_access_token }}",
			},
			JSON: map[string]any{
				"client_id":                  "{{ (index .History 0).JSON.client_id }}",
				"client_name":                "test",
				"client_uri":                 "https://example.org/",
				"logo_uri":                   "",
				"redirect_uris":              []string{"http://[::1]:8000/callback"},
				"software_id":                "10098d11-8b3f-4ebb-b519-cab2301975fa",
				"software_version":           "1.0.0",
				"token_endpoint_auth_method": "none",
				"grant_types":                []string{"authorization_code"},
				"response_types":             []string{"code"},
			},
			ExpectStatus: 200,
			ExpectJSON: `{
				"client_id":"<<PRESENCE>>",
				"registration_access_token": "<<PRESENCE>>",
				"registration_client_uri": "<<PRESENCE>>",
				"client_name":"test",
				"client_uri":"https://example.org/",
				"logo_uri":"",
				"redirect_uris":[
					"http://[::1]:8000/callback"
				],
				"software_id":"10098d11-8b3f-4ebb-b519-cab2301975fa",
				"software_version":"1.0.0",
				"token_endpoint_auth_method":"none",
				"grant_types":["authorization_code"],
				"response_types":["code"]
			}`,
		},
		RequestTest{
			Name:   "client delete",
			Target: "/api/oauth/client/{{ (index .History 0).JSON.client_id }}",
			Method: "DELETE",
			Headers: map[string]string{
				"Authorization": "Bearer {{ (index .History 0).JSON.registration_access_token }}",
			},
			ExpectStatus: 204,
		},
		RequestTest{
			Name:   "client info",
			Target: "/api/oauth/client/{{ (index .History 1).JSON.client_id }}",
			Headers: map[string]string{
				"Authorization": "Bearer {{ (index .History 1).JSON.registration_access_token }}",
			},
			ExpectStatus: 401,
		},
	)
}
