// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "substack.com"
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  config.bodySelectors = ["//div[contains(@class,'available-content')]"]
  config.stripIdOrClass.push("image-link-expand")
}

exports.processMeta = function () {
  // Get the site name from json-ld
  if ($.properties["json-ld"]) {
    const site = $.properties["json-ld"][0].publisher?.name
    if (site) {
      $.site = site
    }
  }
}
