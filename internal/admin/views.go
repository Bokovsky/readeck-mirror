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
	r := server.AuthenticatedRouter(server.WithRedirectLogin)
	h := &adminViews{r, api}

	r.With(server.WithPermission("admin:users", "read")).Group(func(r chi.Router) {
		r.With(api.withUserList).Get("/", h.main)
		r.With(api.withUserList).Get("/users", h.userList)
		r.Get("/users/add", h.userCreate)
		r.With(api.withUser).Get("/users/{uid:[a-zA-Z0-9]{18,22}}", h.userInfo)
	})

	r.With(server.WithPermission("admin:users", "write")).Group(func(r chi.Router) {
		r.Post("/users/add", h.userCreate)
		r.With(api.withUser).Post("/users/{uid:[a-zA-Z0-9]{18,22}}", h.userInfo)
		r.With(api.withUser).Post("/users/{uid:[a-zA-Z0-9]{18,22}}/delete", h.userDelete)
	})

	return h
}

func (h *adminViews) main(w http.ResponseWriter, r *http.Request) {
	server.Redirect(w, r, "./users")
}

func (h *adminViews) userList(w http.ResponseWriter, r *http.Request) {
	tr := server.Locale(r)
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

	server.RenderTemplate(w, r, 200, "/admin/user_list", ctx)
}

func (h *adminViews) userCreate(w http.ResponseWriter, r *http.Request) {
	tr := server.Locale(r)
	f := users.NewUserForm(server.Locale(r))
	f.Get("group").Set("user")

	if r.Method == http.MethodPost {
		forms.Bind(f, r)
		if f.IsValid() {
			u, err := f.CreateUser()
			if err != nil {
				server.Log(r).Error("", slog.Any("err", err))
			} else {
				server.AddFlash(w, r, "success", tr.Gettext("User created."))
				server.Redirect(w, r, "./..", u.UID)
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
	server.RenderTemplate(w, r, 200, "/admin/user_create", ctx)
}

func (h *adminViews) userInfo(w http.ResponseWriter, r *http.Request) {
	tr := server.Locale(r)
	u := r.Context().Value(ctxUserKey{}).(*users.User)
	item := newUserItem(r, u, "./..")

	f := users.NewUserForm(server.Locale(r))
	f.SetUser(u)

	if r.Method == http.MethodPost {
		forms.Bind(f, r)

		if f.IsValid() {
			if _, err := f.UpdateUser(u); err != nil {
				server.Log(r).Error("", slog.Any("err", err))
			} else {
				// Refresh session if same user
				if auth.GetRequestUser(r).ID == u.ID {
					sess := server.GetSession(r)
					sess.Payload.User = u.ID
					sess.Payload.Seed = u.Seed
				}
				server.AddFlash(w, r, "success", tr.Gettext("User updated."))
				server.Redirect(w, r, u.UID)
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

	server.RenderTemplate(w, r, 200, "/admin/user", ctx)
}

func (h *adminViews) userDelete(w http.ResponseWriter, r *http.Request) {
	f := newDeleteForm(server.Locale(r))
	f.Get("_to").Set("/admin/users")
	forms.Bind(f, r)

	u := r.Context().Value(ctxUserKey{}).(*users.User)
	if u.ID == auth.GetRequestUser(r).ID {
		server.Err(w, r, errSameUser)
		return
	}

	if err := f.trigger(u); err != nil {
		server.Err(w, r, err)
		return
	}
	server.Redirect(w, r, f.Get("_to").String())
}
