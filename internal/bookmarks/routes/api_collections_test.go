// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes_test

import (
	"net/http"
	"testing"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestCollectionAPI(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	client := app.Client(WithToken("user"))

	client.RT(t,
		WithTarget("/api/bookmarks/collections"),
		AssertStatus(200),
		AssertJSON(`[]`),
	)

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/bookmarks/collections"),
		WithBody(map[string]any{}),
		AssertStatus(422),
		AssertJSON(`{
			"is_valid": false,
			"errors": null,
			"fields": {
				"author": {
					"is_null": true,
					"is_bound": false,
					"value": "",
					"errors": null
				},
				"bf": {
					"is_null": true,
					"is_bound": false,
					"value": false,
					"errors": null
				},
				"has_errors": {
					"is_null": true,
					"is_bound": false,
					"value": false,
					"errors": null
				},
				"has_labels": {
					"is_null": true,
					"is_bound": false,
					"value": false,
					"errors": null
				},
				"id": {
					"is_null": true,
					"is_bound": false,
					"value": null,
					"errors": null
				},
				"is_archived": {
					"is_null": true,
					"is_bound": false,
					"value": false,
					"errors": null
				},
				"is_loaded": {
					"is_null": true,
					"is_bound": false,
					"value": false,
					"errors": null
				},
				"is_marked": {
					"is_null": true,
					"is_bound": false,
					"value": false,
					"errors": null
				},
				"is_pinned": {
					"is_null": true,
					"is_bound": false,
					"value": false,
					"errors": null
				},
				"labels": {
					"is_null": true,
					"is_bound": false,
					"value": "",
					"errors": null
				},
				"name": {
					"is_null": true,
					"is_bound": false,
					"value": "",
					"errors": [
						"field is required"
					]
				},
				"search": {
					"is_null": true,
					"is_bound": false,
					"value": "",
					"errors": null
				},
				"site": {
					"is_null": true,
					"is_bound": false,
					"value": "",
					"errors": null
				},
				"range_end": {
					"is_null": true,
					"is_bound": false,
					"value": "",
					"errors": null
				},
				"range_start": {
					"is_null": true,
					"is_bound": false,
					"value": "",
					"errors": null
				},
				"read_status": {
					"is_null": true,
					"is_bound": false,
					"value": null,
					"errors": null
				},
				"title": {
					"is_null": true,
					"is_bound": false,
					"value": "",
					"errors": null
				},
				"type": {
					"is_null": true,
					"is_bound": false,
					"value": null,
					"errors": null
				}
			}
		}`),
	)

	client.RT(t,
		WithMethod(http.MethodPost),
		WithTarget("/api/bookmarks/collections"),
		WithBody(map[string]any{
			"name":      "test-collection",
			"is_marked": true,
			"type":      []string{"article"},
			"labels":    "test 🥳",
		}),
		AssertStatus(201),
		AssertRedirect("/api/bookmarks/collections/.+"),
		AssertJSON(`{"status":201,"message":"Collection created"}`),
	)

	client.RT(t,
		WithBody(true),
		WithTarget(client.History[0].Response.Redirect),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"href": "<<PRESENCE>>",
			"created": "<<PRESENCE>>",
			"updated": "<<PRESENCE>>",
			"name": "test-collection",
			"is_pinned": false,
			"is_deleted": false,
			"search":"",
			"title":"",
			"author":"",
			"site":"",
			"type": ["article"],
			"labels":"test 🥳",
			"read_status": null,
			"is_marked": true,
			"is_archived": null,
			"is_loaded": null,
			"has_errors": null,
			"has_labels": null,
			"range_start": "",
			"range_end": ""
		}`),
	)

	client.RT(t,
		WithMethod(http.MethodPatch),
		WithTarget(client.History.PrevURL()),
		WithBody(map[string]any{
			"name":      "new name",
			"is_pinned": true,
		}),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"is_pinned": true,
			"name": "new name",
			"updated": "<<PRESENCE>>"
		}`),
	)

	client.RT(t,
		WithBody(true),
		WithTarget("/api/bookmarks/collections"),
		AssertStatus(200),
		AssertJSON(`[
			{
				"id": "<<PRESENCE>>",
				"href": "<<PRESENCE>>",
				"created": "<<PRESENCE>>",
				"updated": "<<PRESENCE>>",
				"name": "new name",
				"is_pinned": true,
				"is_deleted": false,
				"search":"",
				"title":"",
				"author":"",
				"site":"",
				"type": ["article"],
				"labels":"test 🥳",
				"read_status": null,
				"is_marked": true,
				"is_archived": null,
				"is_loaded": null,
				"has_errors": null,
				"has_labels": null,
				"range_start": "",
				"range_end": ""
			}
		]`),
	)

	client.RT(t,
		WithMethod(http.MethodPatch),
		WithTarget(client.History[1].URL.String()),
		WithBody(map[string]any{
			"name":        "new name",
			"is_archived": nil,
			"is_marked":   false,
		}),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"is_archived": null,
			"is_marked": false,
			"name": "new name",
			"updated": "<<PRESENCE>>"
		}`),
	)

	client.RT(t,
		WithTarget(client.History.PrevURL()),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"href": "<<PRESENCE>>",
			"created": "<<PRESENCE>>",
			"updated": "<<PRESENCE>>",
			"name": "new name",
			"is_pinned": true,
			"is_deleted": false,
			"search":"",
			"title":"",
			"author":"",
			"site":"",
			"type": ["article"],
			"labels":"test 🥳",
			"read_status": null,
			"is_marked": false,
			"is_archived": null,
			"is_loaded": null,
			"has_errors": null,
			"has_labels": null,
			"range_start": "",
			"range_end": ""
		}`),
	)

	client.RT(t,
		WithMethod(http.MethodPatch),
		WithTarget(client.History.PrevURL()),
		WithBody(map[string]any{
			"name":        "new name",
			"is_archived": nil,
			"is_marked":   nil,
		}),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"is_archived": null,
			"is_marked": null,
			"name": "new name",
			"updated": "<<PRESENCE>>"
		}`),
	)

	client.RT(t,
		WithTarget(client.History.PrevURL()),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"href": "<<PRESENCE>>",
			"created": "<<PRESENCE>>",
			"updated": "<<PRESENCE>>",
			"name": "new name",
			"is_pinned": true,
			"is_deleted": false,
			"search":"",
			"title":"",
			"author":"",
			"site":"",
			"type": ["article"],
			"labels":"test 🥳",
			"read_status": null,
			"is_marked": null,
			"is_archived": null,
			"is_loaded": null,
			"has_errors": null,
			"has_labels": null,
			"range_start": "",
			"range_end": ""
		}`),
	)

	client.RT(t,
		WithMethod(http.MethodPatch),
		WithTarget(client.History.PrevURL()),
		WithBody(map[string]any{
			"name":        "new name",
			"search":      "some search title:tt label:label1 label:label2 site:example.com",
			"type":        []string{"article", "video"},
			"read_status": []string{"unread", "reading"},
		}),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"labels":"label1 label2 test 🥳",
			"name": "new name",
			"read_status": ["unread", "reading"],
			"search":"some search",
			"site":"example.com",
			"title":"tt",
			"type": ["article", "video"],
			"updated": "<<PRESENCE>>"
		}`),
	)

	client.RT(t,
		WithTarget(client.History.PrevURL()),
		AssertStatus(200),
		AssertJSON(`{
			"id": "<<PRESENCE>>",
			"href": "<<PRESENCE>>",
			"created": "<<PRESENCE>>",
			"updated": "<<PRESENCE>>",
			"name": "new name",
			"is_pinned": true,
			"is_deleted": false,
			"search":"some search",
			"title":"tt",
			"author":"",
			"site":"example.com",
			"type": ["article", "video"],
			"labels":"label1 label2 test 🥳",
			"read_status": ["unread", "reading"],
			"is_marked": null,
			"is_archived": null,
			"is_loaded": null,
			"has_errors": null,
			"has_labels": null,
			"range_start": "",
			"range_end": ""
		}`),
	)

	client.RT(t,
		WithMethod(http.MethodDelete),
		WithTarget(client.History.PrevURL()),
		AssertStatus(204),
	)

	client.RT(t,
		WithTarget(client.History.PrevURL()),
		AssertStatus(404),
	)
}
