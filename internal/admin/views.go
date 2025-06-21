// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package admin

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/forms"
)

// adminViews is an HTTP handler for the user profile web views.
type adminViews struct {
	chi.Router
	*adminAPI
}

func newAdminViews(api *adminAPI) *adminViews {
	r := api.srv.AuthenticatedRouter(api.srv.WithRedirectLogin)
	h := &adminViews{r, api}

	r.With(api.srv.WithPermission("admin:users", "read")).Group(func(r chi.Router) {
		r.With(api.withUserList).Get("/", h.main)
		r.With(api.withUserList).Get("/users", h.userList)
		r.Get("/users/add", h.userCreate)
		r.With(api.withUser).Get("/users/{uid:[a-zA-Z0-9]{18,22}}", h.userInfo)
	})

	r.With(api.srv.WithPermission("admin:users", "write")).Group(func(r chi.Router) {
		r.Post("/users/add", h.userCreate)
		r.With(api.withUser).Post("/users/{uid:[a-zA-Z0-9]{18,22}}", h.userInfo)
		r.With(api.withUser).Post("/users/{uid:[a-zA-Z0-9]{18,22}}/delete", h.userDelete)
	})

	return h
}

func (h *adminViews) main(w http.ResponseWriter, r *http.Request) {
	h.srv.Redirect(w, r, "./users")
}

func (h *adminViews) userList(w http.ResponseWriter, r *http.Request) {
	tr := h.srv.Locale(r)
	ul := r.Context().Value(ctxUserListKey{}).(userList)
	ul.Items = make([]userItem, len(ul.items))
	for i, item := range ul.items {
		ul.Items[i] = newUserItem(r, item, ".")
	}

	ctx := server.TC{
		"Pagination": ul.Pagination,
		"Users":      ul.Items,
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Users")},
	})

	h.srv.RenderTemplate(w, r, 200, "/admin/user_list", ctx)
}

func (h *adminViews) userCreate(w http.ResponseWriter, r *http.Request) {
	tr := h.srv.Locale(r)
	f := users.NewUserForm(h.srv.Locale(r))
	f.Get("group").Set("user")

	if r.Method == http.MethodPost {
		forms.Bind(f, r)
		if f.IsValid() {
			u, err := f.CreateUser()
			if err != nil {
				h.srv.Log(r).Error("", slog.Any("err", err))
			} else {
				h.srv.AddFlash(w, r, "success", tr.Gettext("User created."))
				h.srv.Redirect(w, r, "./..", u.UID)
				return
			}
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	ctx := server.TC{
		"Form": f,
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Users"), urls.AbsoluteURL(r, "/admin/users").String()},
		{tr.Gettext("New User")},
	})
	h.srv.RenderTemplate(w, r, 200, "/admin/user_create", ctx)
}

func (h *adminViews) userInfo(w http.ResponseWriter, r *http.Request) {
	tr := h.srv.Locale(r)
	u := r.Context().Value(ctxUserKey{}).(*users.User)
	item := newUserItem(r, u, "./..")

	f := users.NewUserForm(h.srv.Locale(r))
	f.SetUser(u)

	if r.Method == http.MethodPost {
		forms.Bind(f, r)

		if f.IsValid() {
			if _, err := f.UpdateUser(u); err != nil {
				h.srv.Log(r).Error("", slog.Any("err", err))
			} else {
				// Refresh session if same user
				if auth.GetRequestUser(r).ID == u.ID {
					sess := h.srv.GetSession(r)
					sess.Payload.User = u.ID
					sess.Payload.Seed = u.Seed
				}
				h.srv.AddFlash(w, r, "success", tr.Gettext("User updated."))
				h.srv.Redirect(w, r, u.UID)
				return
			}
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	ctx := server.TC{
		"User": item,
		"Form": f,
	}
	ctx.SetBreadcrumbs([][2]string{
		{tr.Gettext("Users"), urls.AbsoluteURL(r, "/admin/users").String()},
		{item.Username},
	})

	h.srv.RenderTemplate(w, r, 200, "/admin/user", ctx)
}

func (h *adminViews) userDelete(w http.ResponseWriter, r *http.Request) {
	f := newDeleteForm(h.srv.Locale(r))
	f.Get("_to").Set("/admin/users")
	forms.Bind(f, r)

	u := r.Context().Value(ctxUserKey{}).(*users.User)
	if u.ID == auth.GetRequestUser(r).ID {
		h.srv.Error(w, r, errSameUser)
		return
	}

	if err := f.trigger(u); err != nil {
		h.srv.Error(w, r, err)
		return
	}
	h.srv.Redirect(w, r, f.Get("_to").String())
}
