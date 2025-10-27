<!--
SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>

SPDX-License-Identifier: AGPL-3.0-only
-->
This route returns a `multipart/mixed` response with all the bookmarks passed in `id` (or all of them if unset).

Here is an example:

```
--910345ce0f660bda92b9e8a1192532f999a51151dccb7227d784049b
Bookmark-Id: VnopmpKQ3CmQ6apY9mgDws
Content-Disposition: attachment; filename="info.json"
Content-Type: application/json; charset=utf-8
Date: 2025-06-20T10:53:47Z
Filename: info.json
Last-Modified: 2025-07-03T12:49:30Z
Location: http://localhost:8000/api/bookmarks/VnopmpKQ3CmQ6apY9mgDws
Type: json

...content

--965dd10345ce0f660bda92b9e8a1192532f999a51151dccb7227d784049b
Bookmark-Id: VnopmpKQ3CmQ6apY9mgDws
Content-Disposition: attachment; filename="index.html"
Content-Type: text/html; charset=utf-8
Date: 2025-06-20T10:53:47Z
Filename: index.html
Last-Modified: 2025-07-03T12:49:30Z
Type: html

...content

--965dd10345ce0f660bda92b9e8a1192532f999a51151dccb7227d784049b
Bookmark-Id: VnopmpKQ3CmQ6apY9mgDws
Content-Disposition: attachment; filename="image.jpeg"
Content-Length: 86745
Content-Type: image/jpeg
Filename: image.jpeg
Group: image
Location: http://localhost:8000/bm/Vn/VnopmpKQ3CmQ6apY9mgDws/img/image.jpeg
Path: image.jpeg
Type: resource

...content

--965dd10345ce0f660bda92b9e8a1192532f999a51151dccb7227d784049b
Bookmark-Id: VnopmpKQ3CmQ6apY9mgDws
Content-Disposition: attachment; filename="Wj66qLatSeikPc31FwvqyS.jpg"
Content-Length: 171749
Content-Type: image/jpeg
Filename: Wj66qLatSeikPc31FwvqyS.jpg
Group: embedded
Location: http://localhost:8000/bm/Vn/VnopmpKQ3CmQ6apY9mgDws/_resources/Wj66qLatSeikPc31FwvqyS.jpg
Path: Wj66qLatSeikPc31FwvqyS.jpg
Type: resource

...content

--965dd10345ce0f660bda92b9e8a1192532f999a51151dccb7227d784049b--
```


For each bookmark, there will be entries with a `Type` header:

- a `Type: json` entry, controlled by `with_json`. It contains the same output as an API bookmark information.
- a `Type: html` entry, controlled by `with_html`. It contains the HTML content (article), if any.
- a `Type: markdown` entry, controlled by `with_markdown`. It contains the bookmark converted to Markdown.
- several `Type: resource` entries, controlled by `with_resources`. Each entry is a resource (icon, images, article images).

Each entry has a `Bookmark-Id` attribute that indicates the bookmark it belongs to.

Each `Type: resource` entry contains a `Path` header that's based on the `resource_prefix`
parameter.

Each `Type: resource` entry contains a `Group` header that can take the following values:

- `icon`: the bookmark's icon,
- `image`: the bookmark's image (main picture for photo types, and placeholder for videos),
- `thumbnail`: thumbnail of the image,
- `embedded`: included in the article itself.

The response's content is a stream and should be processed while the data is coming, part by
part.