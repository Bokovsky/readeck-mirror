// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package users_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type fieldValidatorTest struct {
	f      forms.Field
	data   string
	errors []error
}

func (test fieldValidatorTest) assert(assert *require.Assertions) {
	_ = test.f.UnmarshalValues([]string{test.data})
	valid := test.f.IsValid()
	if len(test.errors) > 0 {
		assert.False(valid)
		assert.Len(test.f.Errors(), len(test.errors))
		for i, e := range test.f.Errors() {
			assert.EqualError(e, test.errors[i].Error())
		}
	} else {
		assert.True(valid)
	}
}

func runValidatorTests(tests []fieldValidatorTest) func(t *testing.T) {
	return func(t *testing.T) {
		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				test.assert(require.New(t))
			})
		}
	}
}

func TestValidators(t *testing.T) {
	configs.Config.Accounts.UsernameDenyList = []string{
		"admin*",
		"root",
		"*test*",
	}
	configs.Config.Accounts.EmailDenyList = []string{
		"test@*",
		"*@example.net",
		"*@localhost",
		"*@*.localhost",
	}
	defer func() {
		configs.Config.Accounts.UsernameDenyList = []string{}
	}()

	t.Run("username", runValidatorTests([]fieldValidatorTest{
		{
			f:    forms.NewTextField("", users.IsValidUsername),
			data: "\uff00",
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "a",
			errors: []error{users.ErrInvalidUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "ab",
			errors: []error{users.ErrInvalidUsername},
		},
		{
			f:    forms.NewTextField("", users.IsValidUsername),
			data: "abc",
		},
		{
			f:    forms.NewTextField("", users.IsValidUsername),
			data: "alice@example.org",
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "al ice",
			errors: []error{users.ErrInvalidUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "al\nice",
			errors: []error{users.ErrInvalidUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "al\u3000ice",
			errors: []error{users.ErrInvalidUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "Al\u200Bice",
			errors: []error{users.ErrInvalidUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "Al\x1dice",
			errors: []error{users.ErrInvalidUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "Al\u00ADice",
			errors: []error{users.ErrInvalidUsername},
		},
		{
			f:    forms.NewTextField("", users.IsValidUsername),
			data: "ålice",
		},
		{
			f:    forms.NewTextField("", users.IsValidUsername),
			data: "alice",
		},
		{
			f:    forms.NewTextField("", users.IsValidUsername),
			data: "1alice",
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "admin",
			errors: []error{users.ErrBlockedUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "administrator",
			errors: []error{users.ErrBlockedUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "root",
			errors: []error{users.ErrBlockedUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "abtest",
			errors: []error{users.ErrBlockedUsername},
		},
		{
			f:      forms.NewTextField("", users.IsValidUsername),
			data:   "test-ab",
			errors: []error{users.ErrBlockedUsername},
		},
	}))

	t.Run("email", runValidatorTests([]fieldValidatorTest{
		{
			f:    forms.NewTextField("", users.IsValidUserEmail),
			data: "\uff00",
		},
		{
			f:    forms.NewTextField("", users.IsValidUserEmail),
			data: "alice@example.com",
		},
		{
			f:      forms.NewTextField("", users.IsValidUserEmail),
			data:   "alice@example.net",
			errors: []error{forms.ErrInvalidEmail},
		},
		{
			f:      forms.NewTextField("", users.IsValidUserEmail),
			data:   "alice@localhost",
			errors: []error{forms.ErrInvalidEmail},
		},
		{
			f:      forms.NewTextField("", users.IsValidUserEmail),
			data:   "alice@host.localhost",
			errors: []error{forms.ErrInvalidEmail},
		},
		{
			f:      forms.NewTextField("", users.IsValidUserEmail),
			data:   "alice@example.net",
			errors: []error{forms.ErrInvalidEmail},
		},
	}))
}
