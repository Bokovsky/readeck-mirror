# Readeck

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
<a rel="me" href="https://mastodon.online/@readeck"><img src="https://img.shields.io/badge/%40readeck-blue?logo=mastodon&logoColor=%23fff&color=%236364ff" alt="Follow on Mastodon" /></a>

Readeck is a simple web application that lets you save the
precious readable content of web pages you like and want to keep
forever. \
See it as a bookmark manager and a read later tool.

![Readeck Bookmark List](./screenshots/bookmark-list.webp)

## Contents

- [Features](#features)
- [Installation](#how-to-test-or-install)
- [FAQ](#faq)
- [Under the hood](#under-the-hood)
- [License](#license)

## Features

### 🔖 Bookmarks

Like a page you're reading? Paste the link in Readeck and you're done!

### 📸 Articles, pictures and videos

Readeck saves the readable content of web pages for you to read later. It also detects when a page is an image or a video and adapts its process accordingly.

### ⭐ Labels, favorites, archives

Move bookmarks to archives or favorites and add as many labels as you want.

### 🖍️ Highlights

Highlight the important content of your bookmarks to easily find it later.

### 🗃️ Collections

If you need a dedicated section with all your bookmarks from the past 2 weeks labeled with "cat", Readeck lets you save this search query into a collection so you can access it later.

### 🧩 Browser Extension

Want to keep something for later while browsing? No need to copy and paste a link. Install the browser extension and save bookmarks in one click!

- [For Mozilla Firefox](https://addons.mozilla.org/en-US/firefox/addon/readeck/)
- [For Google Chrome](https://chromewebstore.google.com/detail/readeck/jnmcpmfimecibicbojhopfkcbmkafhee)
- [More Information and Source Code](https://codeberg.org/readeck/browser-extension)

### 📖 E-Book export

What's better than reading your collected articles on your e-reader? You can export any article to an e-book file (EPUB). You can even export a collection to a single book!

On top of that, you can directly access Readeck's catalog and collections from your e-reader if it supports OPDS.

### 🔎 Full text search

Whether you need to find a vague piece of text from an article, or all the articles with a specific label or from a specific website, we've got you covered!

### 🚀 Fast!

Readeck is a modern take on so-called boring, but proven, technology pieces. It guarantees very quick response times and a smooth user experience.

### 🔒 Built for your privacy and long term archival

Will this article you like be online next year? In 10 year? Maybe not; maybe it's all gone, text and images. For this reason, and for your privacy, text and images are all stored in your Readeck instance the moment you save a link.

With the exception of videos, not a single request is made from your browser to an external website.

## How to test or install

Done reading this promotional content? Good! Want to try Readeck on your laptop or a server? Even better!

### Container

To install or test Readeck with Docker or Podman, simply run the image:

```shell
docker run --rm -ti -p 8000:8000 -v readeck-data:/readeck codeberg.org/readeck/readeck:latest
```

You'll find all the container images there: \
[https://codeberg.org/readeck/-/packages/container/readeck/latest](https://codeberg.org/readeck/-/packages/container/readeck/latest)

### Binary file installation

Readeck is distributed as a single binary file. Using it is almost as easy as a container.

- Create a new directory
  ```shell
  mkdir -p readeck-install
  cd readeck-install
  ```
- Download the file matching your system from the [last release](https://codeberg.org/readeck/readeck/releases)

- Make this file executable
- Launch Readeck with the `serve` argument, for example:
  ```shell
  ./readeck-0.9.1-linux-amd64 serve
  ```

### First time launch

Once Readeck has started, it is accessible at: \
**[http://localhost:8000/](http://localhost:8000/)**

### Installation from source code

Installing from source can be useful to try out unreleased Readeck versions, or to set up a development environment for contributing code. For more information, see the [development documentation](https://readeck.org/en/docs/dev).

### Installation for production

If you already know how to deploy containers on a server, installing Readeck for production should be quite straightforward. As for deploying to Linux servers with systemd and NGINX reverse proxy, see the [deployment guide](https://readeck.org/en/docs/deploy).

## FAQ

Please refer to the [FAQ on readeck.org](https://readeck.org/en/docs/faq).

## Under the hood

Readeck was born out of frustration (and COVID lock-downs) from the tools that don't save everything related to the saved content, primarily images.
This key principle guided every step of Readeck development.

### The ZIP file

Every bookmark is stored in a single, immutable, ZIP file. Parts of this file (HTML content, images, etc.) are directly served by the application or converted to a web page or an e-book when needed.

### A simple database

Readeck has a very simple database schema with a few tables (and a bit of clever JSON fields here and there). The recommended database engine is SQLite for most installations.

### A simple stack

Unlike many modern web applications, Readeck is not a single page application built on top of an API with impossible to install dependencies and a mess of background processes.

Readeck is written in [Go](https://go.dev/) and all its content is rendered server side with some interactivity brought by [Stimulus](https://stimulus.hotwired.dev/) and [Turbo](https://turbo.hotwired.dev/).

This has proven to be a great combination when performance really matters.

### Contributing

Readeck's simple stack means that the barrier to entry is relatively low when it comes to contributing to the Readeck project. Contributions are welcome by filing issues, participating in community discussions, submitting translations, and proposing changes to the source code. For more information, see the [documentation on contributing](https://readeck.org/en/contribute).

## License

Readeck is distributed under the terms of the [GNU Affero General Public License v3.0](https://www.gnu.org/licenses/agpl-3.0.html). Here's a short summary of the license conditions:

- Permissions
  - **Commercial use** \
     The licensed material and derivatives may be used for commercial purposes.
  - **Distribution** \
     The licensed material may be distributed.
  - **Modification** \
     The licensed material may be modified.
  - **Patent use** \
     This license provides an express grant of patent rights from contributors.
  - **Private use** \
     The licensed material may be used and modified in private.
- Conditions
  - **Disclose source** \
    Source code must be made available when the licensed material is distributed.
  - **License and copyright notice** \
    A copy of the license and copyright notice must be included with the licensed material.
  - **Network use is distribution** \
    Users who interact with the licensed material via network are given the right to receive a copy of the source code.
  - **Same license** \
    Modifications must be released under the same license when distributing the licensed material. In some cases a similar or related license may be used.
  - **State changes** \
    Changes made to the licensed material must be documented.
- Limitations
  - **Liability** \
    This license includes a limitation of liability.
  - **Warranty** \
    This license explicitly states that it does NOT provide any warranty.
