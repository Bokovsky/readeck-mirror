// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "ted.com" && new URL($.url).pathname.match(/^\/talks\//)
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
