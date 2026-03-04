// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package forms provides helpers and functions to create and validate forms.
package forms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"mime"
	"net/http"
	"net/url"
)

var ctxFormKey = &contextKey{"form"}

var (
	// ErrInvalidInput is the error for invalid data.
	ErrInvalidInput = errors.New("invalid input data")
	// ErrUnexpected is an error that can be used during custom validation
	// or form actions.
	ErrUnexpected = Gettext("an unexpected error has occurred")
)

// Binder describes the basic needed method for a form that can be bound
// from JSON or URL values.
type Binder interface {
	Contexter
	Fields() iter.Seq2[string, Field]
	Get(string) Field
	Bind()
	IsBound() bool
	IsValid() bool
	AddErrors(string, ...error)
	Errors() Errors
}

type marshalledForm struct {
	IsValid bool             `json:"is_valid"`
	Errors  Errors           `json:"errors"`
	Fields  map[string]Field `json:"fields"`
}

// Form is a list of fields.
type Form struct {
	isBound  bool
	fields   []Field
	fieldMap map[string]int
	context  context.Context
	errors   Errors
}

// New returns a new Form instance.
// The context is passed to the field's own context using a naive merge strategy.
// That means that the global form's context is accessible to all fields, which can be used
// for translation or by validators.
func New(ctx context.Context, fields ...Field) (*Form, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	f := &Form{
		context:  ctx,
		fields:   []Field{},
		fieldMap: map[string]int{},
	}

	for _, field := range fields {
		if field.Name() == "" {
			return nil, errors.New("unamed field")
		}
		if _, exists := f.fieldMap[field.Name()]; exists {
			return nil, fmt.Errorf(`field "%s" already defined`, field.Name())
		}
		f.fields = append(f.fields, field)
		f.fieldMap[field.Name()] = len(f.fields) - 1
	}

	// Pass the form's context — with the form itself — to each field
	for _, field := range f.fields {
		field.SetContext(mergeContext(
			field.Context(),
			context.WithValue(f.context, ctxFormKey, f),
		))
	}

	return f, nil
}

// Must returns a new Form instance and panics if there was any error.
func Must(ctx context.Context, fields ...Field) *Form {
	f, err := New(ctx, fields...)
	if err != nil {
		panic(err)
	}
	return f
}

// Context returns the form's context.
func (f Form) Context() context.Context {
	return f.context
}

// SetContext sets the form's context.
func (f *Form) SetContext(ctx context.Context) {
	f.context = ctx
}

// Fields returns the form's field list.
func (f *Form) Fields() iter.Seq2[string, Field] {
	return iterFields(f.fields)
}

// Get returns a field by its name, or nil when it doesn't exist.
func (f *Form) Get(name string) Field {
	if i, ok := f.fieldMap[name]; ok {
		return f.fields[i]
	}
	return nil
}

// Bind set the form as bound.
func (f *Form) Bind() {
	f.isBound = true
}

// IsBound returns true if the form has been bound to input data.
func (f *Form) IsBound() bool {
	return f.isBound
}

// IsValid returns true if the form has no error and all fields are valid.
func (f *Form) IsValid() bool {
	if !f.IsBound() {
		return true
	}

	// We must first ensure that each field's validator is processed.
	ok := true
	for _, field := range f.Fields() {
		ok = field.IsValid() && ok
	}

	return len(f.errors) == 0 && ok
}

// AddErrors adds errors to the form or one of its fields.
// An empty name adds the errors go to the form itself.
func (f *Form) AddErrors(name string, errs ...error) {
	if len(errs) == 0 {
		return
	}

	if name == "" {
		tr := GetTranslator(f.context)
		for _, err := range errs {
			if err == nil {
				continue
			}
			f.errors = append(f.errors, localizedError{err: err, tr: tr})
		}
		return
	}

	if field := f.Get(name); field != nil {
		field.AddErrors(errs...)
	}
}

// Errors returns the form's [Errors].
func (f *Form) Errors() Errors {
	return f.errors
}

// MarshalJSON returns the JSON serialization of a form.
func (f *Form) MarshalJSON() ([]byte, error) {
	return formMarshalJSON(f)
}

// unmarshalJSON decodes JSON values into the form.
// It does so by decoding first the input value into a map
// of raw values. Then each registered field that's present in
// the resulting map is decoded.
func unmarshalJSON(f Binder, r io.Reader) {
	values := map[string]json.RawMessage{}
	if err := json.NewDecoder(r).Decode(&values); err != nil {
		f.AddErrors("", ErrInvalidInput)
		return
	}

	for _, field := range f.Fields() {
		data, exists := values[field.Name()]
		if !exists {
			continue
		}
		if err := field.UnmarshalJSON(data); err != nil {
			continue
		}
	}
}

// unmarshalValues decodes url encoded values into the form.
// It passes every value item to each matching [Field.unmarshalValues].
func unmarshalValues(f Binder, values url.Values) {
	for _, field := range f.Fields() {
		v, exists := values[field.Name()]
		if !exists {
			continue
		}

		if err := field.UnmarshalValues(v); err != nil {
			continue
		}
	}
}

func unmarshalMultipart(f Binder, r *http.Request) {
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(16 << 20); err != nil {
			f.AddErrors("", errors.New("error loading data"))
			return
		}
	}

	// Bind the file fields
	for name, headers := range r.MultipartForm.File {
		if len(headers) == 0 {
			continue
		}
		field := f.Get(name)
		if field == nil {
			continue
		}

		if field, ok := field.(HeaderReader); ok {
			if err := field.UnmarshalFiles(headers); err != nil {
				continue
			}
		}
	}

	// Always finish with loading the regular values
	unmarshalValues(f, r.Form)
}

// LoadValues loads the values from any JSON marshal enabled value.
func LoadValues(f Binder, v any) error {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(v); err != nil {
		return err
	}
	unmarshalJSON(f, buf)

	return nil
}

// Bind loads the data using the method tied
// to the request's content-type header.
func Bind(f Binder, r *http.Request) {
	if f.IsBound() {
		f.AddErrors("", errors.New("form is already bound"))
		return
	}

	f.Bind()
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("content-type"))
	if err != nil {
		f.AddErrors("", errors.New("Invalid content-type"))
		return
	}

	switch mediaType {
	case "application/json", "text/json":
		defer r.Body.Close() //nolint:errcheck
		unmarshalJSON(f, r.Body)
	case "application/x-www-form-urlencoded":
		if err := r.ParseForm(); err != nil {
			f.AddErrors("", errors.New("invalid input"))
		}
		unmarshalValues(f, r.Form)
	case "multipart/form-data":
		unmarshalMultipart(f, r)
	default:
		f.AddErrors("", errors.New("Unknown content-type"))
	}

	// Validate the form
	f.IsValid()
	if f, ok := f.(interface{ Validate() }); ok {
		f.Validate()
	}
}

// BindValues binds a form using [url.Values] parameters.
func BindValues(f Binder, values url.Values) {
	if f.IsBound() {
		f.AddErrors("", errors.New("form is already bound"))
		return
	}
	f.Bind()
	unmarshalValues(f, values)
	f.IsValid()
	if f, ok := f.(interface{ Validate() }); ok {
		f.Validate()
	}
}

// BindURL binds a form using its URL parameters only.
func BindURL(f Binder, r *http.Request) {
	BindValues(f, r.URL.Query())
}

func iterFields(fields []Field) iter.Seq2[string, Field] {
	return func(yield func(string, Field) bool) {
		for _, field := range fields {
			if !yield(field.Name(), field) {
				return
			}
		}
	}
}

func formMarshalJSON(f Binder) ([]byte, error) {
	res := marshalledForm{
		IsValid: f.IsValid(),
		Errors:  f.Errors(),
		Fields:  map[string]Field{},
	}
	maps.Insert(res.Fields, f.Fields())

	return json.Marshal(res)
}
