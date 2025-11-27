// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	"errors"
	"net/http"
	"time"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type collectionDeleteForm struct {
	*forms.Form
}

func newCollectionDeleteForm(tr forms.Translator) *collectionDeleteForm {
	return &collectionDeleteForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewBooleanField("cancel"),
		forms.NewTextField("_to"),
	)}
}

func (f *collectionDeleteForm) trigger(c *bookmarks.Collection) error {
	if !f.Get("cancel").IsNil() && f.Get("cancel").Value().(bool) {
		return tasks.DeleteCollectionTask.Cancel(c.ID)
	}

	return tasks.DeleteCollectionTask.Run(c.ID, c.ID)
}

type collectionForm struct {
	*forms.JoinedForms
}

func newCollectionForm(tr forms.Translator, r *http.Request) *collectionForm {
	return &collectionForm{forms.Join(
		forms.WithTranslator(context.Background(), tr),
		newFilterForm(tr),
		forms.Must(
			forms.WithTranslator(context.Background(), tr),
			forms.NewTextField("name", forms.Trim, forms.FieldValidatorFunc(func(f forms.Field) error {
				switch r.Method {
				case http.MethodPatch:
					return forms.RequiredOrNil(f)
				case http.MethodPost:
					return forms.Required(f)
				}
				return nil
			})),
			forms.NewBooleanField("is_pinned"),
		),
	)}
}

func (f *collectionForm) setFilters(filters *filterForm) {
	for name, field := range filters.Fields() {
		field.Set(f.Get(name).Value())
	}
}

func (f *collectionForm) setCollection(c *bookmarks.Collection) {
	// Regular values
	f.Get("name").Set(c.Name)
	f.Get("is_pinned").Set(c.IsPinned)

	c.Filters.UpdateForm(f)
}

func (f *collectionForm) createCollection(userID int) (*bookmarks.Collection, error) {
	var err error
	defer func() {
		if err != nil {
			f.AddErrors("", forms.ErrUnexpected)
		}
	}()

	if !f.IsBound() {
		return nil, errors.New("form is not bound")
	}

	c := &bookmarks.Collection{
		UserID:  &userID,
		Name:    f.Get("name").String(),
		Filters: bookmarks.NewFiltersFromForm(f),
	}

	err = bookmarks.Collections.Create(c)
	return c, err
}

func (f *collectionForm) updateCollection(c *bookmarks.Collection) (res map[string]interface{}, err error) {
	if !f.IsBound() {
		err = errors.New("form is not bound")
		return
	}

	res = map[string]any{}
	updateMap := map[string]any{}

	needsFilters := false
	for name, field := range f.Fields() {
		switch name {
		case "name", "is_pinned":
			if field.IsBound() {
				res[name] = field.Value()
				updateMap[name] = field.Value()
			}
		default:
			if field.IsBound() {
				res[name] = field.Value()
				needsFilters = true
			}
		}
	}

	if needsFilters {
		updateMap["filters"] = bookmarks.NewFiltersFromForm(f)
	}

	if len(res) > 0 {
		res["updated"] = time.Now().UTC()
		updateMap["updated"] = res["updated"]
		if err = c.Update(updateMap); err != nil {
			f.AddErrors("", forms.ErrUnexpected)
			return
		}
	}
	res["id"] = c.UID
	return
}
