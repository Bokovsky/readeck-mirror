---
SPDX-FileCopyrightText: 2025 Mislav Marohnić <hi@mislav.net>
SPDX-License-Identifier: CC0-1.0
---

# How Readeck extracts content

Readeck typically ingests web pages in one of two ways:

1. A user submits a page URL; either via Readeck's web interface or via its API (which the [Readeck browser extension](https://codeberg.org/readeck/browser-extension) also uses). Readeck then fetches the document from that URL, which can be subject to site paywalls and other web traffic restrictions. Readeck also fetches and archives all the images referenced in the "readable" part of the article.

2. A user submits the URL along with its HTML contents. This is what the browser extension does; it ensures that the exact document that the user is viewing in a web browser is stored in their Readeck library.

With either method, Readeck ends up with a HTML document that needs to be processed to extract the readable article and to archive it as a bookmark. The content extraction approach can be roughly summarized as:

1. Extract document metadata;
2. Load site-specific rules;
3. Execute content scripts;
4. Apply site-specific rules;
5. Extract article contents from the larger document;
6. Archive the article contents, including the images.

## The implementation

The following is a non-exhaustive list of some Readeck's code components that facilitate document extraction:

* `AddProcessors` block in [internal/bookmarks/tasks/tasks.go](./tasks/tasks.go), where the Extractor is assembled, defines the **order of extraction rules** and is a good starting point for diving into this part of the codebase.

* Extractor delegates to `Load` method in [pkg/extract/drop.go](../../pkg/extract/drop.go) for **fetching web resources** using the HTTP client in [internal/httpclient](../httpclient/client.go)

* [pkg/extract/meta/meta.go](../../pkg/extract/meta/meta.go) **scans HTML tags for metadata** such as [Open Graph](https://ogp.me/) and [schema.org](https://schema.org/docs/gs.html)

* [pkg/extract/meta/oembed.go](../../pkg/extract/meta/oembed.go) fetches the [oEmbed](https://oembed.com/) representation of the document

* **Site-specific rules** are loaded from [pkg/extract/contentscripts/assets/site-config](../../pkg/extract/contentscripts/assets/site-config). These rules are indexed by the site's domain name and contain definitions like element selectors to strip away and content selectors to keep ([example](../../pkg/extract/contentscripts/assets/site-config/.medium.com.json)). The Readeck project does not author nor maintain these rules; they are being imported from the community-maintained [FiveFilters site config](https://github.com/fivefilters/ftr-site-config?tab=readme-ov-file#readme) repository.

* [Readeck content scripts](https://readeck.org/en/docs/content-scripts) are then executed; at least those that pass the `isActive` check for the site domain and the document in question. Content scripts written by the user can be loaded by Readeck and executed at runtime when saving each bookmark; their function is often to amend site-specific rules inherited in the previous step, but they could also execute arbitrary logic to replace whole portions of the document. For example, content scripts can "unroll" social media threads or inline all images from a gallery into a single HTML document.

  A handful of content scripts that ship with Readeck can be viewed in [pkg/extract/contentscripts/assets/scripts](../../pkg/extract/contentscripts/assets/scripts). They are mostly named by the site domain they apply to, with one notable script [0-config.js](../../pkg/extract/contentscripts/assets/scripts/0-config.js) being a place to store Readeck's site-specific overrides that don't belong upstream.

* Site-specific rules—potentially amended by content scripts—are then **applied to the HTML document**. For example, `ExtractBody` in [pkg/extract/contentscripts/processors.go](../../pkg/extract/contentscripts/processors.go) will extract any main content elements from the document and discard everything else.

* The HTML document is **run through a _readability_ pass** in [pkg/extract/contents/contents.go](../../pkg/extract/contents/contents.go). This is primarily being handled by [go-readability](https://codeberg.org/readeck/go-readability), Readeck's fork of a Go implementation of [Mozilla Readability.js](https://github.com/mozilla/readability?tab=readme-ov-file#readme) algorithm.

  The algorithm scores DOM elements on the page by how "content-y" they look, then applies various heuristics to determine which elements are most likely article contents and which elements should be safe to discard (for example, site navigation or footer contents). If no site rules are found for a specific document, or if those rules do not define any body selectors, the readability pass is solely responsible for extracting article contents from a web page.

* Images to archive are processed in [internal/bookmarks/archive.go](./archive.go). GIF, PNG, and JPEG images of reasonable size are stored without modifications; other image formats are transcoded to one of the supported formats and large images are downsized to fix a maximum width defined by the `ImageSizeWide` constant.

  WebP images are converted to either JPEG (for lossy WebP) or PNG (for lossless WebP). Readeck does this in order to support exporting bookmarks to EPUB, where WebP support is not widespread, and because there doesn't yet exist a WebP encoder in pure Go.

  Images that are 64px or smaller and almost square aspect ratio are thought to be UI icons (rather than content) and are stripped away.

* The bookmark is now fully archived and ready to read. All files related to the bookmark are stored in a ZIP archive `{data_directory}/bookmarks/*/{bookmark_id}.zip` that contains `index.html` that is the article contents, all archived images, and the full extraction debug log saved to `log`.

* The URL of each of the bookmark's hyperlinks is fetched by `fetchLinksProcessor()` to determine the content type and the title of the resource. After that is done, the list of bookmark's links is stored in the database.

## Debugging content extraction

When debug-level logging is enabled, Readeck server process outputs detailed information when processing and storing each bookmark. In Readeck's web interface, those logs are stored with each bookmark and can be viewed by interacting with the date in the sidebar representing when the bookmark was stored.

It's possible to post a HTML document to Readeck API to get detailed results of content extraction, _without_ that document being saved in Readeck library. This is suitable for development mode while editing Readeck source code or authoring a content script. Here is an example with curl:

```sh
# Send a HTML file to Readeck and extract the article content as if it were fetched from url.
curl -H 'Content-Type: text/html' -H "Authorization: Bearer ${READECK_TOKEN}" \
    'http://localhost:8000/api/cookbook/extract?url='"$url" --data @"$file"
```

Uploading the HTML document this way is preferrable to just submitting the URL and having Readeck repeatedly fetching it during development and testing, since some web publications might find that kind of programmatic traffic suspicious and temporarily block Readeck's IP address from accessing site resources.
