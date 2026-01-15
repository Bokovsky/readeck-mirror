// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"time"

	"github.com/doug-martin/goqu/v9"
	goquexp "github.com/doug-martin/goqu/v9/exp"

	"codeberg.org/readeck/readeck/internal/db/exp"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/internal/searchstring"
	"codeberg.org/readeck/readeck/pkg/forms"
	"codeberg.org/readeck/readeck/pkg/timetoken"
)

const (
	filtersReadStatusUnread  = "unread"
	filtersReadStatusReading = "reading"
	filtersReadStatusRead    = "read"
)

// Filters is the filter list shared between the filter form
// and collection data.
type Filters struct {
	sq searchstring.SearchQuery

	Search     string        `json:"search"`
	Title      string        `json:"title"`
	Author     string        `json:"author"`
	Site       string        `json:"site"`
	Type       types.Strings `json:"type"`
	Labels     string        `json:"labels"`
	ReadStatus types.Strings `json:"read_status"`
	IsMarked   *bool         `json:"is_marked"`
	IsArchived *bool         `json:"is_archived"`
	IsLoaded   *bool         `json:"is_loaded"`
	HasErrors  *bool         `json:"has_errors"`
	HasLabels  *bool         `json:"has_labels"`
	RangeStart string        `json:"range_start"`
	RangeEnd   string        `json:"range_end"`
}

// NewFiltersFromForm return a new [Filters] instance
// populated with the given [forms.Binder] fields.
func NewFiltersFromForm(form forms.Binder) Filters {
	res := Filters{}
	if form != nil {
		(&res).applyForm(form)
		(&res).updateValues()
	}

	return res
}

// Scan loads a [Filters] instance from a column.
func (f *Filters) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	v, err := types.JSONBytes(value)
	if err != nil {
		return err
	}
	json.Unmarshal(v, f) //nolint:errcheck
	return nil
}

// Value encodes a [Filters] value for storage.
func (f Filters) Value() (driver.Value, error) {
	v, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	return string(v), nil
}

func (f *Filters) getFields() map[string]reflect.StructField {
	res := map[string]reflect.StructField{}
	t := reflect.TypeOf(f).Elem()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if tag := sf.Tag.Get("json"); tag != "" {
			res[tag] = sf
		}
	}

	return res
}

func (f *Filters) applyForm(form forms.Binder) {
	sfields := f.getFields()
	rv := reflect.ValueOf(f).Elem()
	for name, field := range form.Fields() {
		sf, ok := sfields[name]
		if !ok {
			continue
		}

		prop := rv.FieldByName(sf.Name)

		switch {
		case sf.Type.Kind() == reflect.Ptr:
			if field.IsNil() {
				prop.SetZero()
			} else {
				prop.Set(reflect.New(sf.Type.Elem()))
				prop.Elem().Set(reflect.ValueOf(field.Value()))
			}
		default:
			if field.IsNil() {
				prop.SetZero()
			} else {
				prop.Set(reflect.ValueOf(field.Value()))
			}
		}
	}
}

// UpdateForm updates the given [forms.Binder]'s fields using the [Filters]
// properties.
func (f Filters) UpdateForm(form forms.Binder) {
	rv := reflect.ValueOf(f)
	for name, sf := range f.getFields() {
		if field := form.Get(name); field != nil {
			_ = rv
			_ = sf
			prop := reflect.Indirect(rv).FieldByName(sf.Name)

			// types of [types.Strings] need to be converted to []string
			// so we can set its value to the field later.
			if prop.Type().ConvertibleTo(reflect.TypeFor[[]string]()) {
				prop = prop.Convert(reflect.TypeFor[[]string]())
			}

			k := sf.Type.Kind()
			switch {
			case k == reflect.Ptr && prop.IsNil():
				// Nil pointer, the field is nil
				field.Set(nil)
			case k == reflect.Slice && prop.IsNil():
				// Empty slice
				field.Set(reflect.New(prop.Type()).Interface())
			case prop.IsZero():
				// Empty value
				field.Set(reflect.New(sf.Type).Interface())
			default:
				// A value with something
				field.Set(prop.Interface())
			}
		}
	}
}

func (f *Filters) updateValues() {
	// First, we must build a search string based on
	// the provided free form search and
	// what we might have in the following fields:
	// title, author, site, label
	f.sq = searchstring.ParseQuery(f.Search)

	setTerms := func(name, value string) {
		if value == "" {
			return
		}
		f.sq.Terms = append(f.sq.Terms, searchstring.ParseField(value, name).Terms...)
	}

	setTerms("title", f.Title)
	setTerms("author", f.Author)
	setTerms("site", f.Site)
	setTerms("label", f.Labels)

	// Remove duplicates from the query
	f.sq = f.sq.Dedup()

	// Remove field definition for unallowed fields
	f.sq = f.sq.Unfield("title", "author", "site", "label")

	// Then, restore the specific properties
	updateValues := func(name string, p *string) {
		v := f.sq.ExtractField(name).RemoveFieldInfo().String()
		if v != *p {
			*p = v
		}
	}

	updateValues("", &f.Search)
	updateValues("title", &f.Title)
	updateValues("author", &f.Author)
	updateValues("site", &f.Site)
	updateValues("label", &f.Labels)
}

// ToSelectDataSet adds the query parameters to the given [*goqu.SelectDataset]
// and returns it.
func (f Filters) ToSelectDataSet(ds *goqu.SelectDataset) *goqu.SelectDataset {
	(&f).updateValues()

	// Separate labels from the final search string
	var labels searchstring.SearchQuery
	var search searchstring.SearchQuery
	if len(f.sq.Terms) > 0 {
		labels, search = f.sq.PopField("label")
		labels = labels.RemoveFieldInfo()
	}

	// Label filter
	if len(labels.Terms) > 0 {
		l := make([]goquexp.BooleanExpression, len(labels.Terms))
		col := goqu.I("b.labels")
		for i, x := range labels.Terms {
			switch {
			case x.Wildcard && x.Exclude:
				l[i] = col.NotLike(x.Value + "%")
			case x.Wildcard:
				l[i] = col.Like(x.Value + "%")
			case x.Exclude:
				l[i] = col.Neq(x.Value)
			default:
				l[i] = col.Eq(x.Value)
			}
		}
		ds = exp.JSONListFilter(ds, l...)
	}

	// Build the search query
	if len(search.Terms) > 0 {
		ds = searchstring.BuildSQL(ds, search, searchConfig[ds.Dialect().Dialect()])
	}
	// Time range
	if f.RangeStart != "" || f.RangeEnd != "" {
		start, _ := timetoken.New(f.RangeStart)
		if f.RangeStart == "" {
			// If start is empty, it's an empty time (0001-01-01 00:00:00)
			start.Absolute = &time.Time{}
		}

		end, _ := timetoken.New(f.RangeEnd)
		ds = ds.Where(exp.DateTime(goqu.C("created")).Between(
			goqu.Range(
				exp.DateTime(start.RelativeTo(nil)),
				exp.DateTime(end.RelativeTo(nil)),
			),
		))
	}

	// read_progress
	if len(f.ReadStatus) > 0 {
		or := goqu.Or()
		c := goqu.C("read_progress").Table("b")
		for _, x := range f.ReadStatus {
			switch x {
			case filtersReadStatusUnread:
				or = or.Append(c.Eq(0))
			case filtersReadStatusReading:
				or = or.Append(c.Between(goqu.Range(1, 99)))
			case filtersReadStatusRead:
				or = or.Append(c.Eq(100))
			}
		}
		ds = ds.Where(or)
	}

	// is_marked
	if f.IsMarked != nil {
		ds = ds.Where(goqu.C("is_marked").Table("b").Eq(*f.IsMarked))
	}

	// is_archived
	if f.IsArchived != nil {
		ds = ds.Where(goqu.C("is_archived").Table("b").Eq(*f.IsArchived))
	}

	// state
	if f.IsLoaded != nil {
		ds = ds.Where(exp.Boolean(
			goqu.C("state").Table("b").Neq(StateLoading),
			*f.IsLoaded,
		))
	}

	// errors
	if f.HasErrors != nil {
		// This one's a bit special. Having errors could be the errors field
		// not being empty or the state being [bookmarks.StateError]
		ds = ds.Where(exp.Boolean(
			goqu.Or(
				goqu.C("state").Table("b").Eq(StateError),
				exp.JSONArrayLength(ds.Dialect(), goqu.C("errors").Table("b")).Gt(0),
			),
			*f.HasErrors,
		))
	}

	// has labels
	if f.HasLabels != nil {
		ds = ds.Where(exp.Boolean(
			exp.JSONArrayLength(ds.Dialect(), goqu.C("labels").Table("b")).Gt(0),
			*f.HasLabels,
		))
	}

	// type
	if len(f.Type) > 0 {
		or := goqu.Or()
		for _, x := range f.Type {
			or = or.Append(goqu.C("type").Table("b").Eq(x))
		}
		ds = ds.Where(or)
	}

	return ds
}

var searchConfig = map[string]*searchstring.BuilderConfig{
	"sqlite3": searchstring.NewBuilderConfig(
		goqu.I("b.id"),
		goqu.I("bookmark_idx.rowid"),
		[][2]string{
			{"", "-catchall"},
			{"title", "title"},
			{"author", "author"},
			{"site", "site"},
			{"label", "label"},
		},
	),
	"postgres": searchstring.NewBuilderConfig(
		goqu.I("b.id"),
		goqu.I("bookmark_search.bookmark_id"),
		[][2]string{
			{"", `bookmark_search.title || bookmark_search.description || bookmark_search."text" || bookmark_search.site || bookmark_search."label"`},
			{"title", "bookmark_search.title"},
			{"author", "bookmark_search.author"},
			{"site", "bookmark_search.site"},
			{"label", "bookmark_search.label"},
		},
	),
}
