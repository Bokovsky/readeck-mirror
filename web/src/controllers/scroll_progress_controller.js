// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["tracked", "indicator", "trigger", "value", "anchor"]
  static values = {
    autoMarkAfter: {
      type: Number,
      default: 0,
    },
    trackInterval: {
      type: Number,
      default: 100,
    },
    notifyInterval: {
      type: Number,
      default: 1500,
    },
  }

  connect() {
    this.current = undefined
    if (this.hasValueTarget && this.hasAnchorTarget) {
      this.current = {
        p: parseInt(this.valueTarget.value, 10) || 0,
        s: String(this.anchorTarget.value),
      }
    }

    // skipEvent holds a boolean that is reset to false the first time
    // the user scrolls the page.
    // This helps preventing sending a progress event when we don't want to
    // - loading a page with a hash in its URL
    // - navigating inside the page
    // - first time we restore the progress value
    this.skipEvent = false
    window.addEventListener("hashchange", () => {
      // We don't want to save a position when navigating inside the page
      this.skipEvent = true
    })

    // scroll to the initial position if defined and if the location hash is empty
    if (document.location.hash == "" && this.current !== undefined) {
      this.scrollTo(this.current.p, this.current.s)
    }

    // Update the indicator in real time
    if (this.hasIndicatorTarget) {
      this.skipEvent = true
      this.indicatorUpdater()
    }

    // No tracked element. If autoMarkAfter is > 0 we mark as read after
    // a delay.
    if (
      !this.hasTrackedTarget &&
      this.autoMarkAfterValue > 0 &&
      this.current.p != 100
    ) {
      setTimeout(() => {
        if (this.current.p != 100) {
          this.current = {p: 100, s: ""}
          this.updatePositionTargets()
        }
      }, this.autoMarkAfterValue * 1000)
      return
    }
  }

  skip() {
    this.skipEvent = true
  }

  trackedTargetConnected() {
    this.skipEvent = true // document.location.hash != ""

    if (!this.hasValueTarget || !this.hasAnchorTarget) {
      return
    }

    // Start the progress tracker
    let scrollTimeout = null
    let notifyTimeout = null

    this.progressTracker((el) => {
      if (el === undefined) {
        return
      }

      if (scrollTimeout !== null) {
        clearTimeout(scrollTimeout)
        scrollTimeout = null
      }

      scrollTimeout = setTimeout(() => {
        scrollTimeout = null
        if (this.skipEvent) {
          this.skipEvent = false
          return
        }

        const value = {
          p: toPercent(this.viewedPercentage),
          s: "",
        }

        // Only set a selector for percentage between 1 and 99 included.
        if (value.p > 0 && value.p < 100) {
          value.s = getSelector(this.trackedTarget, el)
        }

        if (this.current.p == value.p && this.current.s == value.s) {
          // Stop when no change
          return
        }

        this.current = {...value}

        // Second debounce for value updates and notification
        if (notifyTimeout !== null) {
          clearTimeout(notifyTimeout)
          notifyTimeout = null
        }
        notifyTimeout = setTimeout(() => {
          notifyTimeout = null
          this.updatePositionTargets()
        }, this.notifyIntervalValue)
      }, this.trackIntervalValue)
    })
  }

  /**
   * setPosition is an event handler that sets the scroll value.
   * It can receive a param "position" and "notify". The later, if false
   * prevents sending the "progress" event.
   *
   * @param {Event} evt Event
   */
  setPosition(evt) {
    const data = {...this.current}
    if (evt.params.position !== undefined) {
      data.p = parseInt(evt.params.position) || 0
    }

    if (data.p == this.current.p && data.s == this.current.s) {
      return
    }

    this.current = {...data}
    let notify = true
    if (evt.params.notify !== undefined) {
      notify = !!evt.params.notify
    }
    this.updatePositionTargets(notify)
  }

  /**
   * scroll is an event handler that scrolls the document to
   * the currently set position, whithout triggering the observer.
   *
   * @param {Event} evt Event
   */
  scroll(evt) {
    this.skipEvent = true
    this.scrollTo(this.current.p, this.current.s)
  }

  /**
   * updatePositionTargets update the "value" and "anchor" target values
   * and dispatch the progress event to every trigger.
   */
  updatePositionTargets(notify = true) {
    if (this.hasValueTarget) {
      this.valueTarget.value = this.current.p
    }
    if (this.hasAnchorTarget) {
      this.anchorTarget.value = this.current.s
    }

    if (!notify) {
      return
    }

    this.triggerTargets.forEach((t) => {
      this.dispatch("progress", {target: t})
    })
  }

  /**
   * scrollTo scrolls the page to a position and/or an anchor.
   * If the position is 0 or 100, it scrolls directly to top or bottom
   * and bypasses any anchor value.
   * anchor is a CSS selector, if the element is found in the "tracked"
   * target, we scroll it into the viewport.
   *
   * @param {number} position scrolled percentage
   * @param {string} anchor CSS selector
   */
  scrollTo(position, anchor) {
    if (!this.hasTrackedTarget) {
      return
    }

    position = parseInt(position, 10)
    if (isNaN(position)) {
      position = 0
    }

    // 0%, scroll to top
    if (position == 0) {
      window.scroll({top: 0, behavior: "instant"})
      return
    }

    // 100%, scroll to the end
    if (position == 100) {
      window.scroll({
        top: document.body.parentNode.scrollHeight,
        behavior: "instant",
      })
      return
    }

    // Anything in between, scroll to the element
    try {
      const e = this.trackedTarget.querySelector(anchor)
      if (e === null) {
        return
      }
      e.scrollIntoView({
        behavior: "instant",
        block: "center",
      })
    } catch (_) {}
  }

  /**
   * indicatorUpdater listens for scroll events and updates the width of
   * the indicator target.
   */
  indicatorUpdater() {
    // Set initial position
    this.indicatorTarget.value = toPercent(this.viewedPercentage)

    // Start listing for document scroll
    let ticking = false
    document.addEventListener(
      "scroll",
      () => {
        if (!ticking) {
          window.requestAnimationFrame(() => {
            this.indicatorTarget.value = toPercent(this.viewedPercentage)
            ticking = false
          })

          ticking = true
        }
      },
      {
        passive: true,
      },
    )
  }

  /**
   * progressTracker listens to intersections of all elements in a small window
   * on the bottom half of the screen and triggers a callback.
   *
   * @param {function(Element)} callback Tracking callback
   */
  progressTracker(callback) {
    // Get elements has they enter a small window a bit after the middle of the screen.
    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries.find((x) => x.isIntersecting)
        callback(entry !== undefined ? entry.target : undefined)
      },
      {
        // The intersection window is somewhere on the bottom half of the screen.
        rootMargin: "-65% 0px -25% 0px",
      },
    )
    for (let e of this.trackedTarget.getElementsByTagName("*")) {
      if (
        e.children.length == 0 &&
        e.tagName.toLowerCase() !== "rd-annotation"
      ) {
        observer.observe(e)
      }
    }
  }

  /**
   * position returns the current scrolled position.
   *
   * @returns {number} position (from 0 to 100)
   */
  get viewedPercentage() {
    const p = document.body.parentNode

    if (p.scrollHeight - p.clientHeight <= 0) {
      return 100
    }
    return (p.scrollTop / (p.scrollHeight - p.clientHeight)).toFixed(3)
  }
}

/**
 * toPercent convert a floating value to a percentage.
 *
 * @param {number} value value to convert
 * @returns
 */
function toPercent(value) {
  return Math.round(Math.min(value, 1) * 100)
}

/**
 * getSelector returns a CSS selector for an element.
 *
 * @param {Node} root
 * @param {Node} node
 * @returns {string} CSS selector
 */
function getSelector(root, node) {
  let p = node
  const names = []

  while (p.parentElement && p != root) {
    let i = 1
    let s = p
    while (s.previousElementSibling) {
      i++
      s = s.previousElementSibling
    }
    names.unshift(`${p.tagName.toLowerCase()}:nth-child(${i})`)
    p = p.parentElement
  }

  return names.join(">")
}
