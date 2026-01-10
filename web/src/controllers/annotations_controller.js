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
    "note",
    "arrow",
    "whenNew",
    "whenOne",
    "whenMany",
  ]
  static classes = ["hidden"]
  static values = {
    apiUrl: String,
    color: String,
  }

  connect() {
    this.annotation = null
    this.payload = null
    this.currentIDs = []

    let ticking = false
    let t = null

    this.colorTargets.forEach((e, i) => {
      e.addEventListener("click", (evt) => {
        this.colorValue = evt.target.value
      })
    })

    const x = new ResizeObserver((entries) => {
      for (let e of entries) {
        if (getComputedStyle(e.target).display == "none") {
          return
        }
        this.#positionControls()
      }
    })
    x.observe(this.controlsTarget)

    document.addEventListener("selectionchange", async (evt) => {
      if (!ticking) {
        window.requestAnimationFrame(async () => {
          if (t !== null) {
            window.clearTimeout(t)
          }
          t = window.setTimeout(async () => {
            ticking = false
            await this.onSelectText(evt)
          }, 100)
        })

        ticking = true
      }
    })
  }

  colorValueChanged(value, previous) {
    // Initial value
    if (value === "" && previous === undefined) {
      this.colorTargets[0].checked = true
      this.colorValue = this.colorTargets[0].value
      return
    }

    // When the frame is restored
    if (value === "" && !!previous) {
      this.colorTargets.forEach((e) => {
        if (e.value == previous) {
          e.checked = true
          return
        }
      })
      this.colorValue = previous
      return
    }
  }

  /**
   * onSelectText is the listener for "selectionchange".
   */
  async onSelectText() {
    const selection = document.getSelection()
    this.annotation = new Annotation(this.rootTarget, selection)

    if (
      this.annotation.isValid() ||
      this.annotation.coveredAnnotations().length > 0
    ) {
      await this.showControls()
    }

    if (this.annotation.isValid()) {
      this.payload = {
        start_selector: this.annotation.startSelector,
        start_offset: this.annotation.startOffset,
        end_selector: this.annotation.endSelector,
        end_offset: this.annotation.endOffset,
      }
    }
  }

  async showControls() {
    this.currentIDs = []
    this.iterCoveredAnnotations(async (id) => {
      this.currentIDs.push(id)
    })

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
    const selected = [
      ...new Set(covered.map((e) => e.dataset.annotationIdValue)),
    ]

    // Show/hide sub controls
    switch (selected.length) {
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

    // Set color and text when annotation exists
    switch (selected.length) {
      case 0:
        this.noteTarget.value = ""
        break
      case 1:
        // One selected annotation, check if it has a note
        const e = Array.from(
          this.rootTarget.querySelectorAll(
            "rd-annotation[data-annotation-note][title]",
          ),
        ).find((e) => e.dataset.annotationIdValue == selected[0])

        // set the note and fall through the next case (sets color)
        this.noteTarget.value = e !== undefined ? e.getAttribute("title") : ""
      default:
        this.colorTargets.forEach((e) => {
          if (e.value == covered[0].dataset.annotationColor) {
            e.checked = true
            this.colorValue = e.value
            return
          }
        })
    }

    this.#positionControls()
  }

  #positionControls() {
    // Get root, range and controlls coordinates
    const rangeRect = this.annotation.range.getBoundingClientRect()
    const rootRect = this.findRelativeRoot().getBoundingClientRect()

    switch (getComputedStyle(this.controlsTarget).position) {
      case "absolute":
        // Controlls dimension
        const h = this.controlsTarget.clientHeight
        const w = this.controlsTarget.clientWidth

        // Default position
        let position = "top"
        if (rangeRect.top - 20 < h) {
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
        break
      case "fixed":
        // On a fixed position, try not to cover the selection
        this.controlsTarget.style.bottom = "0"
        this.controlsTarget.style.top = "unset"
        if (rangeRect.top > this.controlsTarget.getBoundingClientRect().top) {
          this.controlsTarget.style.bottom = "unset"
          this.controlsTarget.style.top = "0"
        }
    }
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

  #getNote() {
    if (!this.hasNoteTarget) {
      return
    }
    return this.noteTarget.value
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
    if (!this.payload) {
      return
    }

    const body = {
      ...this.payload,
      color: this.colorValue,
      note: this.#getNote(),
    }

    await request(this.apiUrlValue, {
      method: "POST",
      body: body,
    })
    await this.reload()
  }

  /**
   * update updates the selected annotations
   */
  async update() {
    if (this.currentIDs.length == 0) {
      return
    }

    // Common body
    const body = {color: this.colorValue}

    // Can only change the note when there's one updated item
    if (this.currentIDs.length == 1) {
      body.note = this.#getNote()
    }

    const baseURL = new URL(`${this.apiUrlValue}/`, document.URL)

    for (let id of this.currentIDs) {
      await request(new URL(id, baseURL), {
        method: "PATCH",
        body: body,
      })
    }
    await this.reload()
  }

  /**
   * removes the selected annotations
   */
  async delete() {
    const baseURL = new URL(`${this.apiUrlValue}/`, document.URL)

    for (let id of this.currentIDs) {
      await request(new URL(id, baseURL), {method: "DELETE"})
    }
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
