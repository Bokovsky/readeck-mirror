// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "instagram.com" && /^\/p\//.test(new URL($.url).pathname)
}

exports.processMeta = function () {
  $.type = "photo"

  if ($.meta["twitter.title"].length > 0) {
    $.title = $.meta["twitter.title"][0].replace(/ • instagram.*$/i, "")
  }

  if ($.meta["html.title"].length > 0) {
    $.description = $.meta["html.title"][0]
  }
}
