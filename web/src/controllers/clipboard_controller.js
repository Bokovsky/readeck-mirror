// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["content", "icon"]

  async copy(evt) {
    evt.target.disabled = true
    await navigator.clipboard.writeText(this.contentTarget.value)

    const icon = evt.target.querySelector("svg")
    if (icon != undefined) {
      await this.bounceIcon(icon)
    }
    evt.target.disabled = false
  }

  /**
   * @param {HTMLElement} icon icon element
   */
  async bounceIcon(icon) {
    const svg = icon.cloneNode(true)
    icon.parentNode.style.position = "relative"
    svg.style.position = "absolute"
    svg.style.zIndex = "100"
    icon.parentNode.insertBefore(svg, icon)

    const animation = svg.animate(
      [
        {
          transform: "scale(8)",
          opacity: 0,
        },
      ],
      {
        duration: 500,
        iterations: 1,
      },
    )
    await animation.finished
    svg.remove()
  }
}
