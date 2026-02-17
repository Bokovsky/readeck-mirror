// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package libjet

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIndirect(t *testing.T) {
	sV := "test"
	sB := true
	sI := 12
	var pV *string

	type fooType struct {
		v int
	}
	var pF *fooType
	sF := fooType{}

	values := []struct {
		v      any
		expect any
		isNil  bool
	}{
		{sV, "test", false},
		{&sV, "test", false},
		{pV, nil, true},
		{sB, true, false},
		{&sB, true, false},
		{sI, 12, false},
		{&sI, 12, false},
		{fooType{}, fooType{}, false},
		{pF, nil, true},
		{sF, fooType{}, false},
		{fooType{2}, fooType{2}, false},
	}

	for _, tt := range values {
		r, isNil := Indirect(reflect.ValueOf(tt.v))
		require.Exactly(t, tt.expect, r, "%#v", tt.v)
		require.Equal(t, tt.isNil, isNil, "%#v", tt.v)
	}
}

func TestToString(t *testing.T) {
	sV := "test"
	sB := true
	sI := 12
	var pV *string

	type fooType struct {
		v int
	}
	var pF *fooType
	sF := fooType{}

	tests := []struct {
		v      any
		expect string
	}{
		{sV, "test"},
		{&sV, "test"},
		{pV, ""},
		{sB, "true"},
		{&sB, "true"},
		{sI, "12"},
		{&sI, "12"},
		{45.5, "45.5"},
		{fooType{}, "{0}"},
		{pF, ""},
		{sF, "{0}"},
		{fooType{2}, "{2}"},
		{[]byte("test"), "test"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+i), func(t *testing.T) {
			v := ToString(reflect.ValueOf(test.v))
			require.Equal(t, test.expect, v, "%#v", test.v)
		})
	}
}

func TestToInt(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		require.Exactly(t, int(10), ToInt[int](reflect.ValueOf(10)))
	})
	t.Run("int32", func(t *testing.T) {
		require.Exactly(t, int32(10), ToInt[int32](reflect.ValueOf(10)))
	})
	t.Run("int64", func(t *testing.T) {
		require.Exactly(t, int64(10), ToInt[int64](reflect.ValueOf(10)))
	})
	t.Run("uint32", func(t *testing.T) {
		require.Exactly(t, uint32(10), ToInt[uint32](reflect.ValueOf(10)))
	})
	t.Run("uint64", func(t *testing.T) {
		require.Exactly(t, uint64(10), ToInt[uint64](reflect.ValueOf(10)))
	})
	t.Run("uint", func(t *testing.T) {
		require.Exactly(t, uint(10), ToInt[uint](reflect.ValueOf(10)))
	})
	t.Run("float32", func(t *testing.T) {
		require.Exactly(t, int32(10), ToInt[int32](reflect.ValueOf(10.8)))
	})
	t.Run("float64", func(t *testing.T) {
		require.Exactly(t, int64(10), ToInt[int64](reflect.ValueOf(10.8)))
	})
	t.Run("error", func(t *testing.T) {
		require.PanicsWithValue(t, "value is not a number", func() {
			ToInt[int](reflect.ValueOf("10"))
		})
	})
}
