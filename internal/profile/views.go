// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package profile

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/forms"
)

// profileViews is an HTTP handler for the user profile web views.
type profileViews struct {
	chi.Router
	*profileAPI
}

// newProfileViews returns an new instance of ProfileViews.
func newProfileViews(api *profileAPI) *profileViews {
	r := api.srv.AuthenticatedRouter(api.srv.WithRedirectLogin)
	v := &profileViews{r, api}

	r.With(api.srv.WithPermission("profile", "read")).Group(func(r chi.Router) {
		r.Get("/", v.userProfile)
		r.Get("/password", v.userPassword)
	})

	r.With(api.srv.WithPermission("profile", "write")).Group(func(r chi.Router) {
		r.Post("/", v.userProfile)
		r.Post("/password", v.userPassword)
		r.Post("/session", v.userSession)
	})

	r.With(api.srv.WithPermission("profile:tokens", "read")).Group(func(r chi.Router) {
		r.With(api.withTokenList).Get("/tokens", v.tokenList)
		r.With(api.withToken).Get("/tokens/{uid}", v.tokenInfo)
	})

	r.With(api.srv.WithPermission("profile:tokens", "write")).Group(func(r chi.Router) {
		r.Post("/tokens", v.tokenCreate)
		r.With(api.withToken).Post("/tokens/{uid}", v.tokenInfo)
		r.With(api.withToken).Post("/tokens/{uid}/delete", v.tokenDelete)
	})

	return v
}

// userProfile handles GET and POST requests on /profile.
func (v *profileViews) userProfile(w http.ResponseWriter, r *http.Request) {
	tr := v.srv.Locale(r)
	user := auth.GetRequestUser(r)
	f := newProfileForm(v.srv.Locale(r))
	f.setUser(user)

	if r.Method == http.MethodPost {
		forms.Bind(f, r)
		if f.IsValid() {
			if _, err := f.updateUser(user); err != nil {
				v.srv.Log(r).Error("", slog.Any("err", err))
			} else {
				// Set the new seed in the session.
				// We needn't save the session since AddFlash does that already.
				sess := v.srv.GetSession(r)
				sess.Payload.Seed = user.Seed
				v.srv.AddFlash(w, r, "success", tr.Gettext("Profile updated."))
				v.srv.Redirect(w, r, "profile")
				return
			}
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	ctx := server.TC{
		"Form":     f,
		"MailFrom": configs.Config.Email.FromNoReply.Addr(),
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Profile")},
	})

	v.srv.RenderTemplate(w, r, 200, "profile/index", ctx)
}

// userPassword handles GET and POST requests on /profile/password.
func (v *profileViews) userPassword(w http.ResponseWriter, r *http.Request) {
	tr := v.srv.Locale(r)
	f := newPasswordForm(tr)

	if r.Method == http.MethodPost {
		user := auth.GetRequestUser(r)
		f.setUser(user)
		forms.Bind(f, r)
		if f.IsValid() {
			if err := f.updatePassword(user); err != nil {
				v.srv.Log(r).Error("", slog.Any("err", err))
			} else {
				// Set the new seed in the session.
				// We needn't save the session since AddFlash does it already.
				sess := v.srv.GetSession(r)
				sess.Payload.Seed = user.Seed
				v.srv.AddFlash(w, r, "success", tr.Gettext("Your password was changed."))
				v.srv.Redirect(w, r, "password")
				return
			}
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	ctx := server.TC{
		"Form": f,
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Profile"), urls.AbsoluteURL(r, "/profile").String()},
		{tr.Gettext("Password")},
	})
	v.srv.RenderTemplate(w, r, 200, "profile/password", ctx)
}

// userSession handles changes of user session preferences.
// This returns an API response but since it only works with a SessionAuthProvider
// it makes more sense to have it in the views.
func (v *profileViews) userSession(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.GetRequestProvider(r).(*auth.SessionAuthProvider)
	if !ok {
		v.srv.TextMessage(w, r, http.StatusBadRequest, "invalid authentication provider")
		return
	}

	f := newSessionPrefForm(v.srv.Locale(r))
	forms.Bind(f, r)

	if !f.IsValid() {
		v.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	sess := p.GetSession(r)
	updated, err := f.updateSession(sess.Payload)
	if err != nil {
		v.srv.Error(w, r, err)
		return
	}

	sess.Save(w, r)
	v.srv.Render(w, r, http.StatusOK, updated)
}

func (v *profileViews) tokenList(w http.ResponseWriter, r *http.Request) {
	tl := r.Context().Value(ctxTokenListKey{}).(tokenList)
	tr := v.srv.Locale(r)

	ctx := server.TC{
		"Pagination": tl.Pagination,
		"Tokens":     tl.Items,
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Profile"), urls.AbsoluteURL(r, "/profile").String()},
		{tr.Gettext("API Tokens")},
	})

	v.srv.RenderTemplate(w, r, 200, "profile/token_list", ctx)
}

func (v *profileViews) tokenCreate(w http.ResponseWriter, r *http.Request) {
	t := &tokens.Token{
		UserID:      &auth.GetRequestUser(r).ID,
		IsEnabled:   true,
		Application: "internal",
	}
	tr := v.srv.Locale(r)
	if err := tokens.Tokens.Create(t); err != nil {
		v.srv.Log(r).Error("server error", slog.Any("err", err))
		v.srv.AddFlash(w, r, "error", tr.Gettext("An error occurred while creating your token."))
		v.srv.Redirect(w, r, "tokens")
		return
	}

	v.srv.AddFlash(w, r, "success", tr.Gettext("New token created."))
	v.srv.Redirect(w, r, ".", t.UID)
}

func (v *profileViews) tokenInfo(w http.ResponseWriter, r *http.Request) {
	tr := v.srv.Locale(r)
	ti := r.Context().Value(ctxtTokenKey{}).(tokenItem)
	f := newTokenForm(v.srv.Locale(r), auth.GetRequestUser(r))

	if r.Method == http.MethodGet {
		f.setToken(ti.Token)
	}

	if r.Method == http.MethodPost {
		forms.Bind(f, r)
		if f.IsValid() {
			if err := f.updateToken(ti.Token); err != nil {
				v.srv.Log(r).Error("", slog.Any("err", err))
			} else {
				v.srv.AddFlash(w, r, "success", tr.Gettext("Token was updated."))
				v.srv.Redirect(w, r, ti.UID)
				return
			}
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	token, err := tokens.EncodeToken(ti.UID)
	if err != nil {
		v.srv.Status(w, r, http.StatusInternalServerError)
		return
	}

	ctx := server.TC{
		"Token":   ti,
		"Encoded": token,
		"Form":    f,
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Profile"), urls.AbsoluteURL(r, "/profile").String()},
		{tr.Gettext("API Tokens"), urls.AbsoluteURL(r, "/profile/tokens").String()},
		{ti.UID},
	})

	v.srv.RenderTemplate(w, r, 200, "profile/token", ctx)
}

func (v *profileViews) tokenDelete(w http.ResponseWriter, r *http.Request) {
	f := newDeleteTokenForm(v.srv.Locale(r))
	f.Get("_to").Set("/profile/tokens")
	forms.Bind(f, r)

	ti := r.Context().Value(ctxtTokenKey{}).(tokenItem)

	if err := f.trigger(ti.Token); err != nil {
		v.srv.Error(w, r, err)
		return
	}
	v.srv.Redirect(w, r, f.Get("_to").String())
}
