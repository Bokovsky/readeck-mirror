// SPDX-FileCopyrightText: Copyright (c) 2020 Artyom Pervukhin
//
// SPDX-License-Identifier: MIT

// Package csvstruct allows scanning of string slice obtained from a
// csv.Reader.Read call into a struct type.
//
// It supports scanning values to string, integer, float, boolean struct
// fields, and fields with the types implementing encoding.TextUnmarshaler or
// Value interfaces.
package csvstruct

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

// Scanner is a function that scans CSV row to dst, which must be a pointer to
// a struct type. Scanner must be called on the same type it was created from
// by the NewScanner call.
type Scanner func(row []string, dst any) error

func colIdx(header []string, f reflect.StructField) int {
	ignoreCase := f.Tag.Get("case") == "ignore"

	idxFn := func(x string) int {
		return slices.IndexFunc(header, func(v string) bool {
			if !ignoreCase {
				return x == v
			}
			return strings.EqualFold(x, v)
		})
	}

	for t := range strings.SplitSeq(f.Tag.Get("csv"), ",") {
		if i := idxFn(t); i != -1 {
			return i
		}
	}

	return idxFn(f.Name)
}

// NewScanner takes CSV header and dst which must be a pointer to a struct type
// with struct "csv" tags mapped to header names, and returns a Scanner
// function for this type and field ordering. It does not modify dst.
//
// Only exported fields are processed.
func NewScanner(header []string, dst any) (Scanner, error) { // nolint:gocognit,gocyclo
	st := reflect.ValueOf(dst)
	if st.Kind() != reflect.Ptr {
		panic("csvstruct: dst must be a pointer to a struct type")
	}
	st = reflect.Indirect(st)
	if !st.IsValid() || st.Type().Kind() != reflect.Struct {
		panic("csvstruct: dst must be a pointer to a struct type")
	}
	var setters []setter
	for i := 0; i < st.NumField(); i++ {
		field := st.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Tag.Get("csv") == "-" {
			continue
		}

		csvIdx := colIdx(header, field)
		if csvIdx == -1 {
			continue
		}
		val := st.Field(i)

		var fn func(reflect.Value, string) error // setter.fn
		if val.CanAddr() && val.Addr().Type().Implements(textUnmarshalerType) {
			fn = func(field reflect.Value, s string) error {
				return field.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s))
			}
			setters = append(setters, setter{
				csvIdx:   csvIdx,
				fieldIdx: i,
				fn:       fn,
			})
			continue
		}

		switch val.Interface().(type) {
		case int:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseInt(s, 0, 0)
				if err != nil {
					return err
				}
				field.SetInt(x)
				return nil
			}
		case int8:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseInt(s, 0, 8)
				if err != nil {
					return err
				}
				field.SetInt(x)
				return nil
			}
		case int16:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseInt(s, 0, 16)
				if err != nil {
					return err
				}
				field.SetInt(x)
				return nil
			}
		case int32:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseInt(s, 0, 32)
				if err != nil {
					return err
				}
				field.SetInt(x)
				return nil
			}
		case int64:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseInt(s, 0, 64)
				if err != nil {
					return err
				}
				field.SetInt(x)
				return nil
			}
		case uint:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseUint(s, 0, 0)
				if err != nil {
					return err
				}
				field.SetUint(x)
				return nil
			}
		case uint8:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseUint(s, 0, 8)
				if err != nil {
					return err
				}
				field.SetUint(x)
				return nil
			}
		case uint16:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseUint(s, 0, 16)
				if err != nil {
					return err
				}
				field.SetUint(x)
				return nil
			}
		case uint32:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseUint(s, 0, 32)
				if err != nil {
					return err
				}
				field.SetUint(x)
				return nil
			}
		case uint64:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseUint(s, 0, 64)
				if err != nil {
					return err
				}
				field.SetUint(x)
				return nil
			}
		case float32:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseFloat(s, 32)
				if err != nil {
					return err
				}
				field.SetFloat(x)
				return nil
			}
		case float64:
			fn = func(field reflect.Value, s string) error {
				x, err := strconv.ParseFloat(s, 64)
				if err != nil {
					return err
				}
				field.SetFloat(x)
				return nil
			}
		case bool:
			fn = func(field reflect.Value, s string) error {
				val, err := strconv.ParseBool(s)
				if err != nil {
					return err
				}
				field.SetBool(val)
				return nil
			}
		case string:
			fn = func(field reflect.Value, s string) error {
				field.SetString(s)
				return nil
			}
		default:
			return nil, fmt.Errorf("field %q has unsupported type", field.Name)
		}
		setters = append(setters, setter{
			csvIdx:   csvIdx,
			fieldIdx: i,
			fn:       fn,
		})
	}
	if len(setters) == 0 {
		return nil, errors.New("no matches found between header and csv-tagged struct fields")
	}
	originalType := reflect.ValueOf(dst).Type()
	return func(row []string, dst any) error {
		st := reflect.ValueOf(dst)
		if st.Type() != originalType {
			panic("csvstruct: Scanner called on the different type from the one used in the NewScanner call")
		}
		st = reflect.Indirect(st)
		for _, s := range setters {
			if err := s.fn(st.Field(s.fieldIdx), row[s.csvIdx]); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

type setter struct {
	csvIdx   int
	fieldIdx int
	fn       func(field reflect.Value, s string) error
}
