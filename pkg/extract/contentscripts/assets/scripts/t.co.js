// SPDX-FileCopyrightText: © 2024 Joachim Robert <joachim.robert@proton.me>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "t.co"
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  config.httpHeaders["User-Agent"] = "curl/7.0"
}
