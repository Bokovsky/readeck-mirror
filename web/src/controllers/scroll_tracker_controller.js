// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static classes = ["down"]
  static values = {
    delay: {type: Number, default: 250},
    delta: {type: Number, default: 30},
  }

  connect() {
    let prevScroll = window.scrollY
    let ticking = false
    let t = null

    window.addEventListener(
      "scroll",
      () => {
        if (!ticking) {
          window.requestAnimationFrame(() => {
            if (t !== null) {
              window.clearTimeout(t)
            }
            t = window.setTimeout(() => {
              ticking = false

              const y = window.scrollY
              const ymax =
                document.documentElement.scrollHeight - window.innerHeight
              const cur = Math.min(Math.max(y, 0), ymax)
              const delta = cur - prevScroll

              try {
                if (ymax <= y + this.deltaValue) {
                  // Reaching the end, go into "up" state
                  this.element.classList.remove(this.downClass)
                  return
                }

                if (Math.abs(delta) < this.deltaValue) {
                  // Scroll interval too short, stop now
                  return
                }

                if (delta < 0) {
                  // Scrolling up
                  this.element.classList.remove(this.downClass)
                } else {
                  // Scrolling down
                  this.element.classList.add(this.downClass)
                }
              } finally {
                prevScroll = cur
                t = null
              }
            }, this.delayValue)
          })

          ticking = true
        }
      },
      {passive: true},
    )

    document.addEventListener("turbo:before-render", (evt) => {
      if (window.scrollY == 0) return
      // If we are scrolled down, apply the "down" class to the new body element before it replaces
      // the old body. This avoids the flash of CSS transition in which the navbar suddenly appears
      // and then immediately eases out of view on every Turbo re-render.
      evt.detail.newBody.classList.add(this.downClass)
    })
  }
}
