// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package forms

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

var (
	// ErrRequired is a required field.
	ErrRequired = Gettext("field is required")
	// ErrInvalidEmail is an invalid e-mail address.
	ErrInvalidEmail = Gettext("not a valid email address")
	// ErrInvalidURL is an invalid URL.
	ErrInvalidURL = Gettext("invalid URL")
)

var (
	ctxCleanersKey   = &contextKey{"cleaners"}
	ctxValidatorsKey = &contextKey{"validators"}
	ctxChoicesKey    = &contextKey{"choices"}
)

var errValidationSkip = errors.New("skip")

func applyFieldOptions[T any](f Field, options ...any) {
	var cleaners []ValueCleaner
	var validators []Validator

	for _, option := range options {
		switch t := option.(type) {
		case FieldOption:
			t(f)
		case ValueCleaner:
			cleaners = append(cleaners, t)
		case FieldValidator, ValueValidator[T]:
			validators = append(validators, t)
		}
	}

	SetCleaners(f, append(GetCleaners(f), cleaners...)...)
	SetValidators(f, append(GetValidators(f), validators...)...)
}

// ApplyValidators runs the given [FieldValidator] and [ValueValidator] on the field and a value.
// The value can be of type T of []T. When it's a list ([]T), each item will pass through each
// [ValueValidator] instance.
// It returns a list of unwrapped errors.
func ApplyValidators[T any](f Field, value any, validators ...Validator) (errs []error) {
	for _, validator := range validators {
		var fn func() error

		switch validator := validator.(type) {
		case FieldValidator:
			// On a [FieldValidator], we just need to run it.
			fn = func() error {
				return validator.ValidateField(f)
			}
		case ValueValidator[T]:
			// On a [ValueValidator], we must run it on each value when it's a list of T.
			switch value := value.(type) {
			case []T:
				fn = func() error {
					res := []error{}
					for _, v := range value {
						res = append(res, validator.ValidateValue(f, v))
					}
					return errors.Join(res...)
				}
			case T:
				fn = func() error {
					return validator.ValidateValue(f, value)
				}
			}
		}

		if fn != nil {
			err := fn()
			if err == nil {
				continue
			}

			// We unwrap the error as a list so we can catch
			// any embedded skip or fatal error.
			for _, err := range unwrapErrors(err) {
				if errors.Is(err, errValidationSkip) {
					// skip the rest of the validation
					return
				}

				errs = append(errs, err)
				if _, ok := err.(*fatalError); ok {
					// stop validation process
					return
				}
			}
		}
	}

	return errs
}

// Validator describes a generic validator.
// By default, it can be anything but, once attached to a field, relevant
// interfaces are called during cleanup and validation steps.
type Validator interface{}

// ValueCleaner describes a value cleaner.
type ValueCleaner interface {
	Clean(v any) any
}

// FieldValidator describes a field validator (not its value).
type FieldValidator interface {
	ValidateField(f Field) error
}

// ValueValidator describes a value validator.
type ValueValidator[T any] interface {
	ValidateValue(f Field, v T) error
}

// CleanerFunc is a [ValueCleaner].
type CleanerFunc func(v any) any

// Clean implements [ValueCleaner].
func (c CleanerFunc) Clean(v any) any {
	return c(v)
}

// FieldValidatorFunc is a [FieldValidator].
type FieldValidatorFunc func(f Field) error

// ValidateField implements [FieldValidator].
func (c FieldValidatorFunc) ValidateField(f Field) error {
	return c(f)
}

// ValueValidatorFunc is a [ValueValidator].
type ValueValidatorFunc[T any] func(f Field, v T) error

// ValidateValue implements [ValueValidator].
func (c ValueValidatorFunc[T]) ValidateValue(f Field, v T) error {
	return c(f, v)
}

// SkipValidation returns an error that stops any subsequent validator.
func SkipValidation() error {
	return errValidationSkip
}

// Contexter describes a contexter getter and setter.
type Contexter interface {
	Context() context.Context
	SetContext(context.Context)
}

// FieldOption is a function that runs uppon a field's creation.
type FieldOption func(f Field)

// GetCleaners returns the field's [ValueCleaner]s.
func GetCleaners(f Field) []ValueCleaner {
	v, _ := f.Context().Value(ctxCleanersKey).([]ValueCleaner)
	return v
}

// SetCleaners sets a list of [ValueCleaner] to a field.
func SetCleaners(f Field, cleaners ...ValueCleaner) {
	f.SetContext(context.WithValue(f.Context(), ctxCleanersKey, cleaners))
}

// GetValidators returns the field's [Validator]s.
func GetValidators(f Field) []Validator {
	v, _ := f.Context().Value(ctxValidatorsKey).([]Validator)
	return v
}

// SetValidators sets a list of [Validator] to a field.
func SetValidators(f Field, validators ...Validator) {
	f.SetContext(context.WithValue(f.Context(), ctxValidatorsKey, validators))
}

// Default returns a [FieldOption] that sets a field's default value.
func Default(v any) FieldOption {
	return func(f Field) {
		f.Set(v)
	}
}

// DefaultFunc returns a [FieldOption] that sets a field's default value
// using a function.
func DefaultFunc(fn func(Field) any) FieldOption {
	return func(f Field) {
		f.Set(fn(f))
	}
}

// ConditionValidator is a wrapper that runs different validators based
// on a provided condition.
type ConditionValidator[T any] struct {
	condition func(f Field, value T) bool
	whenTrue  []Validator
	whenFalse []Validator
}

// When creates a new [ConditionValidator].
func When[T any](condition func(f Field, value T) bool) *ConditionValidator[T] {
	return &ConditionValidator[T]{condition: condition}
}

// True adds the validators to the "condition true" validators.
func (c *ConditionValidator[T]) True(validators ...Validator) *ConditionValidator[T] {
	c.whenTrue = append(c.whenTrue, validators...)
	return c
}

// False adds the validators to the "condition false" validators.
func (c *ConditionValidator[T]) False(validators ...Validator) *ConditionValidator[T] {
	c.whenFalse = append(c.whenFalse, validators...)
	return c
}

// ValidateValue implements [ValueValidator].
func (c *ConditionValidator[T]) ValidateValue(f Field, v T) error {
	if c.condition(f, v) {
		return errors.Join(ApplyValidators[T](f, v, c.whenTrue...)...)
	}

	return errors.Join(ApplyValidators[T](f, v, c.whenFalse...)...)
}

// Optional return a [When] validator that runs validators only when the field
// is not null and not empty.
func Optional[T any](validators ...Validator) ValueValidator[T] {
	return When(func(f Field, _ T) bool {
		return f.IsNil() || f.IsEmpty() || f.String() == ""
	}).False(validators...)
}

// Trim is a [ValueCleaner] that trims spaces on string values.
var Trim = CleanerFunc(func(v any) any {
	if v, ok := v.(string); ok {
		return strings.TrimSpace(v)
	}
	return v
})

// DiscardEmpty is a [ValueCleaner] that turns an empty string to a nil value.
// This can be used in [ListField] where nil values are discard and you need to
// discard empty ones as well.
var DiscardEmpty = CleanerFunc(func(v any) any {
	if v, ok := v.(string); ok && v == "" {
		return nil
	}
	return v
})

// Required is a [FieldValidator] that returns an error when a field is null, not bound or empty.
var Required = FieldValidatorFunc(func(f Field) error {
	if !f.IsBound() || f.IsEmpty() || f.IsNil() {
		return FatalError(ErrRequired)
	}
	return nil
})

// RequiredOrNil is a [FieldValidator] that returns an error when the field is empty but not null.
var RequiredOrNil = FieldValidatorFunc(func(f Field) error {
	if !f.IsNil() && f.IsEmpty() {
		return FatalError(ErrRequired)
	}
	return nil
})

// Skip skips subsequent validators when the field is null or empty.
var Skip = FieldValidatorFunc(func(f Field) error {
	if f.IsNil() || f.IsEmpty() || f.String() == "" {
		return SkipValidation()
	}
	return nil
})

// TypedValidator is a helper function that returns a [ValueValidator] from a
// validation function and an error message.
func TypedValidator[T any](validator func(T) bool, err error) ValueValidator[T] {
	return ValueValidatorFunc[T](func(f Field, v T) error {
		if f.IsNil() {
			return nil
		}

		if !validator(v) {
			return err
		}
		return nil
	})
}

// IsEmail performs a rough check of the email address. That is, it
// only checks for the presence of "@", only once and in the string.
var IsEmail = TypedValidator(func(v string) bool {
	return strings.Count(v, "@") == 1 && !strings.HasPrefix(v, "@") && !strings.HasSuffix(v, "@")
}, ErrInvalidEmail)

// IsURL checks that the input value is a valid URL
// and matches the given schemes.
func IsURL(schemes ...string) ValueValidator[string] {
	return TypedValidator(func(v string) bool {
		u, err := url.Parse(v)
		if err != nil {
			return false
		}

		if !slices.Contains(schemes, u.Scheme) {
			return false
		}
		return u.Hostname() != ""
	}, ErrInvalidURL)
}

// Gte is an integer validator that checks
// if a value is greater or equal than a parameter.
func Gte(value int) ValueValidator[int] {
	return TypedValidator(func(v int) bool {
		return v >= value
	}, Gettext("must be greater or equal than %d", value))
}

// Lte is an integer validator that checks
// if a value is lower or equal than a parameter.
func Lte(value int) ValueValidator[int] {
	return TypedValidator(func(v int) bool {
		return v <= value
	}, Gettext("must be lower or equal than %d", value))
}

// StrLen is a string validator that checks if
// its length is between a min/max boundary.
func StrLen(minLen, maxLen int) ValueValidator[string] {
	return TypedValidator(func(s string) bool {
		return len([]rune(s)) >= minLen && len([]rune(s)) <= maxLen
	}, Gettext("text must contain between %d and %d characters", minLen, maxLen))
}

// ValueChoice is a key/value pair.
type ValueChoice[T comparable] struct {
	Name  string
	Value T
}

// In returns true when the choice is present in a list of values.
func (c ValueChoice[T]) In(values []T) bool {
	return slices.ContainsFunc(values, func(x T) bool {
		return x == c.Value
	})
}

// ValueChoices is a list of [ValueChoice].
type ValueChoices[T comparable] []ValueChoice[T]

func (c ValueChoices[T]) String() string {
	res := make([]string, len(c))
	for i, x := range c {
		res[i] = fmt.Sprintf("%v", x.Value)
	}

	return strings.Join(res, ", ")
}

// SetChoices sets the given [Contexter]'s choices.
func SetChoices[T comparable](c Contexter, choices ValueChoices[T]) {
	c.SetContext(context.WithValue(c.Context(), ctxChoicesKey, choices))
}

// GetChoices returns the given [Contexter]'s choices.
func GetChoices[T comparable](c Contexter) ValueChoices[T] {
	v, _ := c.Context().Value(ctxChoicesKey).(ValueChoices[T])
	return v
}

// Choices is a [FieldOption] that adds a choice list to the field.
func Choices[T comparable](choices ...ValueChoice[T]) FieldOption {
	return func(field Field) {
		// Set choices to the field
		SetChoices(field, ValueChoices[T](choices))

		// Choices validator
		SetValidators(field, append(GetValidators(field), ValueValidatorFunc[T](func(f Field, v T) error {
			if f.IsNil() {
				return nil
			}

			choices := GetChoices[T](field)
			errs := []error{}
			for _, choice := range choices {
				if choice.Value == v {
					return nil
				}
			}

			errs = append(errs, Gettext("%v is not one of %s", v, choices))
			return errors.Join(errs...)
		}))...)
	}
}

// ChoicesPairs returns a [Choices] based on pairs of key/value. Strings only.
func ChoicesPairs(items [][2]string) FieldOption {
	choices := make([]ValueChoice[string], len(items))
	for i, item := range items {
		choices[i] = Choice(item[1], item[0])
	}

	return Choices(choices...)
}

// Choice returns a new [ValueChoice] instance.
func Choice[T comparable](name string, value T) ValueChoice[T] {
	return ValueChoice[T]{Name: name, Value: value}
}
