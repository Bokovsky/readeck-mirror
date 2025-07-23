// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return /\.(wikinews|wikipedia)\.org$/.test($.host)
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  // Force override for wikinews
  $.overrideConfig(config, "https://wikipedia.org/")

  // Add a content selector
  config.bodySelectors.unshift("//div[@id = 'mw-content-text']")

  // Some more filters
  config.stripSelectors.push(
    "//*[contains(@class, 'mw-editsection')]",
    "//*[contains(@class, 'printfooter')]",
    "//*[contains(@class, 'mw-indicators')]",
    "//*[contains(@class, 'navbox')]",
    "//*[contains(@class, 'navbox-styles')]",
    "//*[contains(@class, 'side-box')]",
    "//div[@id='mw-content-text']/noscript",
  )

  if (new URL($.url).pathname.startsWith("/wiki/")) {
    // A wikipedia page is more or less clean and we can
    // disable readability, which tends to remove way too much
    // data from the content.
    $.readability = false
  }
}
