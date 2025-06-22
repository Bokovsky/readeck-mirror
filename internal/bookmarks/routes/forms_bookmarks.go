// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	goquexp "github.com/doug-martin/goqu/v9/exp"
	"github.com/wneessen/go-mail"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/converter"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/db/exp"
	"codeberg.org/readeck/readeck/internal/email"
	"codeberg.org/readeck/readeck/internal/searchstring"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/locales"
	"codeberg.org/readeck/readeck/pkg/forms"
	"codeberg.org/readeck/readeck/pkg/timetoken"
	"codeberg.org/readeck/readeck/pkg/utils"
)

var validSchemes = []string{"http", "https"}

const (
	filtersTitleUnset = iota
	filtersTitleUnread
	filtersTitleArchived
	filtersTitleFavorites
	filtersTitleArticles
	filtersTitleVideos
	filtersTitlePictures
)

const (
	filtersReadStatusUnread  = "unread"
	filtersReadStatusReading = "reading"
	filtersReadStatusRead    = "read"
)

type orderExpressionList []goquexp.OrderedExpression

type createForm struct {
	*forms.Form
	userID    int
	requestID string
	resources []tasks.MultipartResource
}

func newCreateForm(tr forms.Translator, userID int, requestID string) *createForm {
	return &createForm{
		Form: forms.Must(
			forms.WithTranslator(context.Background(), tr),
			forms.NewTextField("url",
				forms.Trim,
				forms.Required,
				forms.IsURL(validSchemes...),
			),
			forms.NewTextField("title", forms.Trim),
			forms.NewTextListField("labels", forms.Trim, forms.DiscardEmpty),
			forms.NewBooleanField("feature_find_main"),
			forms.NewFileListField("resource"),
		),
		userID:    userID,
		requestID: requestID,
	}
}

// newMultipartResource returns a new instance of multipartResource from
// a [forms.FileOpener]. The input MUST contain a JSON payload on the first line
// (with the url and headers) and the data on the remaining lines.
func (f *createForm) newMultipartResource(opener forms.FileOpener) (res tasks.MultipartResource, err error) {
	var r io.ReadCloser
	r, err = opener.Open()
	if err != nil {
		return
	}
	defer r.Close() // nolint:errcheck

	const bufSize = 256 << 10 // In KiB
	bio := bufio.NewReaderSize(r, bufSize)

	// Read the first line containing the JSON metadata
	var line []byte
	if line, err = bio.ReadBytes('\n'); err != nil {
		return
	}
	if err = json.Unmarshal(line, &res); err != nil {
		return
	}

	if res.URL == "" {
		err = errors.New("No resource URL")
		return
	}

	// Read the rest (the content)
	res.Data, err = io.ReadAll(bio)
	if err != nil {
		return
	}
	if len(res.Data) == 0 {
		err = errors.New("No resource content")
		return
	}

	return
}

func (f *createForm) Validate() {
	if !f.IsValid() {
		return
	}

	// Load all the resources passed in the "resource" field.
	for _, opener := range f.Get("resource").(forms.TypedField[[]forms.FileOpener]).V() {
		resource, err := f.newMultipartResource(opener)
		if err != nil {
			if f.Get("url").String() != resource.URL {
				// As long as the content is not from the requested URL
				// we can ignore an empty value.
				continue
			}
			f.AddErrors("", forms.Gettext("Unable to process input data"))
			break
		}
		f.resources = append(f.resources, resource)
	}
}

func (f *createForm) createBookmark() (b *bookmarks.Bookmark, err error) {
	if !f.IsBound() {
		return nil, errors.New("form is not bound")
	}

	uri, _ := url.Parse(f.Get("url").String())
	uri.Fragment = ""

	b = &bookmarks.Bookmark{
		UserID:   &f.userID,
		State:    bookmarks.StateLoading,
		URL:      uri.String(),
		Title:    f.Get("title").String(),
		Site:     uri.Hostname(),
		SiteName: uri.Hostname(),
	}

	if !f.Get("labels").IsNil() {
		b.Labels = f.Get("labels").(forms.TypedField[[]string]).V()
		slices.Sort(b.Labels)
		b.Labels = slices.Compact(b.Labels)
	}

	defer func() {
		if err != nil {
			f.AddErrors("", forms.ErrUnexpected)
		}
	}()

	if err = bookmarks.Bookmarks.Create(b); err != nil {
		return
	}

	// Start extraction job
	err = tasks.ExtractPageTask.Run(b.ID, tasks.ExtractParams{
		BookmarkID: b.ID,
		RequestID:  f.requestID,
		Resources:  f.resources,
		FindMain:   f.Get("feature_find_main").IsNil() || f.Get("feature_find_main").Value().(bool),
	})
	return
}

type updateForm struct {
	*forms.Form
}

func newUpdateForm(tr forms.Translator) *updateForm {
	return &updateForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("title", forms.Trim),
		forms.NewBooleanField("is_marked"),
		forms.NewBooleanField("is_archived"),
		forms.NewBooleanField("is_deleted"),
		forms.NewIntegerField("read_progress", forms.Gte(0), forms.Lte(100)),
		forms.NewTextField("read_anchor", forms.Trim),
		forms.NewTextListField("labels", forms.Trim, forms.DiscardEmpty),
		forms.NewTextListField("add_labels", forms.Trim, forms.DiscardEmpty),
		forms.NewTextListField("remove_labels", forms.Trim, forms.DiscardEmpty),
		forms.NewTextField("_to", forms.Trim),
	)}
}

func (f *updateForm) update(b *bookmarks.Bookmark) (updated map[string]interface{}, err error) {
	updated = map[string]interface{}{}
	var deleted *bool
	labelsChanged := false

	for _, field := range f.Fields() {
		if !field.IsBound() || field.IsNil() {
			continue
		}
		switch n := field.Name(); n {
		case "title":
			if field.String() != "" {
				b.Title = utils.NormalizeSpaces(field.String())
				updated[n] = field.String()
			}
		case "is_marked":
			b.IsMarked = field.(forms.TypedField[bool]).V()
			updated[n] = field.Value()
		case "is_archived":
			b.IsArchived = field.(forms.TypedField[bool]).V()
			updated[n] = field.Value()
		case "is_deleted":
			deleted = new(bool)
			*deleted = field.(forms.TypedField[bool]).V()
		case "read_progress":
			b.ReadProgress = field.(forms.TypedField[int]).V()
			updated[n] = field.Value()
		case "read_anchor":
			b.ReadAnchor = field.String()
			updated[n] = field.Value()
		// labels, add_labels and remove_labels are declared and
		// processed in this order.
		case "labels":
			b.Labels = field.(forms.TypedField[[]string]).V()
			labelsChanged = true
		case "add_labels":
			b.Labels = append(b.Labels, field.(forms.TypedField[[]string]).V()...)
			labelsChanged = true
		case "remove_labels":
			b.Labels = slices.DeleteFunc(b.Labels, func(s string) bool {
				return slices.Contains(field.(forms.TypedField[[]string]).V(), s)
			})
			labelsChanged = true
		}
	}

	if labelsChanged {
		slices.SortFunc(b.Labels, exp.UnaccentCompare)
		b.Labels = slices.Compact(b.Labels)
		updated["labels"] = b.Labels
	}

	if _, ok := updated["read_progress"]; ok {
		if b.ReadProgress == 0 || b.ReadProgress == 100 {
			b.ReadAnchor = ""
			updated["read_anchor"] = ""
		}
	}

	defer func() {
		updated["id"] = b.UID
		if err != nil {
			f.AddErrors("", forms.ErrUnexpected)
		}
	}()

	if len(updated) > 0 || deleted != nil {
		updated["updated"] = time.Now()
		if err = b.Update(updated); err != nil {
			return
		}

	}

	if deleted != nil {
		updated["is_deleted"] = *deleted
		df := newDeleteForm(nil)
		df.Get("cancel").Set(!*deleted)
		err = df.trigger(b)
	}

	return
}

type deleteForm struct {
	*forms.Form
}

func newDeleteForm(tr forms.Translator) *deleteForm {
	return &deleteForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewBooleanField("cancel"),
		forms.NewTextField("_to", forms.Trim),
	)}
}

// trigger launch the user deletion or cancel task.
func (f *deleteForm) trigger(b *bookmarks.Bookmark) error {
	if !f.Get("cancel").IsNil() && f.Get("cancel").Value().(bool) {
		return tasks.DeleteBookmarkTask.Cancel(b.ID)
	}

	return tasks.DeleteBookmarkTask.Run(b.ID, b.ID)
}

type labelForm struct {
	*forms.Form
}

func newLabelForm(tr forms.Translator) *labelForm {
	return &labelForm{
		Form: forms.Must(
			forms.WithTranslator(context.Background(), tr),
			forms.NewTextField("name", forms.Trim, forms.Required),
		),
	}
}

type labelSearchForm struct {
	*forms.Form
}

func newLabelSearchForm(tr forms.Translator) *labelSearchForm {
	return &labelSearchForm{forms.Must(
		forms.WithTranslator(context.Background(), tr),
		forms.NewTextField("q", forms.Trim, forms.RequiredOrNil),
	)}
}

type labelDeleteForm struct {
	*forms.Form
}

func newLabelDeleteForm(tr forms.Translator) *labelDeleteForm {
	return &labelDeleteForm{
		forms.Must(
			forms.WithTranslator(context.Background(), tr),
			forms.NewBooleanField("cancel"),
		),
	}
}

func (f *labelDeleteForm) trigger(user *users.User, name string) error {
	id := fmt.Sprintf("%d@%s", user.ID, name)

	if !f.Get("cancel").IsNil() && f.Get("cancel").(forms.TypedField[bool]).V() {
		return tasks.DeleteLabelTask.Cancel(id)
	}

	return tasks.DeleteLabelTask.Run(id, tasks.LabelDeleteParams{
		UserID: user.ID, Name: name,
	})
}

type filterForm struct {
	*forms.Form
	title        int
	noPagination bool
	fixedLimit   uint
	sq           searchstring.SearchQuery
}

func newFilterForm(tr forms.Translator) *filterForm {
	return &filterForm{
		Form: forms.Must(
			forms.WithTranslator(context.Background(), tr),
			forms.NewBooleanField("bf"),
			forms.NewTextField("search", forms.Trim),
			forms.NewTextField("title", forms.Trim),
			forms.NewTextField("author", forms.Trim),
			forms.NewTextField("site", forms.Trim),
			forms.NewTextListField("type", forms.Choices(
				forms.Choice(tr.Gettext("Article"), "article"),
				forms.Choice(tr.Gettext("Picture"), "photo"),
				forms.Choice(tr.Gettext("Video"), "video"),
			), forms.Trim),
			forms.NewBooleanField("is_loaded"),
			forms.NewBooleanField("has_errors"),
			forms.NewBooleanField("has_labels"),
			forms.NewTextField("labels", forms.Trim),
			forms.NewTextListField("read_status", forms.Choices(
				forms.Choice(tr.Pgettext("status", "Unviewed"), filtersReadStatusUnread),
				forms.Choice(tr.Pgettext("status", "In-Progress"), filtersReadStatusReading),
				forms.Choice(tr.Pgettext("status", "Completed"), filtersReadStatusRead),
			), forms.Trim),
			forms.NewBooleanField("is_marked"),
			forms.NewBooleanField("is_archived"),
			forms.NewTextField("range_start", forms.Trim, validateTimeToken),
			forms.NewTextField("range_end", forms.Trim, validateTimeToken),
			forms.NewTextListField("id", forms.Trim),
		),
		title: filtersTitleUnset,
	}
}

// newContextFilterForm returns an instance of filterForm. If one already
// exists in the given context, it's reused, otherwise it returns a new one.
func newContextFilterForm(c context.Context, tr forms.Translator) *filterForm {
	ff, ok := checkFilterForm(c)
	if !ok {
		ff = newFilterForm(tr)
	}

	return ff
}

func (f *filterForm) Validate() {
	// First, we must build a search string based on
	// the provided free form search and
	// what we might have in the following fields:
	// title, author, site, label
	f.sq = searchstring.ParseQuery(f.Get("search").String())

	for _, field := range f.Fields() {
		var fname string
		switch n := field.Name(); n {
		case "title", "author", "site":
			fname = n
		case "labels":
			fname = "label"
		}

		if fname == "" || field.String() == "" {
			continue
		}

		q := searchstring.ParseField(field.String(), fname)
		f.sq.Terms = append(f.sq.Terms, q.Terms...)
	}

	// Remove duplicates from the query
	f.sq = f.sq.Dedup()

	// Remove field definition for unallowed fields
	f.sq = f.sq.Unfield("title", "author", "site", "label")

	// Update the specific search fields
	for _, field := range f.Fields() {
		fname := "-"
		switch n := field.Name(); n {
		case "search":
			fname = ""
		case "title", "author", "site":
			fname = n
		case "labels":
			fname = "label"
		}

		if fname == "-" {
			continue
		}
		v := f.sq.ExtractField(fname).RemoveFieldInfo().String()
		if v != field.String() {
			_ = field.UnmarshalValues([]string{v})
		}
	}
}

func (f *filterForm) IsActive() bool {
	if v, ok := f.Get("bf").Value().(bool); ok {
		return v
	}
	return false
}

func (f *filterForm) GetQueryString() string {
	q := url.Values{}
	for _, field := range f.Fields() {
		if field.IsNil() {
			continue
		}
		switch n := field.Name(); n {
		case "type", "read_status":
			for _, s := range field.Value().([]string) {
				q.Add(n, s)
			}
		default:
			q.Add(n, field.String())
		}
	}

	return q.Encode()
}

// setMarked sets the IsMarked property.
func (f *filterForm) setMarked(v bool) {
	f.Get("is_marked").Set(v)
	f.title = filtersTitleFavorites
}

// setArchived sets the IsArchived property.
func (f *filterForm) setArchived(v bool) {
	f.Get("is_archived").Set(v)
	if v {
		f.title = filtersTitleArchived
	} else {
		f.title = filtersTitleUnread
	}
}

func (f *filterForm) setType(v string) {
	f.Get("type").Set([]string{v})
	switch v {
	case "article":
		f.title = filtersTitleArticles
	case "photo":
		f.title = filtersTitlePictures
	case "video":
		f.title = filtersTitleVideos
	}
}

type orderForm struct {
	*forms.Form
	fieldName string
	choices   map[string]goquexp.Orderable
}

func newOrderForm(fieldName string, choices map[string]goquexp.Orderable) *orderForm {
	// Compile a list of choices being pairs of "A" and "-A", "B", "-B",
	fieldChoices := make(forms.ValueChoices[string], len(choices)*2)
	for k := range choices {
		fieldChoices = append(fieldChoices, forms.Choice("", k), forms.Choice("", "-"+k))
	}

	return &orderForm{
		Form: forms.Must(
			context.Background(),
			forms.NewTextListField(fieldName, forms.Trim, forms.Choices(fieldChoices...)),
		),
		fieldName: fieldName,
		choices:   choices,
	}
}

func (f *orderForm) toOrderedExpressions() orderExpressionList {
	if !f.IsBound() || !f.IsValid() {
		return nil
	}
	field := f.Get(f.fieldName)
	values := field.(forms.TypedField[[]string]).V()
	if len(values) == 0 {
		return nil
	}

	res := orderExpressionList{}
	for _, x := range values {
		identifier := f.choices[strings.TrimPrefix(x, "-")]
		if identifier == nil {
			continue
		}
		if strings.HasPrefix(x, "-") {
			res = append(res, identifier.Desc())
			continue
		}
		res = append(res, identifier.Asc())
	}

	return res
}

func (f *orderForm) value() []string {
	if !f.IsBound() || !f.IsValid() {
		return nil
	}

	return f.Get(f.fieldName).(forms.TypedField[[]string]).V()
}

type bookmarkOrderForm struct {
	*orderForm
}

func newBookmarkOrderForm() *bookmarkOrderForm {
	t := goqu.T("b")

	return &bookmarkOrderForm{
		orderForm: newOrderForm("sort", map[string]goquexp.Orderable{
			"created":   t.Col("created"),
			"domain":    t.Col("domain"),
			"duration":  goqu.Case().When(goqu.L("? > 0", t.Col("duration")), t.Col("duration")).Else(goqu.L("? * 0.3", t.Col("word_count"))),
			"published": goqu.Case().When(t.Col("published").IsNot(nil), t.Col("published")).Else(t.Col("created")),
			"site":      t.Col("site_name"),
			"title":     t.Col("title"),
		}),
	}
}

func (f *bookmarkOrderForm) addToTemplateContext(r *http.Request, tr *locales.Locale, c server.TC) {
	if v := f.value(); len(v) > 0 {
		c["CurrentOrder"] = v[0]
	} else {
		c["CurrentOrder"] = "-created"
	}

	qs := url.Values{}
	for k, v := range r.URL.Query() {
		if k == "sort" {
			continue
		}
		qs[k] = v
	}

	setOption := func(name, label string) [3]string {
		qs["sort"] = []string{name}
		defer delete(qs, "sort")
		return [3]string{name, r.URL.Path + "?" + qs.Encode(), label}
	}

	c["OrderOptions"] = [][3]string{
		setOption("-created", tr.Pgettext("sort", "Added, most recent first")),
		setOption("created", tr.Pgettext("sort", "Added, oldest first")),
		setOption("-published", tr.Pgettext("sort", "Published, most recent first")),
		setOption("published", tr.Pgettext("sort", "Published, oldest first")),
		setOption("title", tr.Pgettext("sort", "Title, A to Z")),
		setOption("-title", tr.Pgettext("sort", "Title, Z to A")),
		setOption("site", tr.Pgettext("sort", "Site Name, A to Z")),
		setOption("-site", tr.Pgettext("sort", "Site Name, Z to A")),
		setOption("duration", tr.Pgettext("sort", "Duration, shortest first")),
		setOption("-duration", tr.Pgettext("sort", "Duration, longest first")),
	}
}

var validateTimeToken = forms.ValueValidatorFunc[string](func(_ forms.Field, value string) error {
	if value == "" {
		return nil
	}
	if _, err := timetoken.New(value); err != nil {
		return fmt.Errorf(`"%s" is not a valid date value`, value)
	}
	return nil
})

type shareForm struct {
	*forms.Form
}

func newShareForm(tr forms.Translator) *shareForm {
	return &shareForm{
		Form: forms.Must(
			forms.WithTranslator(context.Background(), tr),
			forms.NewTextField("email",
				forms.Trim,
				forms.Required,
				forms.IsEmail,
			),
			forms.NewTextField("format",
				forms.Trim,
				forms.Choices(
					forms.Choice("Article", "html"),
					forms.Choice("E-Book", "epub"),
				),
				forms.Default("html"),
			),
		),
	}
}

func (f *shareForm) sendBookmark(r *http.Request, b *bookmarks.Bookmark) (err error) {
	if !f.IsBound() {
		err = errors.New("form is not bound")
		return
	}

	var exporter converter.Exporter
	var options []email.MessageOption
	if u := auth.GetRequestUser(r); u != nil && u.Settings.EmailSettings.ReplyTo != "" {
		options = []email.MessageOption{
			func(msg *mail.Msg) error {
				return msg.ReplyTo(u.Settings.EmailSettings.ReplyTo)
			},
		}
	}

	switch f.Get("format").String() {
	case "html":
		exporter = converter.NewHTMLEmailExporter(
			f.Get("email").String(),
			options...,
		)
	case "epub":
		exporter = converter.NewEPUBEmailExporter(
			f.Get("email").String(),
			options...,
		)
	}

	if exporter == nil {
		err = errors.New("no exporter")
		f.AddErrors("", forms.ErrUnexpected)
		return
	}

	if err = exporter.Export(
		r.Context(), nil, r,
		&dataset.BookmarkList{
			Count: 1,
			Items: []*dataset.Bookmark{
				dataset.NewBookmark(server.WithRequest(r.Context(), r), b),
			},
		},
	); err != nil {
		f.AddErrors("", forms.ErrUnexpected)
		return
	}

	return
}
