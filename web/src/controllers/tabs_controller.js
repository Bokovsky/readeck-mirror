// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static classes = ["hidden"]

  connect() {
    this.updateTabs()
  }

  toggle(evt) {
    const id = evt.target.id
    this.element.querySelectorAll("[role=tab]").forEach((t) => {
      if (id == t.id) {
        t.setAttribute("aria-selected", "true")
      } else {
        t.removeAttribute("aria-selected")
      }
    })
    this.updateTabs()
  }

  updateTabs() {
    this.element.querySelectorAll("[role=tab]").forEach((t) => {
      const selected = t.getAttribute("aria-selected") == "true"
      const target = this.element.querySelector(`[aria-labelledby=${t.id}]`)
      if (!target) {
        return
      }
      if (selected) {
        target.classList.remove(...this.hiddenClasses)
      } else {
        target.classList.add(...this.hiddenClasses)
      }
    })
  }
}
