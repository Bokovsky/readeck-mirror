// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package signin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bus"
	"codeberg.org/readeck/readeck/internal/email"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type recoverForm struct {
	*forms.Form
	ttl    time.Duration
	prefix string
}

func newRecoverForm(tr forms.Translator) *recoverForm {
	return &recoverForm{
		Form: forms.Must(
			forms.WithTranslator(context.Background(), tr),
			forms.NewIntegerField("step",
				forms.Required,
				forms.TypedValidator(func(v int) bool {
					return 0 <= v || v <= 3
				}, errors.New("invalid step")),
			),
			forms.NewTextField("email",
				forms.Trim,
				forms.When(func(f forms.Field, _ string) bool {
					step := forms.GetForm(f).Get("step").(forms.TypedField[int]).V()
					return step == 0 || step == 1
				}).
					True(forms.Required),
			),
			forms.NewTextField("password",
				forms.When(func(f forms.Field, _ string) bool {
					step := forms.GetForm(f).Get("step").(forms.TypedField[int]).V()
					return step == 2 || step == 3
				}).
					True(forms.Required, users.IsValidPassword),
			),
		),
		ttl:    time.Duration(2 * time.Hour),
		prefix: "recover_code",
	}
}

func (f *recoverForm) saveCode(code string, userID int) error {
	return bus.Store().Set(
		f.prefix+"_"+code,
		strconv.Itoa(userID),
		f.ttl,
	)
}

func (f *recoverForm) getCode(code string) (int, bool) {
	v := bus.Store().Get(fmt.Sprintf("%s_%s", f.prefix, code))
	if v == "" {
		return 0, false
	}
	userID, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return userID, true
}

func (f *recoverForm) delCode(code string) error {
	return bus.Store().Del(fmt.Sprintf("%s_%s", f.prefix, code))
}

func (h *authHandler) recover(w http.ResponseWriter, r *http.Request) {
	f := newRecoverForm(server.Locale(r))
	f.Get("step").Set(0)

	tc := server.TC{
		"Form": f,
	}

	recoverCode := chi.URLParam(r, "code")

	step0 := func() {
		if !f.IsValid() {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		user, err := users.Users.GetOne(goqu.C("email").Eq(f.Get("email").String()))

		defer func() {
			if err != nil {
				server.Log(r).Error("recover step 0", slog.Any("err", err))
				f.AddErrors("", forms.ErrUnexpected)
			}
		}()

		if err != nil && !errors.Is(err, users.ErrNotFound) {
			return
		}

		mailTc := server.TC{
			"SiteURL":   urls.AbsoluteURL(r, "/"),
			"EmailAddr": f.Get("email").String(),
		}
		code := base58.NewUUID()
		if user != nil {
			if err = f.saveCode(code, user.ID); err != nil {
				return
			}

			mailTc["RecoverLink"] = urls.AbsoluteURL(r, "/login/recover", code)
		}

		msg, err := email.NewMsg(
			configs.Config.Email.FromNoReply.String(),
			f.Get("email").String(),
			"[Readeck] Password Recovery",
			email.WithMDTemplate(
				"/emails/recover.jet.md",
				server.TemplateVars(r),
				mailTc,
			),
		)
		if err != nil {
			server.Err(w, r, err)
			return
		}

		if err = email.Sender.SendEmail(msg); err != nil {
			server.Err(w, r, err)
			return
		}

		f.Get("step").Set(1)
	}

	step2 := func() {
		var err error
		var user *users.User

		userID, ok := f.getCode(recoverCode)
		if !ok {
			tc["Error"] = "Invalid recovery code"
			return
		}
		user, err = users.Users.GetOne(goqu.C("id").Eq(userID))
		if err != nil {
			tc["Error"] = "Invalid recovery code"
			server.Log(r).Error("get user", slog.Any("err", err))
			return
		}

		if r.Method == http.MethodGet {
			return
		}

		if !f.IsValid() {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		defer func() {
			if err != nil {
				server.Log(r).Error("password update", slog.Any("err", err))
				f.AddErrors("", forms.ErrUnexpected)
			}
		}()

		if err = user.SetPassword(f.Get("password").String()); err != nil {
			return
		}
		user.SetSeed()
		if err = user.Save(); err != nil {
			return
		}

		if err = f.delCode(recoverCode); err != nil {
			return
		}
		f.Get("step").Set(3)
	}

	switch r.Method {
	case http.MethodGet:
		if recoverCode != "" {
			f.Get("step").Set(2)
			step2()
		}
	case http.MethodPost:
		forms.Bind(f, r)
		switch f.Get("step").Value() {
		case 0:
			step0()
		case 1:
			// Step 1 is a template only step
			if recoverCode == "" || !f.IsValid() {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		case 2:
			step2()
		case 3:
			// Step 3 is a template only step
			if recoverCode == "" || !f.IsValid() {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}

	}

	server.RenderTemplate(w, r, http.StatusOK, "/auth/recover", tc)
}
