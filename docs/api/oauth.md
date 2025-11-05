<!--
SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>

SPDX-License-Identifier: AGPL-3.0-only
-->

# Authentication with OAuth

If you're writing an application that requires a user to grant the application permission to access their Readeck instance, you should not ask a user to create an API Token but instead, implement the necessary OAuth flow so your application can retrieve a token in a user friendly way.

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

Readeck implement [OAuth 2.0 Dynamic Client Registration Protocol](https://datatracker.ietf.org/doc/html/rfc7591). You can register a client by querying the [Client Creation Route](#post-/oauth/client).

Upon registration, you'll receive a `client_id` that you can use in the next authorization step.

Unlike more traditional client implementations, Readeck OAuth clients are ephemeral:

- You **must** register a new client each time you start an authorization flow.
- The Client is valid for 10 minutes after creation.

## OAuth Authorization Code Flow

The Authorization Code Flow is used by clients to exchange an authorization code for an access token.

After the user returns to the client via the redirect URL, the application will get the authorization code from the URL and use it to request an access token.

This flow can only be used when, on the same device, the client can:

- send the user to the authorization page
- process the redirect URL to retrieve the authorization code

On a device without a browser, a client can use the [Device Code Flow](#overview--oauth-device-code-flow).

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

With a `client_id`, you can use the authorization code flow. You first need to build an authorization URL.

### Authorization

The authorization URL is: `__ROOT_URI__/authorize` and it receives the following query parameters:

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
| `state`             | The state as initially set by the client                 |

Once you receive a code, you can proceed to the [Token Request](#post-/oauth/token) to eventually receive an access token that will let you use the API.

### PKCE

The authorization code flow requires that you use [PKCE](https://datatracker.ietf.org/doc/html/rfc7636) with an S256 method only (the "plain" method is not allowed).

1. The client creates a random **verifier** and produces a SHA-256 hash that is encoded in base64 to make a **challenge**.
2. The **challenge** is added to the authorization URL as `code_challenge` query parameter.
3. When requesting the token, the client sends the **verifier** as `code_verifier` parameter. Then the server, that kept track of the challenge can check it matches the received verifier.

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

## OAuth Device Code Flow

The Device Code Flow is used by browserless or input-constrained devices in the device flow to exchange a previously obtained device code for an access token. An e-reader is a good candidate for using this flow.

<details>
<summary>Device Code Flow</summary>
<pre role="img" aria-label="Device Code sequence diagram">
 ┌────┐               ┌──────┐                         ┌─────────────┐
 │User│               │Client│                         │Authorization│
 └─┬──┘               └──┬───┘                         └──────┬──────┘
   │                     │                                    │
   │                     │(1) Request device code             │
   │                     │───────────────────────────────────>│
   │                     │                                    │
   │                     │(2) Return device code, user code,  │
   │                     │URL and interval                    │
   │                     │<───────────────────────────────────│
   │                     │                                    │
   │(3) Provide user code│                                    │
   │    and URL to user  │                                    │
   │ <───────────────────│                                    │
   │                   ┌────┐───────────────────────────────────┐
   │                   │Loop│                                 │ │
   │                   └────┘                                 │ │
   │                   │ │                                    │ │
   │                   │ │(4) Poll for authorization          │ │
   │                   │ │───────────────────────────────────>│ │
   │                   │ │                                    │ │
   │                   │ │               authorization_pending│ │
   │                   │ │<───────────────────────────────────│ │
   │                   │ │                                    │ │
   │                   └────────────────────────────────────────┘
   │                     │                                    │
   │(5) Open authorization URL and enter user code            │
   ├ ────────────────────────────────────────────────────────>│
   │                     │                                    │
   │(5) Approve client access                                 │
   ├ ────────────────────────────────────────────────────────>│
   │                     │                                    │
   │                     │             (6) Return access_token│
   │                     │<───────────────────────────────────│
   │                     │                                    │
 ┌─┴──┐               ┌──┴───┐                         ┌──────┴──────┐
 │User│               │Client│                         │Authorization│
 └────┘               └──────┘                         └─────────────┘
</pre>
</details>

1. The client request access from Readeck on the [Device Authorization route](#post-/oauth/device)
2. Readeck issues a device code, an end-user code and provides the end-user verification URI. This information is valid for 5 minutes.
3. The client instructs the user to visit the provided end-user verification URI. The client provides the user with the end-user code to enter in order to review the authorization request.
4. While the user reviews the client's request (step 5), the client repeatedly polls the [Token route](#post-/oauth/token) to find out if the user completed the user authorization step. The client includes the device code and its client identifier. The token route can only be polled every 5 seconds.
5. After authentication, Readeck prompts the user to input the user code provided by the device client and prompts the user to accept or decline the request.
6. Readeck validates the device code provided by the client and responds with the access token if the client is granted access, an error if they are denied access, or a pending state, indicating that the client should continue to poll.

<details>
<summary>Python example of the device flow</summary>

```python
import json
import time

import httpx


def main():
    client = httpx.Client(
        base_url="__ROOT_URI__",
        headers={"Accept": "application/json"},
    )

    # Create a client
    rsp = client.post(
        "api/oauth/client",
        data={
            "client_name": "Test App",
            "client_uri": "https://example.net/",
            "software_id": uuid.uuid4(),
            "software_version": "1.0.2",
            "grant_types": ["urn:ietf:params:oauth:grant-type:device_code"],
        },
    )
    rsp.raise_for_status()
    client_id = rsp.json()["client_id"]

    # Get user code.
    rsp = client.post(
        "api/oauth/device",
        data={
            "client_id": client_id,
            "scope": "bookmarks:read bookmarks:write",
        },
    )
    rsp.raise_for_status()

    req_data = rsp.json()

    # The client keeps the device code for itself.
    device_code = req_data["device_code"]

    # User code with a separator for better readability
    user_code = f"{req_data['user_code'][0:4]}-{req_data['user_code'][4:]}"

    # Refresh interval
    interval = req_data["interval"]

    # Information the client must provide the user with.
    print(f"CODE         : {user_code}")
    print(f"URL          : {req_data['verification_uri']}")
    print(f"COMPLETE URL : {req_data['verification_uri_complete']}")

    # Now, the client waits for the user to accept or deny
    # the authorization request.
    wait = 0
    while True:
        if wait > 0:
            # wait before the request so we can use continue in the loop
            time.sleep(wait)
        else:
            wait = interval

        rsp = client.post(
            "api/oauth/token",
            data={
                "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
                "client_id": client_id,
                "device_code": device_code,
            },
        )
        if rsp.status_code >= 500:
            rsp.raise_for_status()

        data = rsp.json()

        if data.get("access_token"):
            print("Token retrieved!")
            print(json.dumps(data, indent=2))
            return

        error = data.get("error")
        match error:
            case "access_denied":
                # The user denied the request
                print("Access was denied")
                return
            case "slow_down":
                # Server asks to slow down, we'll sleep 5s
                continue
            case "authorization_pending":
                # Still waiting
                print("Waiting for authorization...")
                continue
            case "expired_token":
                # The request has expired
                print("Request has expired")
                return
            case _:
                print(f"Fatal error: {error}")
                return


if __name__ == "__main__":
    main()
```

</details>
