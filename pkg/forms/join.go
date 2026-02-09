// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package forms

import (
	"context"
	"iter"
)

func backward[E any](s []E) func(func(int, E) bool) {
	return func(yield func(int, E) bool) {
		for i := len(s) - 1; i >= 0; i-- {
			if !yield(i, s[i]) {
				return
			}
		}
	}
}

// JoinedForms is a [Binder] that combines several forms.
type JoinedForms struct {
	isBound  bool
	forms    []Binder
	fields   []Field
	fieldMap map[string]int
	context  context.Context
	errors   Errors
}

// Join returns a new [JoinedForms] instance.
// All the forms' fields are inserted from the last provided form to the
// first one. The last defined field has then priority over the first one,
// when they share the same name.
// The context is added to each mapped field.
func Join(ctx context.Context, forms ...Binder) *JoinedForms {
	res := &JoinedForms{
		forms:    forms,
		context:  ctx,
		fields:   []Field{},
		fieldMap: map[string]int{},
	}

	// Iterate over all the fields in reverse order. The last form's fields have priority
	// over previously defined fields with the same name.
	i := 0
	for _, form := range backward(forms) {
		form.SetContext(mergeContext(form.Context(), res.context))

		for name, field := range form.Fields() {
			if _, exists := res.fieldMap[name]; !exists {
				field.SetContext(mergeContext(field.Context(), res.context))
				res.fields = append(res.fields, field)
				res.fieldMap[name] = i
				i++
			}
		}
	}

	return res
}

// Context returns the form's context.
func (f JoinedForms) Context() context.Context {
	return f.context
}

// SetContext sets the form's context.
func (f *JoinedForms) SetContext(ctx context.Context) {
	f.context = ctx
}

// Fields returns the form's field list.
func (f *JoinedForms) Fields() iter.Seq2[string, Field] {
	return iterFields(f.fields)
}

// Get returns a field by its name, or nil when it doesn't exist.
func (f *JoinedForms) Get(name string) Field {
	if i, ok := f.fieldMap[name]; ok {
		return f.fields[i]
	}
	return nil
}

// Bind set the form as bound.
func (f *JoinedForms) Bind() {
	// We must bind all the forms in order for validators to run.
	for _, form := range f.forms {
		form.Bind()
	}
	f.isBound = true
}

// IsBound returns true if the form has been bound to input data.
func (f *JoinedForms) IsBound() bool {
	return f.isBound
}

// IsValid returns true when all the forms are valid.
func (f *JoinedForms) IsValid() bool {
	if !f.IsBound() {
		return true
	}

	if len(f.errors) > 0 {
		return false
	}

	ok := true
	for _, form := range f.forms {
		ok = form.IsValid() && ok
	}
	return ok
}

// AddErrors adds errors to the form or one of its fields.
// An empty name adds the errors go to the form itself.
func (f *JoinedForms) AddErrors(name string, errs ...error) {
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

// Errors returns all the forms errors and [JoinedForms] own error list.
func (f *JoinedForms) Errors() Errors {
	res := Errors{}
	for _, form := range f.forms {
		res = append(res, form.Errors()...)
	}
	res = append(res, f.errors...)

	return res
}

// Validate runs forms validators when they implement a Validate function.
func (f *JoinedForms) Validate() {
	for _, form := range f.forms {
		if form, ok := form.(interface{ Validate() }); ok {
			form.Validate()
		}
	}
}

// MarshalJSON returns the JSON serialization of a form.
func (f *JoinedForms) MarshalJSON() ([]byte, error) {
	return formMarshalJSON(f)
}
