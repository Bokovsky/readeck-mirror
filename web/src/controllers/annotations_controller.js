// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"
import {request} from "../lib/request"

export default class extends Controller {
  static targets = [
    "root",
    "controls",
    "color",
    "arrow",
    "whenNew",
    "whenOne",
    "whenMany",
  ]
  static classes = ["hidden"]
  static values = {
    apiUrl: String,
  }

  connect() {
    this.annotation = null
    document.addEventListener("selectionchange", async (evt) => {
      await this.onSelectText(evt)
    })

    const color = this.#getRegisteredColor()
    this.colorTargets.forEach((e, i) => {
      if (!color && i == 0) {
        e.checked = true
      } else if (e.value == color) {
        e.checked = true
      }

      e.addEventListener("click", (evt) => {
        this.#registerColor(evt.target.value)
      })
    })
  }

  /**
   * onSelectText is the listener for "selectionchange".
   *
   * @param {Event} evt selection event
   */
  async onSelectText(evt) {
    // We must wait for next tick so it won't trigger when the event triggers
    // from a click on an existing selection.
    await this.nextTick()

    const selection = document.getSelection()
    this.annotation = new Annotation(this.rootTarget, selection)

    if (
      this.annotation.isValid() ||
      this.annotation.coveredAnnotations().length > 0
    ) {
      await this.showControls()
    }
  }

  async showControls() {
    await this.nextTick()

    // Show controls
    this.controlsTarget.classList.remove(this.hiddenClass)

    /**
     * This hides the controls when there's no selection or a click
     * took place outside the controls.
     *
     * @param {Event} evt
     */
    const onClick = (evt) => {
      const selection = document.getSelection()
      if (!selection.isCollapsed) {
        return
      }
      if (this.controlsTarget.contains(evt.target)) {
        return
      }
      this.controlsTarget.classList.add(this.hiddenClass)
      document.removeEventListener("click", onClick)
    }

    /**
     * Hide the controls on Escape.
     *
     * @param {KeyboardEvent} evt
     */
    const onEsc = (evt) => {
      if (evt.key == "Escape") {
        this.controlsTarget.classList.add(this.hiddenClass)
        document.getSelection().removeAllRanges()
        document.removeEventListener("keyup", onEsc)
      }
    }

    document.addEventListener("click", onClick)
    document.addEventListener("keyup", onEsc)

    const covered = this.annotation.coveredAnnotations()

    // Show/hide sub controls
    switch (covered.length) {
      case 0:
        this.#hide(this.whenOneTargets)
        this.#hide(this.whenManyTargets)
        this.#show(this.whenNewTargets)
        break
      case 1:
        this.#hide(this.whenNewTargets)
        this.#hide(this.whenManyTargets)
        this.#show(this.whenOneTargets)
        break
      default:
        this.#hide(this.whenNewTargets)
        this.#hide(this.whenOneTargets)
        this.#show(this.whenManyTargets)
    }

    // Set color when annotation exists
    if (covered.length > 0) {
      this.colorTargets.forEach((e) => {
        if (e.value == covered[0].dataset.annotationColor) {
          e.checked = true
          return
        }
      })
    }

    // Get root, range and controlls coordinates
    const rangeRect = this.annotation.range.getBoundingClientRect()
    const rootRect = this.findRelativeRoot().getBoundingClientRect()

    // Controlls dimension
    const h = this.controlsTarget.clientHeight
    const w = this.controlsTarget.clientWidth

    // Default position
    let position = "ontouchstart" in document.documentElement ? "bottom" : "top"
    if (rangeRect.top + 10 < h) {
      position = "bottom"
    }
    this.controlsTarget.dataset.position = position

    // Range position relative to its root element
    const rangeTop =
      position == "top"
        ? Math.round(rangeRect.top - rootRect.top)
        : Math.round(rangeRect.top + rangeRect.height - rootRect.top)
    const rangeLeft = Math.round(rangeRect.left - rootRect.left)
    const rangeCenter = Math.round(rangeLeft + rangeRect.width / 2)

    // Set controlls position
    const y = position == "top" ? Math.round(rangeTop - h) : rangeTop
    // prettier-ignore
    const x = Math.floor(
      Math.max(
        0,
        Math.min(
          rangeCenter - w / 2,
          rootRect.width - w - 1,
        ),
      ),
    )

    this.controlsTarget.style.top = `${position == "top" ? y - 4 : y + 4}px`
    this.controlsTarget.style.left = `${x}px`

    // Set arrow position
    if (!this.hasArrowTarget) {
      return
    }
    const arrowWidth = this.arrowTarget.offsetWidth
    // prettier-ignore
    const arrowX = Math.max(
      arrowWidth / 2,
      Math.min(
        rangeCenter - x - arrowWidth / 2,
        w - arrowWidth - arrowWidth / 2,
      ),
    )
    this.arrowTarget.style.marginLeft = `${arrowX}px`
  }

  /**
   * @param {NodeListOf<Element>} nodes
   */
  #hide(nodes) {
    nodes.forEach((e) => {
      e.classList.add(this.hiddenClass)
    })
  }

  /**
   * @param {NodeListOf<Element>} nodes
   */
  #show(nodes) {
    nodes.forEach((e) => {
      e.classList.remove(this.hiddenClass)
    })
  }

  /**
   * This saves the chosen color name in the parent's node dataset.
   * We need to do this because the whole turbo-frame will be updated
   * and so will any value set to it.
   *
   * @param {String} color Color name
   */
  #registerColor(color) {
    this.element.parentElement.dataset.annotationsColor = color
  }

  /**
   * Returns the registered color from the parent node.
   *
   * @returns {String} Color name
   */
  #getRegisteredColor() {
    return this.element.parentElement.dataset.annotationsColor
  }

  /**
   * Returns the selected color.
   *
   * @returns {String} Color name
   */
  #getColor() {
    const el = this.colorTargets.find((e) => !!e.checked)
    if (!!el) {
      return el.value
    }

    return "yellow"
  }

  async nextTick() {
    return await new Promise((resolve) => setTimeout(resolve, 0))
  }

  findRelativeRoot() {
    let p = this.rootTarget
    while (p.parentElement) {
      if (getComputedStyle(p).position == "relative") {
        return p
      }
      p = p.parentElement
    }
    return p
  }

  /**
   * reload loads and replace the turbo frame content
   */
  reload = async () => {
    if (!this.element.src) {
      throw new Error("controller element must have an src attribute")
    }

    // Enable turbo frame and wait for it to be reloaded.
    this.element.disabled = false
    window.getSelection().removeAllRanges()
    await this.element.loaded
  }

  /**
   * create creates a new annotation on the document
   */
  async create() {
    this.annotation.color = this.#getColor()

    await request(this.apiUrlValue, {
      method: "POST",
      body: {
        start_selector: this.annotation.startSelector,
        start_offset: this.annotation.startOffset,
        end_selector: this.annotation.endSelector,
        end_offset: this.annotation.endOffset,
        color: this.annotation.color,
      },
    })
    await this.reload()
  }

  /**
   * update updates the selected annotations
   */
  async update() {
    const color = this.#getColor()
    const baseURL = new URL(`${this.apiUrlValue}/`, document.URL)

    await this.iterCoveredAnnotations(async (id) => {
      await request(new URL(id, baseURL), {
        method: "PATCH",
        body: {
          color: color,
        },
      })
    })
    await this.reload()
  }

  /**
   * removes the selected annotations
   */
  async delete() {
    const baseURL = new URL(`${this.apiUrlValue}/`, document.URL)
    await this.iterCoveredAnnotations(async (id) => {
      await request(new URL(id, baseURL), {method: "DELETE"})
    })
    await this.reload()
  }

  /**
   * iterCoveredAnnotations execute a function on each annotatin in the
   * current selection.
   *
   * @param {function(string): Promise<void>} fn
   */
  async iterCoveredAnnotations(fn) {
    const ids = new Set()
    this.annotation.coveredAnnotations().forEach((n) => {
      ids.add(n.dataset.annotationIdValue)
    })
    if (ids.length == 0) {
      return
    }

    for (const id of ids) {
      await fn(id)
    }
  }
}

class Annotation {
  /**
   * Annotation holds raw information about an annotation. It contains only text with
   * selectors and offsets providing needed information to find an annotation.
   *
   * @param {Node} root
   * @param {Selection} selection
   */
  constructor(root, selection) {
    /** @type {Node} */
    this.root = root

    /** @type {Selection} */
    this.selection = selection

    /** @type {Range} */
    this.range = null

    /** @type {Node[]} */
    this.textNodes = []

    /** @type {Node} */
    this.ancestor = null

    /** @type {string} */
    this.startSelector = null

    /** @type {Number} */
    this.startOffset = null

    /** @type {string} */
    this.endSelector = null

    /** @type {Number} */
    this.endOffset = null

    /** @type {String} */
    this.color = null

    this.init()
  }

  init() {
    // Selection must be a range and contains something
    if (
      this.selection.type.toLowerCase() != "range" ||
      !this.selection.toString().trim()
    ) {
      return
    }

    // Only one range
    if (this.selection.rangeCount != 1) {
      return
    }

    const range = this.selection.getRangeAt(0)
    if (range.collapsed) {
      return
    }

    // Range must be within root element boundaries
    if (!this.root.contains(range.commonAncestorContainer)) {
      return
    }

    // This handles double click on an element (in opposition to selecting text).
    // Containers can be element and we only want to deal with text nodes.
    if (range.startContainer.nodeType == Node.ELEMENT_NODE) {
      walkTextNodes(range.startContainer, (n, i) => {
        if (i == 0) {
          range.setStart(n, 0)
        }
      })
    }
    if (range.endContainer.nodeType == Node.ELEMENT_NODE) {
      let c = range.endContainer
      if (range.endOffset == 0) {
        c = range.endContainer.previousElementSibling
      }
      walkTextNodes(c, (n) => {
        range.setEnd(n, n.textContent.length)
      })
    }

    // start and end containers must be text nodes
    if (
      range.startContainer.nodeType != Node.TEXT_NODE ||
      range.endContainer.nodeType != Node.TEXT_NODE
    ) {
      return
    }

    this.range = range
    const s = getSelector(
      this.root,
      this.range.startContainer,
      this.range.startOffset,
    )
    const e = getSelector(
      this.root,
      this.range.endContainer,
      this.range.endOffset,
    )
    this.startSelector = s.selector
    this.startOffset = s.offset
    this.endSelector = e.selector
    this.endOffset = e.offset

    // Get ancestor
    if (this.range.commonAncestorContainer.nodeType == Node.TEXT_NODE) {
      this.ancestor = this.range.commonAncestorContainer.parentElement
    } else {
      this.ancestor = this.range.commonAncestorContainer
    }

    // Collect included text nodes
    let started = false
    let done = false
    walkTextNodes(this.ancestor, (n) => {
      if (done) {
        return
      }
      if (n == this.range.startContainer) {
        started = true
      }
      if (started) {
        this.textNodes.push(n)
      }
      if (n == this.range.endContainer) {
        done = true
      }
    })
  }

  /**
   * @returns {Node[]}
   */
  coveredAnnotations() {
    return this.textNodes
      .filter((n) => n.parentElement.tagName.toLowerCase() == "rd-annotation")
      .map((n) => n.parentElement)
  }

  /**
   * @returns {Boolean}
   */
  isValid() {
    return this.range != null && this.coveredAnnotations().length == 0
  }
}

/**
 * getSelector returns a CSS selector for a text node at the given offset.
 *
 * @param {Node} root
 * @param {Node} node
 * @param {Number} offset
 * @returns {{selector: string, offset: Number}}
 */
function getSelector(root, node, offset) {
  let p = node.parentElement
  const names = []

  // Get selector
  while (p.parentElement && p != root) {
    let i = 1
    let s = p
    while (s.previousElementSibling) {
      s = s.previousElementSibling
      if (s.tagName.toLowerCase() == p.tagName.toLowerCase()) {
        i++
      }
    }
    names.unshift(`${p.tagName.toLowerCase()}[${i}]`)

    p = p.parentElement
  }

  // Get offset
  let done = false
  let newOffset = 0
  walkTextNodes(node.parentElement, (n, i) => {
    if (done) {
      return
    }
    if (n == node) {
      done = true
    }
    if (!done) {
      newOffset += n.textContent.length
    } else {
      newOffset += offset
    }
  })

  return {selector: names.join("/"), offset: newOffset}
}

/**
 * walkTextNodes calls a callback function on each text node
 * found in a given node and its descendants.
 *
 * @param {Node} node
 * @param {function(Node, Number)} callback
 * @param {Number} [index]
 */
function walkTextNodes(node, callback, index) {
  index = index === undefined ? 0 : index
  for (let child = node.firstChild; child; child = child.nextSibling) {
    if (child.nodeType == Node.TEXT_NODE) {
      callback(child, index)
      index++
    }
    walkTextNodes(child, callback, index)
  }
}
