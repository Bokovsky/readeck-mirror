// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"
import {request} from "../lib/request"
import {cspNonce, markStaleRefresh} from "../lib/turbo"

export default class extends Controller {
  static values = {
    url: {type: String, default: ""},
    method: {type: String, default: ""},
    turbo: {type: Boolean, default: false},
  }

  async fetch(event) {
    const src = !!event.params.url ? event.params.url : this.urlValue
    if (!src) {
      throw new Error("url param is not set")
    }

    const method = !!event.params.method
      ? event.params.method
      : this.methodValue
    const options = {
      method: method || "get",
      headers: {},
    }

    if (this.turboValue) {
      options.headers["X-Turbo"] = "1"
      options.headers = {
        ...options.headers,
        "X-Turbo": "1",
        "X-Turbo-Nonce": cspNonce,
      }
    }

    if (event.currentTarget.tagName.toLowerCase() == "form") {
      // Set the body based on the form elements when the event receiver
      // is a form.
      const form = event.currentTarget
      options.body = new FormData()
      for (let e of form.elements) {
        if (!!e.name) {
          options.body.set(e.name, e.value)
        }
      }
    } else if (
      event.currentTarget.name !== undefined &&
      event.currentTarget.value !== undefined
    ) {
      // Otherwise, anything with a name and value attribute does the job.
      options.body = new FormData()
      options.body.set(event.currentTarget.name, event.currentTarget.value)
    }

    const rsp = await request(src, options)

    // Mark any unsafe request for refresh on back/forward navigation.
    if (
      rsp.ok &&
      !["GET", "HEAD", "OPTIONS"].includes(options.method.toUpperCase())
    ) {
      markStaleRefresh()
    }

    if (this.turboValue) {
      Turbo.renderStreamMessage(await rsp.text())
    }

    this.dispatch(event.params.eventName || "done")
  }
}
