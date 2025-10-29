<!--
SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>

SPDX-License-Identifier: AGPL-3.0-only
-->
# Authentication with OAuth

If you're writing an application that requires a user to grant the application permission to access their Readeck instance, you should not ask a user to create an API Token but, instead, implement the necessary OAuth flow so your application can retrieve a token in a user friendly way.

## Available Scopes

An OAuth token grants the application some permissions based on the requested scopes. This are the available scopes you can request:

| Name              | Description                    |
| :---------------- | ------------------------------ |
| `bookmarks:read`  | Read only access to bookmarks  |
| `bookmarks:write` | Write only access to bookmarks |
| `profile:read`    | Extended profile information   |

You can see which scope applies on each route of this documentation. A route without a scope (and not "public") is not available with an OAuth token.

## Client Registration

Before you can start the authorization flow, you first need to register a client on the Readeck instance.

<details>
<summary>Client Registration Flow</summary>
<pre role="img" aria-label="Client Registration sequence diagram">
 ┌──────┐                 ┌────────────┐
 │Client│                 │Registration│
 └──┬───┘                 └─────┬──────┘
    │                           │
    │Client Registration Request│
    │POST /api/oauth/client     │
    │──────────────────────────>│
    │                           │
    │Client Information Response│
    │<──────────────────────────│
 ┌──┴───┐                 ┌─────┴──────┐
 │Client│                 │Registration│
 └──────┘                 └────────────┘
</pre>
</details>

<details>
<summary>Client Management Flow</summary>
<pre role="img" aria-label="Client Management sequence diagram">
 ┌──────┐                       ┌────────────┐
 │Client│                       │Registration│
 └──┬───┘                       └─────┬──────┘
    │                                 │
    │   Client Information Request    │
    │   GET /api/oauth/client/{id}    │
    │────────────────────────────────>│
    │                                 │
    │   Client Information Response   │
    │<────────────────────────────────│
    │                                 │
    │Read or Update Request           │
    │GET or PUT /api/oauth/client/{id}│
    │────────────────────────────────>│
    │                                 │
    │   Client Information Response   │
    │<────────────────────────────────│
    │                                 │
    │  Delete Request                 │
    │  DELETE /api/oauth/client/{id}  │
    │────────────────────────────────>│
    │                                 │
    │       Delete Confirmation       │
    │<────────────────────────────────│
 ┌──┴───┐                       ┌─────┴──────┐
 │Client│                       │Registration│
 └──────┘                       └────────────┘
</pre>
</details>

Readeck implement [OAuth 2.0 Dynamic Client Registration Protocol](https://datatracker.ietf.org/doc/html/rfc7591) and [OAuth 2.0 Dynamic Client Registration Management Protocol](https://datatracker.ietf.org/doc/html/rfc7592).

You can register a client by querying the [Client Creation Route](#post-/oauth/client).

Upon registration, you'll receive a `client_id` and a `registration_access_token`. You'll need them if you want to fetch, update or delete the client later. You must store this information as safely as a password.

Once you have a client, you can retrieve its information, update it or delete it. See:

- [Client Info](#get-/oauth/client/-id-)
- [Client Update](#put-/oauth/client/-id-)
- [Client Delete](#delete-/oauth/client/-id-)

<details>
<summary>Javascript example of a client flow for an app</summary>

```js
async function clientFlow() {
  // Where you store the client_id and registration_access_token
  const appVersion = "1.1.0"
  const store = {}
  let rsp

  if (!store.clientID) {
    // New client, create one
    rsp = await fetch("__BASE_URI__/oauth/client", {
      method: "POST",
      body: json.Stringify({
        client_name: "My new client",
        client_uri: "https://example.org/",
        redirect_uris: ["https://example.org/callback"],
        software_id: "some-uuid",
        software_version: "1.0.0",
      }),
    })
    let data = await rsp.json()
    store.clientID = data["client_id"]
    store.registrationAccessToken = data["registration_access_token"]

    // we're done
    return
  }

  // We have a client id, check if we need to update it
  rsp = await fetch(`__BASE_URI__/oauth/client/${store.clientID}`, {
    headers: { Authorization: `Bearer ${store.registrationAccessToken}` },
  })
  let data = await rsp.json()

  if (data["software_version"] != appVersion) {
    // We need to update the client
    rsp = await fetch(`__BASE_URI__/oauth/client/${store.clientID}`, {
      method: "PUT",
      body: json.Stringify({
        ...data,
        software_version: appVersion,
      }),
      headers: { Authorization: `Bearer ${store.registrationAccessToken}` },
    })
    let data = await rsp.json()
  }
}
```

</details>

## OAuth Authorization Code Flow

<details>
<summary>Authorization Code Flow</summary>

<pre role="img" aria-label="Authorization Code sequence diagram">
 ┌────┐            ┌──────┐                               ┌─────────────┐      ┌───┐
 │User│            │Client│                               │Authorization│      │API│
 └─┬──┘            └──┬───┘                               └──────┬──────┘      └─┬─┘
   │                  │                                          │               │
   │Enter instance URL│                                          │               │
   │─────────────────>│                                          │               │
   │                  │                                          │               │
   │                  │──┐                                       │               │
   │                  │  │ Generate PKCE verifier and challenge  │               │
   │                  │<─┘                                       │               │
   │                  │                                          │               │
   │                  │        Open Authorization URL            │               │
   │                  │        GET /authorize?...                │               │
   │                  │─────────────────────────────────────────>│               │
   │                  │                                          │               │
   │         Redirect to login/authorization prompt              │               │
   │<────────────────────────────────────────────────────────────│               │
   │                  │                                          │               │
   │Authorize Client                                             │               │
   │POST /authorize?...                                          │               │
   │────────────────────────────────────────────────────────────>│               │
   │                  │                                          │               │
   │                  │          Authorization Code              │               │
   │                  │<─────────────────────────────────────────│               │
   │                  │                                          │               │
   │                  │──┐                                       │               │
   │                  │  │ Check state                           │               │
   │                  │<─┘                                       │               │
   │                  │                                          │               │
   │                  │Request Token (with code and verifier)    │               │
   │                  │POST /api/oauth/token                     │               │
   │                  │─────────────────────────────────────────>│               │
   │                  │                                          │               │
   │                  │                                          │──┐            │
   │                  │                                          │  │ Check PKCE │
   │                  │                                          │<─┘            │
   │                  │                                          │               │
   │                  │             Access Token                 │               │
   │                  │<─────────────────────────────────────────│               │
   │                  │                                          │               │
   │                  │         Request data with Access Token   │               │
   │                  │─────────────────────────────────────────────────────────>│
   │                  │                                          │               │
   │                  │                    Response              │               │
   │                  │<─────────────────────────────────────────────────────────│
 ┌─┴──┐            ┌──┴───┐                               ┌──────┴──────┐      ┌─┴─┐
 │User│            │Client│                               │Authorization│      │API│
 └────┘            └──────┘                               └─────────────┘      └───┘
</pre>

</details>

With the `client_id`, you can use the authorization code flow. You first need to build an authorization URL.

### Authorization

The authorization route is: `__ROOT_URI__/authorize` and it receives the following query parameters:

| Name                    | Description                                                                  |
| :---------------------- | :--------------------------------------------------------------------------- |
| `client_id`             | OAuth Client ID                                                              |
| `redirect_uri`          | Redirection URI (must match exactly one given during client registration)    |
| `scope`                 | Space separated list of [scopes](#overview--available-scopes). At least one. |
| `code_challenge`        | [PKCE](#overview--pkce) Challenge (mandatory)                                |
| `code_challenge_method` | Only `S256` is allowed                                                       |
| `state`                 | Optional [client state](#overview--state)                                    |

Sending a state is not mandatory but strongly advised to prevent cross site request forgery.

### Authorization result

Once a user grants or denies an authorization request, it will be redirected to the `redirect_uri` with the following query parameters:

| Name    | Description                                                           |
| :------ | :-------------------------------------------------------------------- |
| `code`  | The authorization code that the client must pass to the token request |
| `state` | The state as initially set by the client                              |

In case of error (request denied by the user or something else), the redirection contains
the following query parameters:

| Name                | Description                                              |
| :------------------ | :------------------------------------------------------- |
| `error`             | Error code (can be `invalid_request` or `access_denied`) |
| `error_description` | Error description                                        |

Once you receive a code, you can proceed to the [Token Request](#post-/oauth/token) to eventually receive an access token that will let you use the API.

### PKCE

The authorization code flow requires that you use [PKCE](https://datatracker.ietf.org/doc/html/rfc7636) with an S256 method only (the "plain" method is not allowed).

The client creates a random **verifier** and produces a SHA-256 hash that is encoded in base64 to make a **challenge**.

The **challenge** is added to the authorization URL as `code_challenge` query parameter.

When requesting the token, the client sends the **verifier** as `code_verifier` parameter. Then the server, that kept track of the challenge can check it matches the received verifier.

**Important**: The challenge must be base64 encoded, **with URL encoding** and **without padding**.

<details part="details">
<summary>Javascript example of a verifier and challenge generation</summary>

```js
// This generates a 64 character long random alphanumeric string.
function generateRandomString() {
  const alphabet =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
  let res = ""
  const buf = new Uint8Array(64)
  crypto.getRandomValues(buf)
  for (let i in buf) {
    res += alphabet[buf[i] % alphabet.length]
  }
  return res
}

// This hashes the verifier and encodes the hash to URL safe base64.
async function pkceChallengeFromVerifier(v) {
  const b = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(v))
  return btoa(String.fromCharCode(...new Uint8Array(b)))
    .replaceAll("+", "-")
    .replaceAll("/", "_")
    .replaceAll("=", "")
}

const verifier = generateRandomString()
pkceChallengeFromVerifier(verifier).then((challenge) => {
  console.log(verifier)
  console.log(challenge)
})
```

</details>

### State

The `state` parameter that the client can add to the authorization URL is for the client only. When present, it is sent back in the redirection URI that contains the authorization code. The client can keep track of it and check it matches its initial value. It is strongly recommended to use it.
