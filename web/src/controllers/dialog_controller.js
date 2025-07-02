// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static values = {
    select: String,
  }

  open() {
    if (!this.hasSelectValue) {
      return
    }

    document.querySelector(this.selectValue).showModal()
  }
}
