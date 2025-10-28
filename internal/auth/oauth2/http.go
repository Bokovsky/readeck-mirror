// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oauth2

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
)

// SetupRoutes mounts the routes for the oauth domain.
func SetupRoutes(s *server.Server) {
	r := chi.NewRouter()
	r.Get("/", metadataHandler)
	s.AddRoute("/.well-known/oauth-authorization-server", r)

	s.AddRoute("/api/oauth", newOAuthAPI())
	s.AddRoute("/authorize", newAuthorizeViewRouter())
	s.AddRoute("/device", newDeviceViewRouter())
}

// serverMetadata implements OAuth 2.0 Authorization Server Metadata
// https://datatracker.ietf.org/doc/html/rfc8414
type serverMetadata struct {
	Issuer                            string   `json:"issuer,omitempty"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint                     string   `json:"token_endpoint,omitempty"`
	DeviceAuthorizationEndpoint       string   `json:"device_authorization_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	ServiceDocumentation              string   `json:"service_documentation,omitempty"`
}

func metadataHandler(w http.ResponseWriter, r *http.Request) {
	meta := serverMetadata{
		Issuer:                            urls.AbsoluteURL(r, "/").String(),
		AuthorizationEndpoint:             urls.AbsoluteURL(r, "/authorize").String(),
		TokenEndpoint:                     urls.AbsoluteURL(r, "/api/oauth/token").String(),
		DeviceAuthorizationEndpoint:       urls.AbsoluteURL(r, "/api/oauth/device").String(),
		RegistrationEndpoint:              urls.AbsoluteURL(r, "/api/oauth/client").String(),
		RevocationEndpoint:                urls.AbsoluteURL(r, "/api/oauth/revoke").String(),
		GrantTypesSupported:               []string{grantTypeAuthCode, grantTypeDeviceCode},
		ResponseTypesSupported:            []string{"code"},
		ScopesSupported:                   []string{},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none", "bearer"},
	}

	for _, x := range users.GroupList(server.Locale(r), "__oauth_scope__", nil) {
		meta.ScopesSupported = append(meta.ScopesSupported, x[0])
	}

	server.Render(w, r, http.StatusOK, meta)
}
