// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package forms_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/forms"
)

type fieldValidatorTest struct {
	f      forms.Field
	data   string
	expect any
	errors []error
}

func (test fieldValidatorTest) assert(assert *require.Assertions) {
	_ = test.f.UnmarshalJSON([]byte(test.data))
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
	assert.Equal(test.expect, test.f.Value())
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
	t.Run("required", runValidatorTests([]fieldValidatorTest{
		{
			f:      forms.NewTextField("", forms.Required),
			data:   `null`,
			expect: nil,
			errors: []error{forms.ErrRequired},
		},
		{
			f:      forms.NewTextField("", forms.Required),
			data:   `""`,
			expect: "",
			errors: []error{forms.ErrRequired},
		},
		{
			f:      forms.NewTextField("", forms.Trim, forms.Required),
			data:   `"  \t   \r\n"`,
			expect: "",
			errors: []error{forms.ErrRequired},
		},
		{
			f:      forms.NewTextField("", forms.Required),
			data:   `"abc"`,
			expect: "abc",
		},
		{
			f:      forms.NewIntegerField("", forms.Required, forms.Gte(10)),
			data:   `null`,
			expect: nil,
			errors: []error{forms.ErrRequired},
		},
	}))

	t.Run("required on list", runValidatorTests([]fieldValidatorTest{
		{
			f:      forms.NewTextListField("", forms.Required),
			data:   `null`,
			expect: nil,
			errors: []error{forms.ErrRequired},
		},
		{
			f:      forms.NewTextListField("", forms.Required),
			data:   `[]`,
			expect: []string(nil),
			errors: []error{forms.ErrRequired},
		},
		{
			f:      forms.NewTextListField("", forms.Required),
			data:   `[""]`,
			expect: []string{""},
		},
		{
			f:      forms.NewTextListField("", forms.Trim, forms.Required),
			data:   `["   ", "\t  \n"]`,
			expect: []string{"", ""},
		},
	}))

	t.Run("required or null", runValidatorTests([]fieldValidatorTest{
		{
			f:      forms.NewTextField("", forms.RequiredOrNil),
			data:   `""`,
			expect: "",
			errors: []error{forms.ErrRequired},
		},
		{
			f:      forms.NewTextField("", forms.Trim, forms.RequiredOrNil),
			data:   `"  \t   \r\n"`,
			expect: "",
			errors: []error{forms.ErrRequired},
		},
		{
			f:      forms.NewBooleanField("", forms.RequiredOrNil),
			data:   `null`,
			expect: nil,
		},
		{
			f:      forms.NewBooleanField("", forms.RequiredOrNil),
			data:   `false`,
			expect: false,
		},
	}))

	t.Run("skip", runValidatorTests([]fieldValidatorTest{
		{
			f:      forms.NewTextField("", forms.Skip, forms.IsEmail),
			data:   `null`,
			expect: nil,
		},
		{
			f:      forms.NewTextField("", forms.Skip, forms.IsEmail),
			data:   `""`,
			expect: "",
		},
		{
			f:      forms.NewTextField("", forms.Skip, forms.IsEmail),
			data:   `"alice@example.net"`,
			expect: "alice@example.net",
		},
		{
			f:      forms.NewTextField("", forms.Skip, forms.IsEmail),
			data:   `"alice"`,
			expect: "alice",
			errors: []error{forms.ErrInvalidEmail},
		},
	}))

	t.Run("when", runValidatorTests([]fieldValidatorTest{
		{
			f: forms.NewTextField("", forms.When(func(_ forms.Field, _ string) bool {
				return true
			}).True(forms.Required)),
			data:   `null`,
			expect: nil,
			errors: []error{forms.ErrRequired},
		},
		{
			f: forms.NewTextField("", forms.When(func(_ forms.Field, _ string) bool {
				return false
			}).True(forms.Required)),
			data:   `null`,
			expect: nil,
		},
		{
			f: forms.NewTextField("",
				forms.When(func(_ forms.Field, _ string) bool {
					return true
				}).
					True(forms.Required),
				forms.IsEmail,
			),
			data:   `""`,
			expect: "",
			errors: []error{forms.ErrRequired},
		},
		{
			f: forms.NewTextField("",
				forms.When(func(_ forms.Field, _ string) bool {
					return false
				}).
					True(forms.Required),
				forms.IsEmail,
			),
			data:   `""`,
			expect: "",
			errors: []error{forms.ErrInvalidEmail},
		},
	}))

	t.Run("optional", runValidatorTests([]fieldValidatorTest{
		{
			f:      forms.NewTextField("", forms.Optional[string](forms.IsEmail)),
			data:   `null`,
			expect: nil,
		},
		{
			f:      forms.NewTextField("", forms.Optional[string](forms.IsEmail)),
			data:   `""`,
			expect: "",
		},
		{
			f:      forms.NewTextField("", forms.Optional[string](forms.IsEmail)),
			data:   `"alice@example.net"`,
			expect: "alice@example.net",
		},
		{
			f:      forms.NewTextField("", forms.Optional[string](forms.IsEmail)),
			data:   `"alice"`,
			expect: "alice",
			errors: []error{forms.ErrInvalidEmail},
		},
	}))

	t.Run("email", runValidatorTests([]fieldValidatorTest{
		{
			f:      forms.NewTextField("", forms.IsEmail),
			data:   `"test@example.org"`,
			expect: "test@example.org",
		},
		{
			f:      forms.NewTextField("", forms.IsEmail),
			data:   `"test@example@.org"`,
			expect: "test@example@.org",
			errors: []error{forms.ErrInvalidEmail},
		},
		{
			f:      forms.NewTextField("", forms.IsEmail),
			data:   `"@test"`,
			expect: "@test",
			errors: []error{forms.ErrInvalidEmail},
		},
		{
			f:      forms.NewTextField("", forms.IsEmail),
			data:   `"test@"`,
			expect: "test@",
			errors: []error{forms.ErrInvalidEmail},
		},
		{
			f:      forms.NewTextField("", forms.IsEmail),
			data:   `"foo"`,
			expect: "foo",
			errors: []error{forms.ErrInvalidEmail},
		},
		{
			f:      forms.NewTextField("", forms.IsEmail),
			data:   `""`,
			expect: "",
			errors: []error{forms.ErrInvalidEmail},
		},
		{
			f:      forms.NewTextField("", forms.IsEmail),
			data:   `null`,
			expect: nil,
		},
		{
			f:      forms.NewTextListField("", forms.IsEmail),
			data:   `["test@example.net"]`,
			expect: []string{"test@example.net"},
		},
		{
			f:      forms.NewTextListField("", forms.IsEmail),
			data:   `["test@example.net", "foo"]`,
			expect: []string{"test@example.net", "foo"},
			errors: []error{forms.ErrInvalidEmail},
		},
	}))

	t.Run("URL", runValidatorTests([]fieldValidatorTest{
		{
			f:      forms.NewTextField("", forms.IsURL("http")),
			data:   `"http://example.net/"`,
			expect: "http://example.net/",
		},
		{
			f:      forms.NewTextField("", forms.IsURL("http", "https")),
			data:   `"https://example.net/"`,
			expect: "https://example.net/",
		},
		{
			f:      forms.NewTextField("", forms.IsURL("http")),
			data:   `"http://` + string(rune(0x7f)) + `example.net/"`,
			expect: "http://" + string(rune(0x7f)) + "example.net/",
			errors: []error{forms.ErrInvalidURL},
		},
		{
			f:      forms.NewTextField("", forms.IsURL()),
			data:   `"http://example.net/"`,
			expect: "http://example.net/",
			errors: []error{forms.ErrInvalidURL},
		},
		{
			f:      forms.NewTextField("", forms.IsURL("http")),
			data:   `"http://"`,
			expect: "http://",
			errors: []error{forms.ErrInvalidURL},
		},
		{
			f:      forms.NewTextField("", forms.IsURL("http", "https")),
			data:   `"ftp://example.net/"`,
			expect: "ftp://example.net/",
			errors: []error{forms.ErrInvalidURL},
		},
	}))

	t.Run("gte & lte", runValidatorTests([]fieldValidatorTest{
		{
			// Gte / Lte don't apply on non integer field
			f:      forms.NewTextField("test", forms.Gte(10)),
			data:   `"5"`,
			expect: "5",
		},
		{
			f:      forms.NewIntegerField("test", forms.Gte(10)),
			data:   "10",
			expect: 10,
		},
		{
			f:      forms.NewIntegerField("test", forms.Gte(10)),
			data:   "15",
			expect: 15,
		},
		{
			f:      forms.NewIntegerField("test", forms.Gte(10)),
			data:   "2",
			expect: 2,
			errors: []error{errors.New("must be greater or equal than 10")},
		},
		{
			f:      forms.NewIntegerField("test", forms.Lte(10)),
			data:   "10",
			expect: 10,
		},
		{
			f:      forms.NewIntegerField("test", forms.Lte(10)),
			data:   "15",
			expect: 15,
			errors: []error{errors.New("must be lower or equal than 10")},
		},
		{
			f:      forms.NewIntegerField("test", forms.Lte(10)),
			data:   "2",
			expect: 2,
		},
	}))

	t.Run("strLen", runValidatorTests([]fieldValidatorTest{
		{
			f:      forms.NewTextField("test", forms.StrLen(2, 10)),
			data:   `"abc"`,
			expect: "abc",
		},
		{
			f:      forms.NewTextField("test", forms.StrLen(2, 10)),
			data:   `"問掃玉光尤向入神間示"`,
			expect: "問掃玉光尤向入神間示",
		},
		{
			f:      forms.NewTextField("test", forms.StrLen(2, 10)),
			data:   `"a"`,
			expect: "a",
			errors: []error{errors.New("text must contain between 2 and 10 characters")},
		},
		{
			f:      forms.NewTextField("test", forms.StrLen(2, 10)),
			data:   `"abcdefghijk"`,
			expect: "abcdefghijk",
			errors: []error{errors.New("text must contain between 2 and 10 characters")},
		},
	}))

	t.Run("default", func(t *testing.T) {
		tests := []struct {
			f forms.Field
			v any
		}{
			{
				forms.NewTextField("", forms.Default("abc")),
				"abc",
			},
			{
				forms.NewIntegerField("", forms.Default("abc")),
				0,
			},
			{
				forms.NewBooleanField("", forms.Default(true)),
				true,
			},
			{
				forms.NewTextListField("", forms.Default([]string{"abc"})),
				[]string{"abc"},
			},
			{
				forms.NewTextField("", forms.DefaultFunc(func(_ forms.Field) any {
					return "abc"
				})),
				"abc",
			},
			{
				forms.NewTextField("", forms.DefaultFunc(func(_ forms.Field) any {
					return 12
				})),
				"",
			},
		}

		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				require.Exactly(t, test.v, test.f.Value())
			})
		}
	})

	t.Run("choices", runValidatorTests([]fieldValidatorTest{
		{
			f: forms.NewTextField("", forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `"a"`,
			expect: "a",
		},
		{
			f: forms.NewTextField("", forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `null`,
			expect: nil,
		},
		{
			f: forms.NewTextField("", forms.Required, forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `null`,
			expect: nil,
			errors: []error{forms.ErrRequired},
		},
		{
			f: forms.NewTextField("", forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `"x"`,
			expect: "x",
			errors: []error{errors.New("x is not one of a, b")},
		},
		{
			f: forms.NewIntegerField("", forms.Choices(
				forms.Choice("A", 1), forms.Choice("B", 2),
			)),
			data:   `2`,
			expect: 2,
		},
		{
			// It won't apply at all.
			f: forms.NewIntegerField("", forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `5`,
			expect: 5,
		},
		{
			f: forms.NewTextField("", forms.ChoicesPairs([][2]string{
				{"a", "A"},
				{"b", "B"},
			})),
			data:   `"a"`,
			expect: "a",
		},
		{
			f: forms.NewTextField("", forms.ChoicesPairs([][2]string{
				{"a", "A"},
				{"b", "B"},
			})),
			data:   `"x"`,
			expect: "x",
			errors: []error{errors.New("x is not one of a, b")},
		},
	}))

	t.Run("choices on lists", runValidatorTests([]fieldValidatorTest{
		{
			f: forms.NewTextListField("", forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `["a"]`,
			expect: []string{"a"},
		},
		{
			f: forms.NewTextListField("", forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `null`,
			expect: nil,
		},
		{
			f: forms.NewTextListField("", forms.Required, forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `null`,
			expect: nil,
			errors: []error{forms.ErrRequired},
		},
		{
			f: forms.NewTextListField("", forms.Required, forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `[]`,
			expect: []string(nil),
			errors: []error{forms.ErrRequired},
		},
		{
			f: forms.NewTextListField("", forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `["a", "x"]`,
			expect: []string{"a", "x"},
			errors: []error{errors.New("x is not one of a, b")},
		},
		{
			f: forms.NewTextListField("", forms.Choices(
				forms.Choice("A", "a"), forms.Choice("B", "b"),
			)),
			data:   `["a", "x", "z"]`,
			expect: []string{"a", "x", "z"},
			errors: []error{
				errors.New("x is not one of a, b"),
				errors.New("z is not one of a, b"),
			},
		},
		{
			f: forms.NewIntegerListField("", forms.Choices(
				forms.Choice("A", 1), forms.Choice("B", 2),
			)),
			data:   `[2]`,
			expect: []int{2},
		},
		{
			// It won't apply at all.
			f: forms.NewIntegerListField("", forms.Choices(
				forms.Choice("A", "1"), forms.Choice("B", "2"),
			)),
			data:   `[5, 10]`,
			expect: []int{5, 10},
		},
	}))

	t.Run("choices func", func(t *testing.T) {
		assert := require.New(t)
		cs := forms.ValueChoices[string]{
			{"A", "a"}, {"B", "b"},
		}
		ci := forms.ValueChoices[int]{
			{"A", 1}, {"B", 2},
		}

		form := forms.Must(
			context.Background(),
			forms.NewTextField("s", forms.Choices(cs...)),
			forms.NewIntegerField("i", forms.Choices(ci...)),
		)
		assert.Exactly(cs, form.Get("s").(*forms.TextField).Choices())
		assert.Exactly(ci, form.Get("i").(*forms.IntegerField).Choices())

		assert.True(ci[0].In([]int{1, 2}))
		assert.False(ci[0].In([]int{2}))
	})
}
