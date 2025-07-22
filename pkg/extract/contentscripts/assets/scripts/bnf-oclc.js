// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.host.endsWith(".bnf.idm.oclc.org")
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  const targetSite = /(.+)\.bnf\.idm\.oclc\.org$/.exec($.host)[1]

  switch (targetSite) {
    case "www-mediapart-fr":
      $.overrideConfig(config, "https://mediapart.fr/")
      break
    case "www-arretsurimages-net":
      $.overrideConfig(config, "https://arretsurimages.net")
      break
  }
}
