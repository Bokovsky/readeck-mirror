// SPDX-FileCopyrightText: © 2024 Taniki [https://11d.im/]
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "404media.co"
}

exports.processMeta = function () {
  $.authors = $.meta["twitter.data1"]
}
