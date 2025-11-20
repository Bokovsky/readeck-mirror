<!--
SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>

SPDX-License-Identifier: AGPL-3.0-only
-->
# Introduction

The Readeck API provides REST endpoints that can be used for any purpose, should it be a mobile application, a script, you name it.

## API Endpoint

You can access this API on `__BASE_URI__`.

Most of your requests and responses are using JSON as the exchange format.

## Test the API

On this documentation, you can test every route.

If you don't provide an API token in [Authentication](#auth), you can still test all the routes but note that the given curl examples only work with an API token.

# Token Authentication

If you're writing a script for yourself, the easiest way is to [generate an API token](../profile/tokens) that you can use using the `Bearer` HTTP authorization scheme.

For example, you first request will look like:

```sh
curl -H "Authorization: Bearer <TOKEN>" __BASE_URI__/profile
```

Or, in NodeJS:

```js
fetch("__BASE_URI__/profile", {
    headers: {
    "Authorization": "Bearer <TOKEN>",
    },
})
```