<!--
SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>

SPDX-License-Identifier: AGPL-3.0-only
-->

This route creates a new bookmark.

Only the `url` parameter is mandatory. The route's body can be a JSON object or a `multipart/form-data` payload.

### Sending an existing HTML content as a page

If you want to save a page for which you already have an HTML content, you can call this route with an `html` parameter that contains the page's source. This parameter is accepted in `multipart/form-data` and `JSON`.

#### Example with a JSON payload

```http
POST __BASE_URI__/bookmarks

Content-Type: application/json
Accept: application/json

{
    "url": "https://example.org/",
    "html": "<p>test"
}
```

#### Example with a multipart payload

```request
POST __BASE_URI__/bookmarks

Content-Type: multipart/form-data;boundary="delimiter123"
Accept: application/json

--delimiter123
Content-Disposition: form-data; name="url"

https://example.org/
--delimiter123
Content-Disposition: form-data; name="html"; filename="_"

<p>test
--delimiter123--
```

### Sending non HTML and/or multiple resources

If you want to save a page with extra resources (such as images), you need to call this route with
a `multipart/form-data` body and some `resource` values.

Each `resource` entry is a file part, it **must** contain the following headers:

| Name                  | Content                                    |
| :-------------------- | :----------------------------------------- |
| `Content-Disposition` | `form-data; name="resource"; filename="_"` |
| `Location`            | `https://some-url`                         |

The `filename` property in `Content-Disposition` **must** have a value, otherwise headers won't
be read properly. The value doesn't matter and can be, for example, `_`.

A resource can have more headers, like a `Content-Type`.

#### Example

```
POST __BASE_URI__/bookmarks

Content-Type: multipart/form-data;boundary="delimiter123"
Accept: application/json

--delimiter123
Content-Disposition: form-data; name="url"

https://example.org/
--delimiter123
Content-Disposition: form-data; name="resource"; filename="_"
Location: https://example.org/
Content-Type: text/html

<p>test
    <img src="/logo.svg">
--delimiter123
Content-Disposition: form-data; name="resource"; filename="_"
Location: https://example.org/logo.svg
Content-Type: image/svg+xml

<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1">
<path d="M0,0h1v1H0" fill="#f00"/>
</svg>
--delimiter123--
```
