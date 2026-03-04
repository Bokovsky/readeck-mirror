// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package forms_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/forms"
)

type joinedForm struct {
	*forms.JoinedForms
}

func newJoinedForm(ctx context.Context) *joinedForm {
	return &joinedForm{
		forms.Join(
			ctx,
			newSimpleForm(),
			forms.Must(context.Background(),
				forms.NewTextField("name", forms.Required),
				forms.NewBooleanField("active",
					forms.FieldValidatorFunc(func(f forms.Field) error {
						if f.Context().Value(ctxSimpleKey{}) != nil {
							return forms.Gettext("context error")
						}
						return nil
					}),
				),
			),
		),
	}
}

func (f *joinedForm) Validate() {
	f.JoinedForms.Validate()
	if f.Context().Value(ctxSimpleKey{}) != nil {
		f.AddErrors("", forms.Gettext("context error"))
		f.AddErrors("name", forms.Gettext("context error on name"))
	}
}

func TestJoin(t *testing.T) {
	F1 := func() forms.Binder {
		return forms.Must(context.Background(),
			forms.NewTextField("name"),
			forms.NewIntegerField("amount", forms.Gte(0)),
		)
	}
	F2 := func() forms.Binder {
		return forms.Must(context.Background(),
			forms.NewTextField("name", forms.Required),
			forms.NewTextField("address", forms.Required),
		)
	}

	t.Run("join", func(t *testing.T) {
		f1 := F1()
		f2 := F2()
		form := forms.Join(context.Background(), f1, f2)

		assert := require.New(t)
		assert.Exactly(form.Get("name"), f2.Get("name"))
		assert.Exactly(form.Get("amount"), f1.Get("amount"))
		assert.Exactly(form.Get("address"), f2.Get("address"))
	})

	t.Run("nested validation", runRequestForm(mimeValues, func() forms.Binder {
		return newJoinedForm(context.WithValue(context.Background(), ctxSimpleKey{}, ""))
	}, []formTest{
		{
			`{}`,
			`{
				"is_valid": false,
				"errors": [
					"simple context error",
					"context error"
				],
				"fields": {
					"active": {
						"is_null": true,
						"is_bound": false,
						"value": false,
						"errors": [
							"context error"
						]
					},
					"id": {
						"is_null": true,
						"is_bound": false,
						"value": 0,
						"errors": [
							"field is required"
						]
					},
					"name": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": [
							"field is required",
							"context error on name"
						]
					}
				}
				}`,
		},
	}))

	t.Run("request", runRequestForm(mimeJSON, func() forms.Binder {
		return forms.Join(context.Background(), F1(), F2())
	}, []formTest{
		{
			`{}`,
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
					"address": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": [
							"field is required"
						]
					},
					"amount": {
						"is_null": true,
						"is_bound": false,
						"value": 0,
						"errors": null
					},
					"name": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": [
							"field is required"
						]
					}
				}
			}`,
		},
		{
			`{
				"name": "alice",
				"address": "Amsterdam",
				"amount": 20
			 }`,
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"address": {
						"is_null": false,
						"is_bound": true,
						"value": "Amsterdam",
						"errors": null
					},
					"amount": {
						"is_null": false,
						"is_bound": true,
						"value": 20,
						"errors": null
					},
					"name": {
						"is_null": false,
						"is_bound": true,
						"value": "alice",
						"errors": null
					}
				}
			}`,
		},
	}))

	t.Run("AddErrors", func(t *testing.T) {
		ctx := context.Background()

		f1, err := forms.New(ctx)
		require.NoError(t, err)
		f2, err := forms.New(ctx)
		require.NoError(t, err)

		form := forms.Join(ctx, f1, f2)
		req, err := http.NewRequestWithContext(ctx, "GET", "/path", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		require.NoError(t, err)
		forms.Bind(form, req)

		assert.True(t, form.IsValid())
		form.AddErrors("", errors.New("explode"))
		form.AddErrors("", errors.New("fail"))
		assert.False(t, form.IsValid())
		assert.Equal(t, "explode, fail", form.Errors().String())
	})
}
