// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

const cspNonce = document.querySelector(
  'html>head>meta[name="csp-nonce"]',
).content

document.addEventListener("turbo:before-fetch-request", (evt) => {
  // Method MUST be uppercase
  evt.detail.fetchOptions.method = evt.detail.fetchOptions.method.toUpperCase()

  // Mark the request for turbo rendering
  evt.detail.fetchOptions.headers["X-Turbo"] = "1"
  evt.detail.fetchOptions.headers["X-Turbo-Nonce"] = cspNonce
})

document.addEventListener("turbo:submit-end", (evt) => {
  // Empty children with data-turbo-empty-submit-end
  // attribute after form submission.
  evt.target
    .querySelectorAll("[data-turbo-empty-submit-end]")
    .forEach((x) => (x.value = ""))
})

export {cspNonce}
