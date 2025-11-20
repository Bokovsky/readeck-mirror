// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package libjet provides some utility functions for Jet templates.
package libjet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/adler32"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/CloudyKit/jet/v6"

	"codeberg.org/readeck/readeck/pkg/utils"
)

type (
	reflectWithString struct {
		v  reflect.Value
		ks string
	}

	orderedMap struct {
		i      int
		keys   []reflectWithString
		values reflect.Value
	}
)

func (m *orderedMap) ProvidesIndex() bool {
	return true
}

func (m *orderedMap) Range() (k reflect.Value, v reflect.Value, done bool) {
	if m.i+1 > len(m.keys) {
		done = true
		return
	}

	k = reflect.ValueOf(m.keys[m.i].ks)
	v = m.values.MapIndex(m.keys[m.i].v)
	m.i++
	return
}

var strType = reflect.TypeOf("")

var funcMap = map[string]jet.Func{
	"string": func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("string", 1, 1)
		return reflect.ValueOf(ToString(a.Get(0)))
	},
	"empty": jet.Func(func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("empty", 1, 1)
		return reflect.ValueOf(IsEmpty(a.Get(0)))
	}),
	"default": func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("default", 2, 2)
		if ToString(a.Get(0)) == "" {
			return a.Get(1)
		}
		return a.Get(0)
	},
	"join": func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("join", 2, 2)
		_, isNil := Indirect(a.Get(0))
		if isNil {
			return reflect.ValueOf("")
		}
		if a.Get(0).Type().Kind() != reflect.Slice {
			panic("invalid list type in join()")
		}

		list := make([]string, a.Get(0).Len())
		for i := range list {
			list[i] = ToString(a.Get(0).Index(i))
		}

		sep := ToString(a.Get(1))

		return reflect.ValueOf(strings.Join(list, sep))
	},
	"isList": func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("isList", 1, 1)
		return reflect.ValueOf(a.Get(0).Kind() == reflect.Slice)
	},
	"checksum": func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("checksum", 1, 1)
		h := adler32.New()
		h.Write([]byte(ToString(a.Get(0))))

		return reflect.ValueOf(strconv.FormatUint(uint64(h.Sum32()), 16))
	},
	"shortText": func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("shortText", 2, 2)
		s := ToString(args.Get(0))
		maxChars := ToInt[int](args.Get(1))

		return reflect.ValueOf(utils.ShortText(s, maxChars))
	},
	"shortURL": func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("shortText", 2, 2)
		s := ToString(args.Get(0))
		maxChars := ToInt[int](args.Get(1))

		return reflect.ValueOf(utils.ShortURL(s, maxChars))
	},
	"humanReadable": func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("sizeBytes", 1, 1)
		i := ToInt[uint64](args.Get(0))

		return reflect.ValueOf(utils.FormatBytes(i))
	},
	"json": func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("json", 1, 2)
		v, _ := Indirect(args.Get(0))
		buf := new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)

		nice, _ := Indirect(args.Get(1))
		if _, ok := nice.(bool); ok {
			enc.SetIndent("", "  ")
		}

		enc.Encode(v) //nolint:errcheck
		return reflect.ValueOf(buf.String())
	},
	"attrList": func(args jet.Arguments) reflect.Value {
		if args.NumOfArguments()%2 > 0 {
			panic("attrList(): incomplete key-value pair")
		}

		res := attrList{}

		for i := 0; i < args.NumOfArguments(); i += 2 {
			k := args.Get(i)
			v := args.Get(i + 1)
			if !k.IsValid() {
				args.Panicf("attrList(): key argument at position %d is not a valid value!", i)
			}
			if !v.IsValid() {
				args.Panicf("attrList(): key argument at position %d is not a valid value!", i+1)
			}
			if !k.Type().ConvertibleTo(strType) {
				args.Panicf("attrList(): can't use %+v as string key: %s is not convertible to string", k, k.Type())
			}
			if v.Kind() == reflect.Bool {
				res[k.String()] = []any{v.Bool()}
				continue
			}
			if !v.Type().ConvertibleTo(strType) {
				args.Panicf("attrList(): can't use %+v as string key: %s is not convertible to string", v, v.Type())
			}

			val, isNil := Indirect(v)
			if !isNil {
				res[k.String()] = []any{val}
			}
		}

		return reflect.ValueOf(res)
	},
	"orderMap": func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("orderMap", 1, 1)

		v := args.Get(0)
		sv := make([]reflectWithString, v.Len())
		mi := v.MapRange()
		for i := 0; mi.Next(); i++ {
			sv[i] = reflectWithString{mi.Key(), ToString(mi.Key())}
		}

		slices.SortFunc(sv, func(i, j reflectWithString) int {
			return strings.Compare(i.ks, j.ks)
		})

		res := &orderedMap{keys: sv, values: v}
		return reflect.ValueOf(res)
	},
}

// FuncMap returns the jet function map.
func FuncMap() map[string]jet.Func {
	return funcMap
}

// VarMap returns the jet global variable map.
func VarMap() map[string]interface{} {
	return map[string]interface{}{
		"unsafeWrite": func(src io.Reader) jet.RendererFunc {
			return func(r *jet.Runtime) {
				io.Copy(r.Writer, src) //nolint:errcheck
			}
		},
	}
}

// AddFuncToSet adds a given function to a jet.Set template set.
func AddFuncToSet(set *jet.Set, key string) {
	if f, ok := funcMap[key]; ok {
		set.AddGlobalFunc(key, f)
	}
}

// Indirect returns the underlying value of a reflect.Value.
// It resolves pointers and indicates if the value is nil.
func Indirect(val reflect.Value) (interface{}, bool) {
	switch val.Kind() {
	case reflect.Invalid:
		return nil, true
	case reflect.Ptr, reflect.Interface:
		if val.IsNil() {
			return nil, true
		}
		return Indirect(val.Elem())
	case reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		if val.IsNil() {
			return nil, true
		}
	}

	return val.Interface(), false
}

// IsEmpty returns true if the value is considered empty.
func IsEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Map, reflect.Slice:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Invalid:
		return true
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			return true
		}
		return IsEmpty(v.Elem())
	case reflect.Struct:
		val, _ := Indirect(v)
		if t, ok := val.(time.Time); ok && t.IsZero() {
			return true
		}
	}
	return false
}

// ToString converts a value to a string.
func ToString(v reflect.Value) string {
	val, isNil := Indirect(v)
	if isNil || val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}

	if val, ok := val.(fmt.Stringer); ok {
		return val.String()
	}

	return fmt.Sprintf("%v", val)
}

// ToInt returns a value as an integer value.
func ToInt[T int | int32 | int64 | uint | uint32 | uint64](v reflect.Value) T {
	val, isNil := Indirect(v)
	if isNil || val == nil {
		return 0
	}

	switch v := val.(type) {
	case float32, float64:
		return T(v.(float64))
	case int64:
		return T(v)
	case int32:
		return T(v)
	case int:
		return T(v)
	case uint64:
		return T(v)
	case uint32:
		return T(v)
	case uint:
		return T(v)
	}

	panic("value is not a number")
}

type attrList map[string][]any

func (l attrList) Render(r *jet.Runtime) {
	i := 0
	//nolint:errcheck // we're writing with a buffer or http.ResponseWriter
	for k, values := range l {
		if len(values) == 1 {
			if x, ok := values[0].(bool); ok && x {
				r.Write([]byte(k))
				r.Write([]byte(" "))
				continue
			} else if ok && !x {
				continue
			}
		}

		r.Writer.Write([]byte(k + `="`))
		for j, x := range values {
			v, err := getString(x)
			if err != nil {
				panic(err)
			}
			r.Write([]byte(v))
			if j+1 < len(values) {
				r.Write([]byte(" "))
			}
		}
		r.Writer.Write([]byte(`"`))
		if i+1 < len(l) {
			r.Write([]byte(" "))
		}
		i++
	}
}

func (l attrList) Add(key string, value any) {
	l[key] = append(l[key], value)
}

func (l attrList) Set(key string, value any) {
	l[key] = []any{value}
}

func getString(input any) (string, error) {
	switch v := input.(type) {
	case string:
		return v, nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%f", v), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	default:
		if s, ok := input.(fmt.Stringer); ok {
			return s.String(), nil
		}
	}

	return "", fmt.Errorf(`cannot convert "%v"`, input)
}
