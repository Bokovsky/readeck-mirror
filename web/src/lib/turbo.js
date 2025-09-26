// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

const cspNonce = document.querySelector(
  'html>head>meta[name="csp-nonce"]',
).content

const staleRefreshKey = "stale-refresh"

function markStaleRefresh() {
  window.localStorage.setItem(staleRefreshKey, "true")
}

window.addEventListener("pageshow", (event) => {
  // Bypass browser's bfcache when the user navigate using
  // the back button and when we have a specific local storage key.

  if (window.localStorage.getItem(staleRefreshKey) != "true") {
    return
  }
  window.localStorage.removeItem(staleRefreshKey)

  if (
    event.persisted ||
    window.performance.getEntriesByType("navigation")[0].type == "back_forward"
  ) {
    window.location.reload()
  }
})

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

  // Mark any unsafe request for refresh on back/forward navigation.
  if (
    evt.detail.success &&
    !["GET", "HEAD", "OPTIONS"].includes(
      evt.detail.formSubmission.method.toUpperCase(),
    )
  ) {
    markStaleRefresh()
  }
})

export {cspNonce, markStaleRefresh}
