// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  /**
   *
   * @param {Event} event
   * @returns
   */
  scroll(event) {
    if (event.currentTarget.tagName.toLowerCase() != "a") {
      return
    }
    let url
    try {
      url = new URL(event.currentTarget.href)
    } catch (e) {
      return
    }

    if (!url.hash) {
      return
    }

    const el = document.querySelector(url.hash)
    if (!el) {
      return
    }

    el.scrollIntoView({block: "start", inline: "nearest", behavior: "auto"})
    history.pushState({}, "", url)
  }
}
