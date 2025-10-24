// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oauth2

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/ctxr"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type (
	ctxClientKey struct{}
)

var withClient, getClient = ctxr.WithGetter[*Client](ctxClientKey{})

type clientResponse struct {
	ID                      string   `json:"client_id"`
	ClientURI               string   `json:"registration_client_uri"`
	AccessToken             string   `json:"registration_access_token"`
	Name                    string   `json:"client_name"`
	URI                     string   `json:"client_uri"`
	Logo                    string   `json:"logo_uri"`
	RedirectURIs            []string `json:"redirect_uris"`
	SoftwareID              string   `json:"software_id"`
	SoftwareVersion         string   `json:"software_version"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

func newClientResponse(ctx context.Context, client *Client) clientResponse {
	token, _ := configs.Keys.OauthClientTokenKey().Encode(client.UID)

	return clientResponse{
		ID:                      client.UID,
		ClientURI:               urls.AbsoluteURLContext(ctx, "/api/oauth/client", client.UID).String(),
		AccessToken:             token,
		Name:                    client.Name,
		URI:                     client.Website,
		RedirectURIs:            client.RedirectURIs,
		Logo:                    client.Logo,
		SoftwareID:              client.SoftwareID,
		SoftwareVersion:         client.SoftwareVersion,
		TokenEndpointAuthMethod: "none",
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
	}
}

type clientAPI struct {
	chi.Router
}

func newClientAPI() *clientAPI {
	api := &clientAPI{chi.NewRouter()}

	api.Post("/", api.clientCreate)
	api.With(
		api.withClient,
	).Route("/{uid}", func(r chi.Router) {
		r.Get("/", api.clientInfo)
		r.Put("/", api.clientUpdate)
		r.Delete("/", api.clientDelete)
	})

	return api
}

// withAuthenticatedClient retrieves the client from a provided
// bearer token.
func withAuthenticatedClient(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessToken, ok := strings.CutPrefix(r.Header.Get("authorization"), "Bearer ")
		if !ok {
			server.Err(w, r, errInvalidClient)
			return
		}

		accessToken = strings.TrimSpace(accessToken)
		token, err := configs.Keys.OauthClientTokenKey().Decode(accessToken)
		if err != nil {
			server.Err(w, r, errInvalidClient)
			return
		}

		client, err := Clients.GetOne(goqu.C("uid").Eq(token))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				server.Err(w, r, errInvalidClient)
			} else {
				server.Err(w, r, errServerError.withError(err))
			}
			return
		}

		ctx := withClient(r.Context(), client)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withClient wraps [withAuthenticatedClient] and checks that the
// path's client ID matches the authenticated client.
func (api *clientAPI) withClient(next http.Handler) http.Handler {
	return withAuthenticatedClient(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if getClient(r.Context()).UID != chi.URLParam(r, "uid") {
				server.Err(w, r, errInvalidClient)
				return
			}
			next.ServeHTTP(w, r)
		}),
	)
}

func (api *clientAPI) clientCreate(w http.ResponseWriter, r *http.Request) {
	f := newClientForm(server.Locale(r))
	forms.Bind(f, r)

	if !f.IsValid() {
		server.Err(w, r, f.getError())
		return
	}

	client, err := f.createClient()
	if err != nil {
		server.Err(w, r, errServerError.withError(err))
		return
	}

	server.Render(w, r, http.StatusCreated, newClientResponse(r.Context(), client))
}

func (api *clientAPI) clientInfo(w http.ResponseWriter, r *http.Request) {
	server.Render(w, r, http.StatusOK,
		newClientResponse(r.Context(), getClient(r.Context())),
	)
}

func (api *clientAPI) clientUpdate(w http.ResponseWriter, r *http.Request) {
	client := getClient(r.Context())

	f := newClientForm(server.Locale(r))
	f.setClient(client)
	forms.Bind(f, r)

	if !f.IsValid() {
		server.Err(w, r, f.getError())
		return
	}

	res, err := f.updateClient(client)
	if err != nil {
		server.Err(w, r, errServerError.withError(err))
		return
	}
	if len(res) > 0 {
		client, _ = Clients.GetOne(goqu.C("id").Eq(client.ID))
	}

	server.Render(w, r, http.StatusOK, newClientResponse(r.Context(), client))
}

func (api *clientAPI) clientDelete(w http.ResponseWriter, r *http.Request) {
	if err := getClient(r.Context()).Delete(); err != nil {
		server.Err(w, r, errServerError.withError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
