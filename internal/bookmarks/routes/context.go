// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/ctxr"
)

// All route related context keys and functions must live here.

// Base template context.
type (
	ctxBaseContextKey struct{}
)

var withBaseContext, getBaseContext, checkBaseContext = ctxr.WithAll[server.TC](ctxBaseContextKey{})

// Bookmark context keys.
type (
	ctxAnnotationListKey     struct{}
	ctxBookmarkKey           struct{}
	ctxBookmarkListDsKey     struct{}
	ctxBookmarkListKey       struct{}
	ctxBookmarkListTaggerKey struct{}
	ctxBookmarkOrderKey      struct{}
	ctxDefaultLimitKey       struct{}
	ctxFiltersKey            struct{}
	ctxLabelKey              struct{}
	ctxLabelListKey          struct{}
	ctxSharedEmailKey        struct{}
	ctxSharedLinkKey         struct{}
)

var (
	withAnnotationList, getAnnotationList                = ctxr.WithGetter[*dataset.AnnotationList](ctxAnnotationListKey{})
	withBookmark, getBookmark, checkBookmark             = ctxr.WithAll[*bookmarks.Bookmark](ctxBookmarkKey{})
	withBookmarkListDS, getBookmarkListDS                = ctxr.WithGetter[*goqu.SelectDataset](ctxBookmarkListDsKey{})
	withBookmarkList, getBookmarkList, checkBookmarkList = ctxr.WithAll[*dataset.BookmarkList](ctxBookmarkListKey{})
	withBookmarkListTaggers, checkBookmarkListTaggers    = ctxr.WithChecker[[]server.Etagger](ctxBookmarkListTaggerKey{})
	withBookmarkOrder, checkBookmarkOrder                = ctxr.WithChecker[orderExpressionList](ctxBookmarkOrderKey{})
	withDefaultLimit, checkDefaultLimit                  = ctxr.WithChecker[int](ctxDefaultLimitKey{})
	withFilterForm, checkFilterForm                      = ctxr.WithChecker[*filterForm](ctxFiltersKey{})
	withLabel, getLabel                                  = ctxr.WithGetter[string](ctxLabelKey{})
	withLabelList, getLabelList                          = ctxr.WithGetter[dataset.LabelList](ctxLabelListKey{})
	withSharedEmail, getSharedEmail                      = ctxr.WithGetter[dataset.SharedEmail](ctxSharedEmailKey{})
	withSharedLink, getSharedLink                        = ctxr.WithGetter[dataset.SharedLink](ctxSharedLinkKey{})
)

// Collection context keys.
type (
	ctxCollectionKey     struct{}
	ctxCollectionListKey struct{}
)

var (
	withCollection, getCollection, checkCollection = ctxr.WithAll[*bookmarks.Collection](ctxCollectionKey{})
	withCollectionList, getCollectionList          = ctxr.WithGetter[*dataset.CollectionList](ctxCollectionListKey{})
)
