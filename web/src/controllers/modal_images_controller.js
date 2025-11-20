// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["dialog", "img", "trigger"]
  static values = {
    selector: {
      type: String,
      default: "img[src]",
    },
  }

  connect() {
    this.element
      .querySelectorAll(this.selectorValue)
      .forEach((e) => this.addTrigger(e))
    this.dialogTarget.addEventListener("click", (evt) => {
      this.dialogTarget.close()
    })
  }

  addTrigger(e) {
    if (e == this.imgTarget) {
      return
    }
    const src = e.src
    if (!src) {
      return
    }

    // Disable on small images (icons, etc.)
    if (e.width < 48 || e.height < 48 || (e.width < 128 && e.height < 128)) {
      return
    }

    const wrapper = this.triggerTarget.content.cloneNode(true).firstElementChild
    const b = wrapper.firstElementChild

    // If the image is the only child of a link, we used the link
    // a our wrapper's child node.
    // This is to avoid inserting a button inside a link element.
    if (e.parentNode.nodeName == "A" && e.parentNode.children.length == 1) {
      e = e.parentNode
    }

    e.parentNode.insertBefore(wrapper, e)
    wrapper.appendChild(e)

    b.addEventListener("click", (evt) => {
      evt.preventDefault()
      this.imgTarget.setAttribute("src", src)
      this.dialogTarget.showModal()
    })
  }
}
