// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return (
    $.domain == "ted.com" &&
    new URL($.url).pathname.match(/^\/(talks|dubbing)\//) !== null
  )
}

/**
 * @param {Node} document
 */
exports.documentLoaded = function (document) {
  // TED talks can exist on /dubbing/... paths and expose
  // a broken oembed link.
  // This fixes the link to oembed with "/talks/..." path.
  document
    .querySelectorAll("link[href][type='application/json+oembed']")
    .forEach((e) => {
      const url = new URL(e.getAttribute("href"))
      const p = new URL(url.searchParams.get("url"))
      p.pathname = p.pathname.replace(/^\/dubbing\//, "/talks/")
      url.searchParams.set("url", p.toString())
      e.setAttribute("href", url.toString())
    })
}

exports.processMeta = function () {
  // set duration
  $.meta["x.duration"] = $.meta["graph.video:duration"]

  // get transcript
  const transcript = getTranscript()
  if (transcript) {
    console.debug("found transcript", { paragraphs: transcript.length })
    $.html = `<section id="main">${transcript
      .map((p) => {
        return `<p>${p}</p>`
      })
      .join("\n")}</section>`
    $.readability = true
  }
}

function getTranscript() {
  const data = ($.properties.json || [])
    .map((x) => {
      return x.props?.pageProps?.transcriptData?.translation?.paragraphs
    })
    .find((x) => {
      return x != undefined
    })

  if (data) {
    return data.map((p) => {
      return (p.cues || [])
        .map((cue) => {
          return cue.text
        })
        .join(" ")
    })
  }

  return []
}
