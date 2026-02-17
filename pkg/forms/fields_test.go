// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package forms_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/forms"
)

type fieldTest[D any, T any] struct {
	data  D
	value any
	v     T
	str   string
	empty bool
	err   error
}

func (test fieldTest[D, T]) assert(assert *require.Assertions, field forms.Field, err error) {
	if test.err == nil {
		assert.True(field.IsBound(), "field is bound")
		assert.NoError(err, "no error")
		assert.Empty(field.Errors(), "no field error")
	} else {
		assert.False(field.IsBound(), "field is not bound")
		assert.ErrorIs(err, test.err, "decoding error")
		assert.Len(field.Errors(), 1, "only one error")
		assert.ErrorIs(field.Errors()[0], test.err, "field error")
	}

	assert.Equal(test.empty, field.IsEmpty(), "empty")
	assert.Exactly(test.value, field.Value(), "value")
	assert.Exactly(test.v, field.(forms.TypedField[T]).V(), "typed value")
	assert.Exactly(test.str, field.String(), "string")
}

func runSetField[T any](constructor func() forms.Field, tests []fieldTest[any, T]) func(t *testing.T) {
	return func(t *testing.T) {
		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				field := constructor()
				field.Set(test.data)
				assert := require.New(t)
				assert.False(field.IsBound())
				assert.Equal(test.empty, field.IsEmpty(), "empty")
				assert.Exactly(test.value, field.Value(), "value")
				assert.Exactly(test.v, field.(forms.TypedField[T]).V(), "typed value")
				assert.Exactly(test.str, field.String(), "string")
			})
		}
	}
}

func runJSONField[T any](constructor func() forms.Field, tests []fieldTest[string, T]) func(t *testing.T) {
	return func(t *testing.T) {
		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				field := constructor()
				err := field.UnmarshalJSON([]byte(test.data))
				test.assert(require.New(t), field, err)
			})
		}
	}
}

func runValuesField[T any](constructor func() forms.Field, tests []fieldTest[[]string, T]) func(t *testing.T) {
	return func(t *testing.T) {
		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				field := constructor()
				err := field.UnmarshalValues(test.data)
				test.assert(require.New(t), field, err)
			})
		}
	}
}

func TestTextField(t *testing.T) {
	t.Run("set", runSetField(func() forms.Field {
		return forms.NewTextField("")
	}, []fieldTest[any, string]{
		{
			data:  nil,
			value: nil,
			v:     "",
			str:   "",
			empty: true,
		},
		{
			data:  "",
			value: "",
			v:     "",
			str:   "",
			empty: true,
		},
		{
			data:  "value",
			value: "value",
			v:     "value",
			str:   "value",
		},
		{
			data:  new("pointer value"),
			value: "pointer value",
			v:     "pointer value",
			str:   "pointer value",
		},
	}))

	t.Run("json", runJSONField(func() forms.Field {
		return forms.NewTextField("")
	}, []fieldTest[string, string]{
		{
			data:  `""`,
			value: "",
			v:     "",
			str:   "",
			empty: true,
		},
		{
			data:  `"test"`,
			value: "test",
			v:     "test",
			str:   "test",
		},
		{
			data:  "null",
			value: nil,
			v:     "",
			str:   "",
			empty: true,
		},
		{
			data:  "//",
			value: nil,
			v:     "",
			str:   "",
			empty: true,
			err:   forms.ErrInvalidValue,
		},
		{
			data:  "1234",
			value: "",
			v:     "",
			str:   "",
			empty: true,
			err:   forms.ErrInvalidValue,
		},
	}))

	t.Run("values", runValuesField(func() forms.Field {
		return forms.NewTextField("")
	}, []fieldTest[[]string, string]{
		{
			data:  []string{""},
			value: "",
			v:     "",
			str:   "",
			empty: true,
		},
		{
			data:  []string{"\uff00"},
			value: nil,
			v:     "",
			str:   "",
			empty: true,
		},
		{
			data:  []string{"abc"},
			value: "abc",
			v:     "abc",
			str:   "abc",
		},
		{
			data:  []string{"foo"},
			v:     "foo",
			value: "foo",
			str:   "foo",
		},
		{
			data:  []string{"bar", "foo"},
			v:     "bar",
			value: "bar",
			str:   "bar",
		},
	}))

	t.Run("json trimmed", runJSONField(func() forms.Field {
		return forms.NewTextField("", forms.Trim)
	}, []fieldTest[string, string]{
		{
			data:  `"   \t\n "`,
			value: "",
			v:     "",
			str:   "",
			empty: true,
		},
		{
			data:  `"  \tabc \n\r  "`,
			value: "abc",
			v:     "abc",
			str:   "abc",
		},
		{
			data:  "null",
			value: nil,
			v:     "",
			str:   "",
			empty: true,
		},
	}))

	t.Run("values trimmed", runValuesField(func() forms.Field {
		return forms.NewTextField("", forms.Trim)
	}, []fieldTest[[]string, string]{
		{
			data:  []string{"\uff00"},
			value: nil,
			v:     "",
			str:   "",
			empty: true,
		},
		{
			data:  []string{"\t   \r\n "},
			value: "",
			v:     "",
			str:   "",
			empty: true,
		},
		{
			data:  []string{"  abc  \n\t "},
			value: "abc",
			v:     "abc",
			str:   "abc",
		},
	}))

	t.Run("json discard empty", runJSONField(func() forms.Field {
		return forms.NewTextField("", forms.DiscardEmpty)
	}, []fieldTest[string, string]{
		{
			data:  `""`,
			value: nil,
			v:     "",
			str:   "",
			empty: true,
		},
	}))

	t.Run("values discard empty", runValuesField(func() forms.Field {
		return forms.NewTextField("", forms.DiscardEmpty)
	}, []fieldTest[[]string, string]{
		{
			data:  []string{""},
			value: nil,
			v:     "",
			str:   "",
			empty: true,
		},
	}))
}

func TestBooleanField(t *testing.T) {
	t.Run("set", runSetField(func() forms.Field {
		return forms.NewBooleanField("")
	}, []fieldTest[any, bool]{
		{
			data:  true,
			value: true,
			v:     true,
			str:   "true",
		},
		{
			data:  new(false),
			value: false,
			v:     false,
			str:   "false",
		},
		{
			data:  nil,
			value: nil,
			v:     false,
			str:   "",
			empty: true,
		},
		{
			data:  "test",
			value: false,
			v:     false,
			str:   "",
			empty: true,
		},
	}))

	t.Run("json", runJSONField(func() forms.Field {
		return forms.NewBooleanField("")
	}, []fieldTest[string, bool]{
		{
			data:  "true",
			v:     true,
			value: true,
			str:   "true",
		},
		{
			data:  "false",
			v:     false,
			value: false,
			str:   "false",
		},
		{
			data:  "null",
			v:     false,
			value: nil,
			str:   "",
			empty: true,
		},
		{
			data:  "1234",
			v:     false,
			value: false,
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))

	t.Run("values", runValuesField(func() forms.Field {
		return forms.NewBooleanField("")
	}, []fieldTest[[]string, bool]{
		{
			data:  []string{""},
			v:     false,
			value: nil,
			str:   "",
			empty: true,
		},
		{
			data:  []string{"\uff00"},
			v:     false,
			value: nil,
			str:   "",
			empty: true,
		},
		{
			data:  []string{"on"},
			v:     true,
			value: true,
			str:   "true",
		},
		{
			data:  []string{"f"},
			v:     false,
			value: false,
			str:   "false",
		},
		{
			data:  []string{"t"},
			v:     true,
			value: true,
			str:   "true",
		},
		{
			data:  []string{"t", "f"},
			v:     true,
			value: true,
			str:   "true",
		},
		{
			data:  []string{"0"},
			v:     false,
			value: false,
			str:   "false",
		},
		{
			data:  []string{"1"},
			v:     true,
			value: true,
			str:   "true",
		},
		{
			data:  []string{"whatever"},
			v:     false,
			value: false,
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))
}

func TestIntegerField(t *testing.T) {
	t.Run("set", runSetField(func() forms.Field {
		return forms.NewIntegerField("")
	}, []fieldTest[any, int]{
		{
			data:  10,
			value: 10,
			v:     10,
			str:   "10",
		},
		{
			data:  new(-5),
			value: -5,
			v:     -5,
			str:   "-5",
		},
		{
			data:  nil,
			value: nil,
			v:     0,
			str:   "",
			empty: true,
		},
		{
			data:  "abc",
			value: 0,
			v:     0,
			str:   "",
			empty: true,
		},
	}))

	t.Run("json", runJSONField(func() forms.Field {
		return forms.NewIntegerField("")
	}, []fieldTest[string, int]{
		{
			data:  "null",
			v:     0,
			value: nil,
			str:   "",
			empty: true,
		},
		{
			data:  "10",
			v:     10,
			value: 10,
			str:   "10",
		},
		{
			data:  "-5",
			v:     -5,
			value: -5,
			str:   "-5",
		},
		{
			data:  `102.5`,
			v:     0,
			value: 0,
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
		{
			data:  `"abcd"`,
			v:     0,
			value: 0,
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))

	t.Run("values", runValuesField(func() forms.Field {
		return forms.NewIntegerField("")
	}, []fieldTest[[]string, int]{
		{
			data:  []string{"\uff00"},
			v:     0,
			value: nil,
			str:   "",
			empty: true,
		},
		{
			data:  []string{"10"},
			v:     10,
			value: 10,
			str:   "10",
		},
		{
			data:  []string{"-5"},
			v:     -5,
			value: -5,
			str:   "-5",
		},
		{
			data:  []string{"102.5"},
			v:     0,
			value: 0,
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
		{
			data:  []string{"whatever"},
			v:     0,
			value: 0,
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))

	t.Run("values trimmed", runValuesField(func() forms.Field {
		return forms.NewIntegerField("", forms.Trim)
	}, []fieldTest[[]string, int]{
		{
			data:  []string{"\uff00"},
			value: nil,
			v:     0,
			str:   "",
			empty: true,
		},
		{
			data:  []string{"\t   \r\n "},
			value: nil,
			v:     0,
			str:   "",
			empty: true,
		},
		{
			data:  []string{"  12  \n\t "},
			value: 12,
			v:     12,
			str:   "12",
		},
	}))
}

func TestDatetimeField(t *testing.T) {
	d1, _ := time.Parse(time.DateOnly, "2020-01-30")
	d2, _ := time.Parse(time.DateTime, "2020-01-30 14:24:06")

	t.Run("set", runSetField(func() forms.Field {
		return forms.NewDatetimeField("")
	}, []fieldTest[any, time.Time]{
		{
			data:  nil,
			value: nil,
			v:     time.Time{},
			str:   "",
			empty: true,
		},
		{
			data:  time.Time{},
			value: time.Time{},
			v:     time.Time{},
			str:   "",
			empty: true,
		},
		{
			data:  d1,
			value: d1,
			v:     d1,
			str:   "2020-01-30",
		},
		{
			data:  &d1,
			value: d1,
			v:     d1,
			str:   "2020-01-30",
		},
		{
			data:  "abcd",
			value: time.Time{},
			v:     time.Time{},
			str:   "",
			empty: true,
		},
	}))

	t.Run("json", runJSONField(func() forms.Field {
		return forms.NewDatetimeField("")
	}, []fieldTest[string, time.Time]{
		{
			data:  `""`,
			v:     time.Time{},
			value: nil,
			str:   "",
			empty: true,
		},
		{
			data:  `"2020-01-30"`,
			v:     d1,
			value: d1,
			str:   "2020-01-30",
		},
		{
			data:  `"2020-01-30 14:24:06"`,
			v:     d2,
			value: d2,
			str:   "2020-01-30",
		},
		{
			data:  "null",
			v:     time.Time{},
			value: nil,
			str:   "",
			empty: true,
		},
		{
			data:  `"blaaa"`,
			v:     time.Time{},
			value: time.Time{},
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
		{
			data:  "15",
			v:     time.Time{},
			value: time.Time{},
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))

	t.Run("values", runValuesField(func() forms.Field {
		return forms.NewDatetimeField("")
	}, []fieldTest[[]string, time.Time]{
		{
			data:  []string{""},
			v:     time.Time{},
			value: nil,
			str:   "",
			empty: true,
		},
		{
			data:  []string{"2020-01-30"},
			v:     d1,
			value: d1,
			str:   "2020-01-30",
		},
		{
			data:  []string{"2020-01-30 14:24:06"},
			v:     d2,
			value: d2,
			str:   "2020-01-30",
		},
		{
			data:  []string{"\uff00"},
			v:     time.Time{},
			value: nil,
			str:   "",
			empty: true,
		},
		{
			data:  []string{"blaaa"},
			v:     time.Time{},
			value: time.Time{},
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))
}

func TestTextListField(t *testing.T) {
	t.Run("set", runSetField(func() forms.Field {
		return forms.NewTextListField("")
	}, []fieldTest[any, []string]{
		{
			data:  nil,
			value: nil,
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{},
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{"value"},
			value: []string{"value"},
			v:     []string{"value"},
			str:   "value",
		},
		{
			data:  new([]string{"pointer", "value"}),
			value: []string{"pointer", "value"},
			v:     []string{"pointer", "value"},
			str:   "pointer, value",
		},
		{
			data:  12,
			value: nil,
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []int{1, 2, 3},
			value: nil,
			v:     []string(nil),
			str:   "",
			empty: true,
		},
	}))

	t.Run("json", runJSONField(func() forms.Field {
		return forms.NewTextListField("")
	}, []fieldTest[string, []string]{
		{
			data:  "null",
			value: nil,
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  "//",
			value: nil,
			v:     []string(nil),
			str:   "",
			empty: true,
			err:   forms.ErrInvalidValue,
		},
		{
			data:  `[]`,
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  `["a", "b", "c"]`,
			value: []string{"a", "b", "c"},
			v:     []string{"a", "b", "c"},
			str:   "a, b, c",
		},
		{
			data:  `["a", null, "b", "c"]`,
			value: []string{"a", "b", "c"},
			v:     []string{"a", "b", "c"},
			str:   "a, b, c",
		},
		{
			data:  `[null]`,
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  `[""]`,
			value: []string{""},
			v:     []string{""},
			str:   "",
		},
		{
			data:  `123`,
			value: nil,
			v:     []string(nil),
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
		{
			data:  `["a", 1, 2, null]`,
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
		{
			data:  `[1, 2, 3]`,
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))

	t.Run("values", runValuesField(func() forms.Field {
		return forms.NewTextListField("")
	}, []fieldTest[[]string, []string]{
		{
			data:  []string{},
			value: nil,
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{"\uff00"},
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{"a"},
			value: []string{"a"},
			v:     []string{"a"},
			str:   "a",
		},
		{
			data:  []string{"a", "b", "12"},
			value: []string{"a", "b", "12"},
			v:     []string{"a", "b", "12"},
			str:   "a, b, 12",
		},
	}))

	t.Run("json trimmed", runJSONField(func() forms.Field {
		return forms.NewTextListField("", forms.Trim)
	}, []fieldTest[string, []string]{
		{
			data:  `[]`,
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  `["\ta  ", "  b\r\n", "c"]`,
			value: []string{"a", "b", "c"},
			v:     []string{"a", "b", "c"},
			str:   "a, b, c",
		},
		{
			data:  `[1, 2, 3]`,
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))

	t.Run("values trimmed", runValuesField(func() forms.Field {
		return forms.NewTextListField("", forms.Trim)
	}, []fieldTest[[]string, []string]{
		{
			data:  []string{},
			value: nil,
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{"\uff00"},
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{"\ta  ", "   b\r\n", "12"},
			value: []string{"a", "b", "12"},
			v:     []string{"a", "b", "12"},
			str:   "a, b, 12",
		},
	}))

	t.Run("json discard empty", runJSONField(func() forms.Field {
		return forms.NewTextListField("", forms.DiscardEmpty)
	}, []fieldTest[string, []string]{
		{
			data:  `[""]`,
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  `["", "abc", ""]`,
			value: []string{"abc"},
			v:     []string{"abc"},
			str:   "abc",
			empty: false,
		},
	}))

	t.Run("values discard empty", runValuesField(func() forms.Field {
		return forms.NewTextListField("", forms.DiscardEmpty)
	}, []fieldTest[[]string, []string]{
		{
			data:  []string{""},
			value: []string(nil),
			v:     []string(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{"", "abc", ""},
			value: []string{"abc"},
			v:     []string{"abc"},
			str:   "abc",
			empty: false,
		},
	}))
}

func TestIntegerListField(t *testing.T) {
	t.Run("set", runSetField(func() forms.Field {
		return forms.NewIntegerListField("")
	}, []fieldTest[any, []int]{
		{
			data:  nil,
			value: nil,
			v:     []int(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []int{},
			value: []int(nil),
			v:     []int(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []int{123},
			value: []int{123},
			v:     []int{123},
			str:   "123",
		},
		{
			data:  new([]int{1, 2, 3}),
			value: []int{1, 2, 3},
			v:     []int{1, 2, 3},
			str:   "1, 2, 3",
		},
		{
			data:  12,
			value: nil,
			v:     []int(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{"a", "b", "c"},
			value: nil,
			v:     []int(nil),
			str:   "",
			empty: true,
		},
	}))

	t.Run("json", runJSONField(func() forms.Field {
		return forms.NewIntegerListField("")
	}, []fieldTest[string, []int]{
		{
			data:  "null",
			value: nil,
			v:     []int(nil),
			str:   "",
			empty: true,
		},
		{
			data:  "//",
			value: nil,
			v:     []int(nil),
			str:   "",
			empty: true,
			err:   forms.ErrInvalidValue,
		},
		{
			data:  `[]`,
			value: []int(nil),
			v:     []int(nil),
			str:   "",
			empty: true,
		},
		{
			data:  `[1, 2, 3]`,
			value: []int{1, 2, 3},
			v:     []int{1, 2, 3},
			str:   "1, 2, 3",
		},
		{
			data:  `[1, null, 2, 3]`,
			value: []int{1, 2, 3},
			v:     []int{1, 2, 3},
			str:   "1, 2, 3",
		},
		{
			data:  `[null]`,
			value: []int(nil),
			v:     []int(nil),
			str:   "",
			empty: true,
		},
		{
			data:  `123`,
			value: nil,
			v:     []int(nil),
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
		{
			data:  `["a", 1, 2, null]`,
			value: []int(nil),
			v:     []int(nil),
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
		{
			data:  `["a", "b", "c"]`,
			value: []int(nil),
			v:     []int(nil),
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))

	t.Run("values", runValuesField(func() forms.Field {
		return forms.NewIntegerListField("")
	}, []fieldTest[[]string, []int]{
		{
			data:  []string{},
			value: nil,
			v:     []int(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{"\uff00"},
			value: []int(nil),
			v:     []int(nil),
			str:   "",
			empty: true,
		},
		{
			data:  []string{"1", "2", "3"},
			value: []int{1, 2, 3},
			v:     []int{1, 2, 3},
			str:   "1, 2, 3",
		},
		{
			data:  []string{"1", "\uff00", "2", "3"},
			value: []int{1, 2, 3},
			v:     []int{1, 2, 3},
			str:   "1, 2, 3",
		},
		{
			data:  []string{"1", "2", "c"},
			value: []int(nil),
			v:     []int(nil),
			str:   "",
			err:   forms.ErrInvalidValue,
			empty: true,
		},
	}))
}

func TestTranslatedErrors(t *testing.T) {
	tests := []struct {
		tr       forms.Translator
		errors   []error
		expected []string
	}{
		{
			nil,
			[]error{errors.New("test"), forms.Gettext("values %s", "a")},
			[]string{"test", "values a"},
		},
		{
			prefixTranslator("prefix"),
			[]error{errors.New("test"), forms.Gettext("values %s", "a")},
			[]string{"test", "prefix:values a"},
		},
		{
			prefixTranslator("prefix"),
			[]error{errors.New("test"), forms.Pgettext("ctx", "values %s", "a")},
			[]string{"test", "ctx:prefix:values a"},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := require.New(t)

			field := forms.NewTextField("test")
			field.SetContext(forms.WithTranslator(field.Context(), test.tr))
			field.AddErrors(test.errors...)

			errs := []string{}
			for _, err := range field.Errors() {
				errs = append(errs, err.Error())
			}

			assert.Equal(test.expected, errs)
		})
	}
}
