// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["dialog", "trigger", "template"]
  static values = {
    selector: {
      type: String,
      default: "",
    },
    baseUrl: {
      type: String,
      default: "",
    },
  }

  connect() {
    if (!this.selectorValue) {
      return
    }

    this.element
      .querySelectorAll(this.selectorValue)
      .forEach((e) => this.#addTrigger(e))
    this.dialogTarget.addEventListener("close", () => {
      this.#clearModal()
    })
    this.dialogTarget.addEventListener("click", () => {
      this.dialogTarget.close()
    })
  }

  get #iframe() {
    return this.dialogTarget.querySelector("iframe")
  }

  #clearModal() {
    this.#iframe.removeAttribute("src")
    this.#iframe.removeAttribute("width")
    this.#iframe.removeAttribute("height")
  }

  #addTrigger(e) {
    const params = new URLSearchParams(e.dataset.iframeParams)
    const w = params.get("w") || 0
    const h = params.get("h") || 0
    if (w == 0 || h == 0) {
      return
    }

    const wrapper = this.triggerTarget.content.cloneNode(true).firstElementChild
    const b = wrapper.firstElementChild

    e.parentNode.insertBefore(wrapper, e)
    wrapper.appendChild(e)

    b.addEventListener("click", (evt) => {
      const params = new URLSearchParams(e.dataset.iframeParams)

      const w = params.get("w") || 0
      const h = params.get("h") || 0

      const src = new URL(this.baseUrlValue, document.location)
      params.forEach((v, k) => {
        src.searchParams.set(k, v)
      })

      this.#iframe.setAttribute("src", src.toString())
      this.#iframe.setAttribute("width", w)
      this.#iframe.setAttribute("height", h)

      this.dialogTarget.showModal()
      this.dialogTarget.style.aspectRatio = `${w}/${h}`
    })
  }
}
