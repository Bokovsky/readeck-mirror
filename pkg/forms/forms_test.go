// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package forms_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kinbiko/jsonassert"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/forms"
)

type ctxSimpleKey struct{}

type mimeType string

var (
	mimeJSON   mimeType = "application/json"
	mimeValues mimeType = "application/x-www-form-urlencoded"
)

type prefixTranslator string

func (tr prefixTranslator) Gettext(s string, vars ...any) string {
	return fmt.Sprintf("%s:%s", tr, fmt.Sprintf(s, vars...))
}

func (tr prefixTranslator) Pgettext(ctx string, str string, vars ...any) string {
	return fmt.Sprintf("%s:%s", ctx, tr.Gettext(str, vars...))
}

type testResult string

func (result testResult) assert(t *testing.T, f forms.Binder) {
	assert := require.New(t)
	assert.True(f.IsBound())
	data, err := json.Marshal(f)
	assert.NoError(err)

	jsonassert.New(t).Assertf(string(data), "%s", result)
	if t.Failed() {
		t.Errorf("received JSON: %s\n", string(data))
		t.FailNow()
	}
}

type formTest struct {
	data   string
	result testResult
}

type multipartTest struct {
	data   func(*multipart.Writer) error
	result testResult
}

func runRequestForm(contentType mimeType, constructor func() forms.Binder, tests []formTest) func(t *testing.T) {
	return func(t *testing.T) {
		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				f := constructor()
				assert := require.New(t)
				assert.False(f.IsBound())

				r, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(test.data))
				r.Header.Set("content-type", string(contentType))
				forms.Bind(f, r)
				test.result.assert(t, f)
			})
		}
	}
}

func runMultipartForm(constructor func() forms.Binder, tests []multipartTest) func(t *testing.T) {
	return func(t *testing.T) {
		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				f := constructor()
				assert := require.New(t)
				assert.False(f.IsBound())

				body := new(bytes.Buffer)
				mp := multipart.NewWriter(body)
				assert.NoError(test.data(mp))
				assert.NoError(mp.Close())

				r, _ := http.NewRequest(http.MethodPost, "/", body)
				r.Header.Set("content-type", mp.FormDataContentType())
				forms.Bind(f, r)
				test.result.assert(t, f)
			})
		}
	}
}

type simpleForm struct {
	*forms.Form
}

func newSimpleForm() *simpleForm {
	return &simpleForm{forms.Must(
		context.Background(),
		forms.NewTextField("name"),
		forms.NewIntegerField("id", forms.Required),
	)}
}

func (f *simpleForm) Validate() {
	if f.Context().Value(ctxSimpleKey{}) != nil {
		f.AddErrors("", errors.New("simple context error"))
	}
}

type l10nForm struct {
	*forms.Form
}

func (f *l10nForm) Validate() {
	f.AddErrors("", forms.Gettext("global error"))
}

func newL10nForm() *l10nForm {
	return &l10nForm{forms.Must(
		forms.WithTranslator(context.Background(), prefixTranslator("prefix")),
		forms.NewIntegerField("id", forms.Required),
	)}
}

type defaultsForm struct {
	*forms.Form
}

func newDefaultsForm() *defaultsForm {
	dt, _ := time.Parse(time.DateTime, "2024-01-04 12:35:02")
	f := &defaultsForm{forms.Must(
		context.Background(),
		forms.NewBooleanField("bool", forms.Default(true)),
		forms.NewTextField("text", forms.Default("abc")),
		forms.NewIntegerField("int", forms.Default(123)),
		forms.NewDatetimeField("time", forms.Default(dt)),
	)}

	return f
}

func TestNewForm(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		assert := require.New(t)

		f, err := forms.New(
			context.Background(),
			forms.NewTextField(""),
		)
		assert.Nil(f)
		assert.Errorf(err, "unamed field")

		f, err = forms.New(
			context.Background(),
			forms.NewTextField("name"),
			forms.NewTextField("name"),
		)
		assert.Nil(f)
		assert.Errorf(err, `field "name" already defined`)

		assert.Panics(func() {
			forms.Must(
				context.Background(),
				forms.NewTextField(""),
			)
		})

		assert.Panics(func() {
			forms.Must(
				context.Background(),
				forms.NewTextField("name"),
				forms.NewTextField("name"),
			)
		})
	})

	t.Run("form context", func(t *testing.T) {
		f := newSimpleForm()
		f.Get("name").Set("alice")
		assert := require.New(t)
		assert.Exactly(f.Form, forms.GetForm(f.Get("id")))

		field := forms.NewTextField("")
		assert.Nil(forms.GetForm(field))
	})
}

func TestValuePriority(t *testing.T) {
	tests := []struct {
		method string
		url    string
		body   url.Values
		expect any
	}{
		{
			http.MethodGet,
			"/?name=abc",
			nil,
			"abc",
		},
		{
			http.MethodGet,
			"/?name=abc",
			url.Values{},
			"abc",
		},
		{
			http.MethodPost,
			"/?name=abc",
			url.Values{"name": []string{"xyz"}},
			"xyz",
		},
		{
			http.MethodPost,
			"/",
			url.Values{"name": []string{"xyz"}},
			"xyz",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			f := forms.Must(
				context.Background(),
				forms.NewTextField("name"),
			)

			body := new(bytes.Buffer)
			if test.body != nil {
				body.WriteString(test.body.Encode())
			}

			r, _ := http.NewRequest(test.method, test.url, body)
			r.Header.Set("Content-Type", string(mimeValues))

			forms.Bind(f, r)

			assert := require.New(t)
			assert.True(f.IsBound())
			assert.Equal(test.expect, f.Get("name").Value())
		})
	}
}

func TestBindURL(t *testing.T) {
	tests := []struct {
		url    string
		expect any
	}{
		{
			"/?name=abc",
			"abc",
		},
		{
			"/",
			nil,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			f := forms.Must(
				context.Background(),
				forms.NewTextField("name"),
			)

			r, _ := http.NewRequest(http.MethodGet, test.url, nil)

			forms.BindURL(f, r)

			assert := require.New(t)
			assert.True(f.IsBound())
			assert.Equal(test.expect, f.Get("name").Value())
		})
	}
}

func TestSimpleForm(t *testing.T) {
	t.Run("json", runRequestForm(mimeJSON, func() forms.Binder {
		return newSimpleForm()
	}, []formTest{
		{
			`{"name": "test", "id": 2}`,
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"id": {
						"is_null": false,
						"is_bound": true,
						"value": 2,
						"errors": null
					},
					"name": {
						"is_null": false,
						"is_bound": true,
						"value": "test",
						"errors": null
					}
				}
			}`,
		},
		{
			"",
			`{
				"is_valid": false,
				"errors": [
					"invalid input data"
				],
				"fields": {
					"id": {
						"is_null": true,
						"is_bound": false,
						"value": 0,
						"errors": ["field is required"]
					},
					"name": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": null
					}
				}
			}`,
		},
		{
			`{"name": 123}`,
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
					"id": {
						"is_null": true,
						"is_bound": false,
						"value": 0,
						"errors": [
							"field is required"
						]
					},
					"name": {
						"is_null": false,
						"is_bound": false,
						"value": "",
						"errors": [
							"invalid value"
						]
					}
				}
			}`,
		},
	}))

	t.Run("form values", runRequestForm(mimeValues, func() forms.Binder {
		return newSimpleForm()
	}, []formTest{
		{
			`name=test&name=alice&id=2`,
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"id": {
						"is_null": false,
						"is_bound": true,
						"value": 2,
						"errors": null
					},
					"name": {
						"is_null": false,
						"is_bound": true,
						"value": "test",
						"errors": null
					}
				}
			}`,
		},
		{
			"",
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
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
						"errors": null
					}
				}
			}`,
		},
		{
			`name=123`,
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
					"id": {
						"is_null": true,
						"is_bound": false,
						"value": 0,
						"errors": [
							"field is required"
						]
					},
					"name": {
						"is_null": false,
						"is_bound": true,
						"value": "123",
						"errors": null
					}
				}
			}`,
		},
	}))

	t.Run("multipart values", runMultipartForm(func() forms.Binder {
		return newSimpleForm()
	}, []multipartTest{
		{
			func(_ *multipart.Writer) error { return nil },
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
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
						"errors": null
					}
				}
			}`,
		},
		{
			func(mp *multipart.Writer) error {
				_ = mp.WriteField("name", "alice")
				_ = mp.WriteField("name", "test")
				_ = mp.WriteField("id", "2")
				return nil
			},
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"id": {
						"is_null": false,
						"is_bound": true,
						"value": 2,
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

	t.Run("l10n", runRequestForm(mimeJSON, func() forms.Binder {
		return newL10nForm()
	}, []formTest{
		{
			`{"name": 123}`,
			`{
				"is_valid": false,
				"errors": [
					"prefix:global error"
				],
				"fields": {
					"id": {
						"is_null": true,
						"is_bound": false,
						"value": 0,
						"errors": [
							"prefix:field is required"
						]
					}
				}
			}`,
		},
	}))
}

func TestDefaultValues(t *testing.T) {
	t.Run("json", runRequestForm(mimeJSON, func() forms.Binder {
		return newDefaultsForm()
	}, []formTest{
		{
			`{}`,
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"bool": {
						"is_null": false,
						"is_bound": false,
						"value": true,
						"errors": null
					},
					"int": {
						"is_null": false,
						"is_bound": false,
						"value": 123,
						"errors": null
					},
					"text": {
						"is_null": false,
						"is_bound": false,
						"value": "abc",
						"errors": null
					},
					"time": {
						"is_null": false,
						"is_bound": false,
						"value": "2024-01-04T12:35:02Z",
						"errors": null
					}
				}
			}`,
		},
		{
			`{
					"bool": false,
					"int": 5,
					"text": "xyz",
					"time": "2024-02-05 11:23:45"
				}`,
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"bool": {
						"is_null": false,
						"is_bound": true,
						"value": false,
						"errors": null
					},
					"int": {
						"is_null": false,
						"is_bound": true,
						"value": 5,
						"errors": null
					},
					"text": {
						"is_null": false,
						"is_bound": true,
						"value": "xyz",
						"errors": null
					},
					"time": {
						"is_null": false,
						"is_bound": true,
						"value": "2024-02-05T11:23:45Z",
						"errors": null
					}
				}
			}`,
		},
		{
			`{
					"bool": 12,
					"int": true,
					"text": 55,
					"time": "abc"
				}`,
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
					"bool": {
						"is_null": false,
						"is_bound": false,
						"value": false,
						"errors": [
							"invalid value"
						]
					},
					"int": {
						"is_null": false,
						"is_bound": false,
						"value": 0,
						"errors": [
							"invalid value"
						]
					},
					"text": {
						"is_null": false,
						"is_bound": false,
						"value": "",
						"errors": [
							"invalid value"
						]
					},
					"time": {
						"is_null": false,
						"is_bound": false,
						"value": "0001-01-01T00:00:00Z",
						"errors": [
							"invalid value"
						]
					}
				}
			}`,
		},
	}))

	t.Run("form values", runRequestForm(mimeValues, func() forms.Binder {
		return newDefaultsForm()
	}, []formTest{
		{
			``,
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"bool": {
						"is_null": false,
						"is_bound": false,
						"value": true,
						"errors": null
					},
					"int": {
						"is_null": false,
						"is_bound": false,
						"value": 123,
						"errors": null
					},
					"text": {
						"is_null": false,
						"is_bound": false,
						"value": "abc",
						"errors": null
					},
					"time": {
						"is_null": false,
						"is_bound": false,
						"value": "2024-01-04T12:35:02Z",
						"errors": null
					}
				}
			}`,
		},
		{
			"bool=f&int=5&text=xyz&time=2024-02-05%2011:23:45",
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"bool": {
						"is_null": false,
						"is_bound": true,
						"value": false,
						"errors": null
					},
					"int": {
						"is_null": false,
						"is_bound": true,
						"value": 5,
						"errors": null
					},
					"text": {
						"is_null": false,
						"is_bound": true,
						"value": "xyz",
						"errors": null
					},
					"time": {
						"is_null": false,
						"is_bound": true,
						"value": "2024-02-05T11:23:45Z",
						"errors": null
					}
				}
			}`,
		},
		{
			"bool=12&int=true&text=55&time=abc",
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
					"bool": {
						"is_null": false,
						"is_bound": false,
						"value": false,
						"errors": [
							"invalid value"
						]
					},
					"int": {
						"is_null": false,
						"is_bound": false,
						"value": 0,
						"errors": [
							"invalid value"
						]
					},
					"text": {
						"is_null": false,
						"is_bound": true,
						"value": "55",
						"errors": null
					},
					"time": {
						"is_null": false,
						"is_bound": false,
						"value": "0001-01-01T00:00:00Z",
						"errors": [
							"invalid value"
						]
					}
				}
			}`,
		},
	}))
}

type complexForm struct {
	*forms.Form
}

func newComplexForm(tr forms.Translator) *complexForm {
	return &complexForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("name",
			forms.Trim, forms.Required,
			forms.ValueValidatorFunc[string](func(_ forms.Field, v string) error {
				if strings.Contains(v, "xx") {
					return forms.Gettext("value contains xx")
				}
				return nil
			}),
		),
		forms.NewTextField("group", forms.Trim, forms.Choices(
			forms.Choice("User", "user"),
			forms.Choice("Admin", "admin"),
		), forms.Default("user")),
		forms.NewTextListField("acls", forms.Trim, forms.Choices(
			forms.Choice("Read", "r"),
			forms.Choice("Write", "w"),
		)),
	)}
}

func TestComplexForm(t *testing.T) {
	runRequestForm("application/json", func() forms.Binder {
		return newComplexForm(prefixTranslator("E"))
	}, []formTest{
		{
			`{}`,
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
					"acls": {
						"is_null": true,
						"is_bound": false,
						"value": null,
						"errors": null
					},
					"group": {
						"is_null": false,
						"is_bound": false,
						"value": "user",
						"errors": null
					},
					"name": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": [
							"E:field is required"
						]
					}
				}
			}`,
		},
		{
			`{"name": "alice"}`,
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"acls": {
						"is_null": true,
						"is_bound": false,
						"value": null,
						"errors": null
					},
					"group": {
						"is_null": false,
						"is_bound": false,
						"value": "user",
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
		{
			`{"name": "alice", "group": null}`,
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"acls": {
						"is_null": true,
						"is_bound": false,
						"value": null,
						"errors": null
					},
					"group": {
						"is_null": true,
						"is_bound": true,
						"value": "",
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
		{
			`{"name": "alice", "group": "admin", "acls": ["r", "w"]}`,
			`{
				"is_valid": true,
				"errors": null,
				"fields": {
					"acls": {
						"is_null": false,
						"is_bound": true,
						"value": ["r", "w"],
						"errors": null
					},
					"group": {
						"is_null": false,
						"is_bound": true,
						"value": "admin",
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
		{
			`{"name": "alixxce", "group": "admin"}`,
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
					"acls": {
						"is_null": true,
						"is_bound": false,
						"value": null,
						"errors": null
					},
					"group": {
						"is_null": false,
						"is_bound": true,
						"value": "admin",
						"errors": null
					},
					"name": {
						"is_null": false,
						"is_bound": true,
						"value": "alixxce",
						"errors": [
							"E:value contains xx"
						]
					}
				}
			}`,
		},
		{
			`{"name": "alixxce", "group": "foo", "acls": ["r", "g"]}`,
			`{
				"is_valid": false,
				"errors": null,
				"fields": {
					"acls": {
						"is_null": false,
						"is_bound": true,
						"value": ["r", "g"],
						"errors": [
							"E:g is not one of r, w"
						]
					},
					"group": {
						"is_null": false,
						"is_bound": true,
						"value": "foo",
						"errors": [
							"E:foo is not one of user, admin"
						]
					},
					"name": {
						"is_null": false,
						"is_bound": true,
						"value": "alixxce",
						"errors": [
							"E:value contains xx"
						]
					}
				}
			}`,
		},
	})(t)
}

func TestContextForm(t *testing.T) {
	type ctxKeyUser struct{}

	t.Run("alice",
		runRequestForm(mimeValues, func() forms.Binder {
			ctx := context.WithValue(context.Background(), ctxKeyUser{}, "alice")
			return forms.Must(
				forms.WithTranslator(ctx, prefixTranslator("E")),
				forms.NewTextField("group",
					forms.Default("user"),
					forms.Required,
					forms.ValueValidatorFunc[string](func(f forms.Field, _ string) error {
						username := f.Context().Value(ctxKeyUser{}).(string)
						if username != "alice" {
							return forms.FatalError(errors.New("forbidden"))
						}
						return nil
					}),
				),
			)
		}, []formTest{
			{
				"",
				`{
					"is_valid": false,
					"errors": null,
					"fields": {
						"group": {
							"is_null": false,
							"is_bound": false,
							"value": "user",
							"errors": [
								"E:field is required"
							]
						}
					}
				}`,
			},
			{
				"group=",
				`{
					"is_valid": false,
					"errors": null,
					"fields": {
						"group": {
							"is_null": false,
							"is_bound": true,
							"value": "",
							"errors": [
								"E:field is required"
							]
						}
					}
				}`,
			},
			{
				"group=admin",
				`{
					"is_valid": true,
					"errors": null,
					"fields": {
						"group": {
							"is_null": false,
							"is_bound": true,
							"value": "admin",
							"errors": null
						}
					}
				}`,
			},
		}),
	)

	t.Run("bob",
		runRequestForm(mimeValues, func() forms.Binder {
			ctx := context.WithValue(context.Background(), ctxKeyUser{}, "bob")
			return forms.Must(
				forms.WithTranslator(ctx, prefixTranslator("E")),
				forms.NewTextField("group",
					forms.Default("user"),
					forms.Required,
					forms.ValueValidatorFunc[string](func(f forms.Field, _ string) error {
						username := f.Context().Value(ctxKeyUser{}).(string)
						if username != "alice" {
							return forms.FatalError(forms.Gettext("forbidden"))
						}
						return nil
					}),
				),
			)
		}, []formTest{
			{
				"group=admin",
				`{
					"is_valid": false,
					"errors": null,
					"fields": {
						"group": {
							"is_null": false,
							"is_bound": true,
							"value": "admin",
							"errors": [
								"E:forbidden"
							]
						}
					}
				}`,
			},
		}),
	)
}
