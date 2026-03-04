// SPDX-FileCopyrightText: Copyright 2020 Saul Hardman <hello@iamsaul.co.uk>
// SPDX-License-Identifier: MIT
// SPDX-PackageHomePage: https://github.com/saulhardman/postcss-hover-media-feature
// SPDX-FileContributor: Modified by Olivier Meunier

"use strict"

const selectorParser = require("postcss-selector-parser")

const selectorProcessor = selectorParser((selectors) => {
  let hoverSelectors = []

  selectors.walk((selector) => {
    if (
      selector.type === "pseudo" &&
      selector.toString() === ":hover" &&
      selector.parent.value !== ":not" &&
      selector.parent.toString() !== ":hover"
    ) {
      hoverSelectors.push(selector.parent.toString())
    }
  })

  let nonHoverSelectors = selectors.reduce((acc, selector) => {
    if (hoverSelectors.includes(selector.toString())) {
      return acc
    }

    return [...acc, selector.toString()]
  }, [])

  return {hoverSelectors, nonHoverSelectors}
})

function isAlreadyNested(rule) {
  let container = rule.parent

  while (container !== null && container.type !== "root") {
    if (
      container.type === "atrule" &&
      container.params.includes("hover: hover")
    ) {
      return true
    }

    container = container.parent
  }

  return false
}

function createMediaQuery(rule, {AtRule}) {
  let media = new AtRule({name: "media", params: "(hover: hover)"})

  media.source = rule.source

  media.append(rule)

  return media
}

const plugin = () => {
  return {
    postcssPlugin: "hover-media-query",

    Rule(rule, {AtRule}) {
      if (!rule.selector.includes(":hover") || isAlreadyNested(rule)) {
        return
      }

      let {hoverSelectors = [], nonHoverSelectors = []} =
        selectorProcessor.transformSync(rule.selector, {lossless: false})

      if (hoverSelectors.length === 0) {
        return
      }

      let mediaQuery = createMediaQuery(
        rule.clone({selectors: hoverSelectors}),
        {AtRule},
      )

      rule.after(mediaQuery)

      if (nonHoverSelectors.length > 0) {
        rule.replaceWith(rule.clone({selectors: nonHoverSelectors}))

        return
      }

      rule.remove()
    },
  }
}

plugin.postcss = true
module.exports = plugin
