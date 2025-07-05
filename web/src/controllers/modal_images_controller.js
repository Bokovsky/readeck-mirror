// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["dialog", "img", "trigger"]

  connect() {
    this.element.querySelectorAll("img").forEach((e) => this.addTrigger(e))
    this.dialogTarget.addEventListener("click", (evt) => {
      this.dialogTarget.close()
    })
  }

  addTrigger(e) {
    if (e == this.imgTarget) {
      return
    }

    const wrapper = this.triggerTarget.content.cloneNode(true).firstElementChild
    const b = wrapper.firstElementChild

    e.parentNode.insertBefore(wrapper, e)
    wrapper.appendChild(e)

    b.addEventListener("click", (evt) => {
      evt.preventDefault()
      this.imgTarget.setAttribute("src", e.src)
      this.dialogTarget.showModal()
    })
  }
}
