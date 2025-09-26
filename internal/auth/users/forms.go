// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package users

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/pkg/forms"
)

type (
	ctxUserFormKey struct{}
)

// rxUsername is the regexp used to validate a username.
var rxUsername = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// IsValidPassword is the password validation rule.
var IsValidPassword = forms.TypedValidator(func(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	return len(v) >= 8
}, errors.New("password must be at least 8 character long"))

// IsValidUsername is the username validation rule.
var IsValidUsername = forms.TypedValidator(func(v string) bool {
	return rxUsername.MatchString(v)
}, errors.New(`must contain English letters, digits, "_" and "-" only`))

// UserForm is the form used for user creation and update.
type UserForm struct {
	*forms.Form
}

// NewUserForm returns a UserForm instance.
func NewUserForm(tr forms.Translator) *UserForm {
	hasUser := func() *forms.ConditionValidator[string] {
		return forms.When(func(f forms.Field, _ string) bool {
			u, _ := forms.GetForm(f).Context().Value(ctxUserFormKey{}).(*User)
			return u != nil
		})
	}

	availableGroups := [][2]string{
		{"none", tr.Pgettext("role", "no group")},
	}
	availableGroups = append(availableGroups, GroupList(tr, "__group__", nil)...)

	return &UserForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("username",
			forms.Trim,
			hasUser().
				True(forms.RequiredOrNil).
				False(forms.Required),
			IsValidUsername,
		),
		forms.NewTextField("password",
			hasUser().
				False(forms.Required),
			forms.ValueValidatorFunc[string](func(f forms.Field, v string) error {
				if f.IsBound() && v != "" && strings.TrimSpace(v) == "" {
					return forms.Gettext("password is empty")
				}
				return nil
			}),
		),
		forms.NewTextField("email",
			forms.Trim,
			hasUser().
				True(forms.RequiredOrNil).
				False(forms.Required),
			forms.IsEmail,
		),
		forms.NewTextField("group",
			forms.Trim,
			forms.Default("user"),
			forms.ChoicesPairs(availableGroups),
			hasUser().False(forms.Required),
		),
	)}
}

// SetUser adds a user to the form's context.
func (f *UserForm) SetUser(u *User) {
	ctx := context.WithValue(f.Context(), ctxUserFormKey{}, u)
	f.SetContext(ctx)

	f.Get("username").Set(u.Username)
	f.Get("email").Set(u.Email)
	f.Get("group").Set(u.Group)
}

// Bind prepares the form before data binding.
// It changes some validators in case of user update.
func (f *UserForm) Bind() {
	f.Form.Bind()

	u, _ := f.Context().Value(ctxUserFormKey{}).(*User)
	if u == nil {
		// set default group
		f.Get("group").Set("user")
		return
	}
}

// Validate performs extra form validation.
func (f *UserForm) Validate() {
	u, _ := f.Context().Value(ctxUserFormKey{}).(*User)

	userQuery := Users.Query().
		Where(goqu.C("username").Eq(f.Get("username").String()))
	emailQuery := Users.Query().
		Where(goqu.C("email").Eq(f.Get("email").String()))

	if u != nil {
		userQuery = userQuery.Where(goqu.C("id").Neq(u.ID))
		emailQuery = emailQuery.Where(goqu.C("id").Neq(u.ID))
	}

	// Check that username is not already in use
	if c, err := userQuery.Count(); err != nil {
		f.AddErrors("", errors.New("validation process error"))
	} else if c > 0 {
		f.AddErrors("username", errors.New("username is already in use"))
	}

	// Check that email is not already in use
	if c, err := emailQuery.Count(); err != nil {
		f.AddErrors("", errors.New("validation process error"))
	} else if c > 0 {
		f.AddErrors("email", errors.New("email address is already in use"))
	}
}

// CreateUser performs the user creation.
func (f *UserForm) CreateUser() (*User, error) {
	u := &User{
		Username: f.Get("username").String(),
		Email:    f.Get("email").String(),
		Password: f.Get("password").String(),
		Group:    f.Get("group").String(),
	}

	err := Users.Create(u)
	if err != nil {
		f.AddErrors("", forms.ErrUnexpected)
	}

	return u, err
}

// UpdateUser performs a user update and returns a mapping of
// updated fields.
func (f *UserForm) UpdateUser(u *User) (res map[string]interface{}, err error) {
	if !f.IsBound() {
		err = errors.New("form is not bound")
		return
	}

	res = make(map[string]interface{})
	for _, field := range f.Fields() {
		switch field.Name() {
		case "password":
			if field.IsNil() || strings.TrimSpace(field.String()) == "" {
				continue
			}
			p, err := u.HashPassword(field.String())
			if err != nil {
				f.AddErrors("", forms.ErrUnexpected)
				return nil, err
			}
			res[field.Name()] = p
		default:
			if field.IsBound() && !field.IsNil() {
				res[field.Name()] = field.Value()
			}
		}
	}

	if len(res) > 0 {
		res["updated"] = time.Now()
		res["seed"] = u.SetSeed()
		if err = u.Update(res); err != nil {
			f.AddErrors("", forms.ErrUnexpected)
			return
		}
		if _, ok := res["password"]; ok {
			res["password"] = "-"
		}
	}
	res["id"] = u.ID
	delete(res, "seed")
	return
}
