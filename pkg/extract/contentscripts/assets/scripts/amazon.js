// SPDX-FileCopyrightText: © 2025 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  let host = $.host
  if ($.host.startsWith("www.")) {
    host = host.substring(4)
  }
  return host == "amazon.ae" ||
    host == "amazon.ca" ||
    host == "amazon.cn" ||
    host == "amazon.co.jp" ||
    host == "amazon.co.uk" ||
    host == "amazon.com.au" ||
    host == "amazon.com.be" ||
    host == "amazon.com.br" ||
    host == "amazon.com.mx" ||
    host == "amazon.com.tr" ||
    host == "amazon.de" ||
    host == "amazon.eg" ||
    host == "amazon.es" ||
    host == "amazon.fr" ||
    host == "amazon.in" ||
    host == "amazon.it" ||
    host == "amazon.nl" ||
    host == "amazon.pl" ||
    host == "amazon.sa" ||
    host == "amazon.se" ||
    host == "amazon.sg"
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  $.overrideConfig(config, "https://amazon.com/")
}
