// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package users

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/forms"
	"codeberg.org/readeck/readeck/pkg/glob"
)

type (
	ctxUserFormKey struct{}
)

// Error definitions.
var (
	ErrInvalidUsername  = forms.Gettext(`username is not valid`)
	ErrBlockedUsername  = forms.Gettext("username is not available")
	ErrBlockedEmailAddr = forms.ErrInvalidEmail
)

// IsValidPassword is the password validation rule.
var IsValidPassword = forms.TypedValidator(func(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	return len(v) >= 8
}, errors.New("password must be at least 8 character long"))

// IsValidUsername is the username validator.
// A valid username contains at least 3 characters from [a-z0-9_-]
// and start with a letter.
var IsValidUsername = forms.ValueValidatorFunc[string](func(f forms.Field, v string) error {
	if f.IsNil() {
		return nil
	}

	if len(v) < 3 {
		return ErrInvalidUsername
	}

	for _, x := range v {
		if unicode.Is(unicode.C, x) || unicode.Is(unicode.Space, x) {
			return ErrInvalidUsername
		}
	}

	for _, blocked := range configs.Config.Accounts.UsernameDenyList {
		if glob.Glob(blocked, v) {
			return ErrBlockedUsername
		}
	}

	return nil
})

// IsValidUserEmail is the user's email address validator.
var IsValidUserEmail = forms.ValueValidatorFunc[string](func(f forms.Field, v string) error {
	if err := forms.IsEmail.ValidateValue(f, v); err != nil {
		return err
	}

	for _, blocked := range configs.Config.Accounts.EmailDenyList {
		if glob.Glob(blocked, v) {
			return ErrBlockedEmailAddr
		}
	}

	return nil
})

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
	availableGroups = append(availableGroups, GroupList(tr, "@group", nil)...)

	return &UserForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("username",
			forms.Trim,
			hasUser().
				True(forms.RequiredOrNil).
				False(forms.Required),
			forms.MaxLen(128),
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
			forms.MaxLen(128),
			IsValidUserEmail,
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

	// A username can be an email address only if both match
	// TODO: when forms/v2 lands, make this part of a shared BaseUserForm
	// used by all user forms (admin, profile, onboarding)
	username := f.Get("username").String()
	email := f.Get("email").String()
	if strings.ContainsRune(username, '@') && username != email {
		f.AddErrors("username", ErrInvalidUsername)
		return
	}

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
func (f *UserForm) UpdateUser(u *User) (res map[string]any, err error) {
	if !f.IsBound() {
		err = errors.New("form is not bound")
		return
	}

	res = make(map[string]any)
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
		res["updated"] = time.Now().UTC()
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

// ProvisioningForm is the form used for retrieving an existing or new user
// based on its username and email address.
type ProvisioningForm struct {
	*forms.Form
}

// NewProvisioningForm returns a new [NewProvisioningForm].
func NewProvisioningForm(tr forms.Translator) *ProvisioningForm {
	availableGroups := [][2]string{
		{"none", tr.Pgettext("role", "no group")},
	}
	availableGroups = append(availableGroups, GroupList(tr, "@group", nil)...)

	return &ProvisioningForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("username", forms.MaxLen(128), IsValidUsername),
		forms.NewTextField("email", forms.MaxLen(128), IsValidUserEmail),
		forms.NewTextField("group", forms.RequiredOrNil, forms.ChoicesPairs(availableGroups)),
	)}
}

// Validate performs extra form validations.
func (f *ProvisioningForm) Validate() {
	// A username can be an email address only if both match
	username := f.Get("username").String()
	email := f.Get("email").String()
	if strings.ContainsRune(username, '@') && username != email {
		f.AddErrors("username", ErrInvalidUsername)
		return
	}
}

// LoadUser loads a user based on its username or email.
// When it exists, there must be only one result for the tupple username + email.
// If the user needs an update, a non empty [goqu.Record] is returned so any process
// calling this method can perform the update.
// When the user doesn't exist, the returned [User] has an ID 0 and can be immediately
// created with [Users.Create]. It already contains a generated password.
func (f *ProvisioningForm) LoadUser(username, email, group string) (*User, goqu.Record, error) {
	values := url.Values{"username": {username}, "email": {email}}
	if group != "" {
		values.Set("group", group)
	}

	forms.BindValues(f, values)

	if !f.IsValid() {
		if len(f.Errors()) > 0 {
			return nil, nil, f.Errors()
		}
		for _, field := range f.Fields() {
			if len(field.Errors()) > 0 {
				return nil, nil, forms.Errors{fmt.Errorf("%s: %s", field.Name(), field.Errors())}
			}
		}
	}

	res := []*User{}
	err := Users.Query().Where(
		goqu.Or(
			goqu.C("username").Eq(username),
			goqu.C("email").Eq(email),
		),
	).ScanStructs(&res)
	if err != nil {
		return nil, nil, err
	}

	if len(res) > 1 {
		return nil, nil, fmt.Errorf("more than one user is associated with %s and %s", username, email)
	}

	user := new(User)
	rec := goqu.Record{}
	if len(res) == 0 {
		if group == "" {
			group = "user"
		}
		user.Username = username
		user.Email = email
		user.Group = group
		user.Password = MakePassword(64)
	} else {
		user = res[0]
		if user.Username != username {
			rec["username"] = username
		}
		if user.Email != email {
			rec["email"] = email
		}
		if group != "" && user.Group != group {
			rec["group"] = group
		}
	}

	return user, rec, nil
}
