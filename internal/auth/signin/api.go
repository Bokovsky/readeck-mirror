// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package signin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type authAPI struct {
	chi.Router
	srv *server.Server
}

func newAuthAPI(s *server.Server) *authAPI {
	r := chi.NewRouter()
	r.Use(
		server.Csrf,
	)

	api := &authAPI{Router: r, srv: s}
	api.Post("/", api.auth)

	return api
}

// auth performs the user authentication with its username and
// password and then, returns a token tied to this user.
func (api *authAPI) auth(w http.ResponseWriter, r *http.Request) {
	f := newTokenLoginForm(server.Locale(r))

	forms.Bind(f, r)

	if !f.IsValid() {
		server.Render(w, r, http.StatusBadRequest, f)
		return
	}

	user := checkUser(f)
	if !f.IsValid() || user == nil {
		server.Msg(w, r, &server.Message{
			Status:  http.StatusForbidden,
			Message: errInvalidLogin.Error(),
		})
		return
	}

	t := &tokens.Token{
		UserID:      &user.ID,
		IsEnabled:   true,
		Application: f.Get("application").String(),
	}

	if roles, ok := f.Get("roles").Value().([]string); ok {
		t.Roles = roles
	}

	if err := tokens.Tokens.Create(t); err != nil {
		server.Err(w, r, err)
		return
	}

	token, err := tokens.EncodeToken(t.UID)
	if err != nil {
		server.Err(w, r, err)
		return
	}

	server.Render(w, r, http.StatusCreated, tokenReturn{
		UID:   t.UID,
		Token: token,
	})
}

type tokenReturn struct {
	UID   string `json:"id"`
	Token string `json:"token"`
}
