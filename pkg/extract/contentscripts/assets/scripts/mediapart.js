// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.isActive = function () {
  return (
    $.domain == "mediapart.fr" || $.host == "www-mediapart-fr.bnf.idm.oclc.org"
  )
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  config.bodySelectors.push(
    "//div[contains(concat(' ',normalize-space(@class),' '),' content-article ')]",
  )
  config.stripIdOrClass.push("cookie-consent-banner-content")
}

/**
 *
 * @param {Node} document
 */
exports.documentLoaded = function (document) {
  // https://codeberg.org/readeck/readeck/issues/1068
  document.querySelectorAll(".dropcap-wrapper>span").forEach((n) => {
    if (n.hasAttribute("aria-hidden")) {
      n.parentNode.removeChild(n)
      return
    }
    if (n.hasAttribute("class")) {
      n.setAttribute(
        "class",
        n.getAttribute("class").replace("screen-reader-only", ""),
      )
    }
  })
}
