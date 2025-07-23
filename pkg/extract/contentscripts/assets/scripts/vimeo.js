// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

const vttTimeStamp = /^\d+:\d+:\d+\.\d+\s+-->\s+\d+:\d+:\d+\.\d+/
const rxAutogen = /-x-autogen/
const rxDuration = /^PT(\d+)H(\d+)M(\d+)S$/

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "vimeo.com"
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  config.singlePageLinkSelectors = []
}

exports.processMeta = function () {
  const videoId = new URL($.url).pathname.replace(/^\/(\d+)$/, "$1")
  const info = getPlayerInfo(videoId)

  // Get thumbnail
  const size = Object.keys(info.video?.thumbs || []).find((x) => true)
  if (size) {
    $.meta["x.picture_url"] = info.video.thumbs[size]
  }

  // Get duration
  const duration = getDuration()
  if (duration) {
    $.meta["x.duration"] = String(duration)
  }

  // Fetch transcript
  const track = getTextTrack(info)
  if (track.length > 0) {
    $.html = `<section id="main"><p>${track.join("<br>\n")}</p></section>`
    $.readability = true
  }
}

function getPlayerInfo(videoId) {
  const rsp = requests.get(`https://player.vimeo.com/video/${videoId}/config`)
  rsp.raiseForStatus()
  return rsp.json()
}

function getDuration() {
  const duration = $.properties["json-ld"]?.[0]?.[0]?.duration
  const m = duration.match(rxDuration)
  if (m) {
    return parseInt(m[1]) * 3600 + parseInt(m[2]) * 60 + parseInt(m[3])
  }
  return null
}

function getTextTrack(info) {
  let tracks = info.request?.text_tracks
  if (!tracks || tracks.length == 0) {
    return []
  }

  tracks.sort((a, b) => {
    if (b.lang.match(rxAutogen) && !a.lang.match(rxAutogen)) {
      return -1
    } else if (a.lang.match(rxAutogen) && !b.lang.match(rxAutogen)) {
      return 1
    }
    return 0
  })

  let url = `https://vimeo.com${tracks.find(() => true).url}`
  const rsp = requests.get(url)
  rsp.raiseForStatus()
  return parseVtt(rsp.text())
}

function parseVtt(text) {
  const lines = text.split(/\r?\n/)
  const res = []
  let acc = null

  for (let line of lines) {
    if (line.match(vttTimeStamp) !== null) {
      acc = []
    } else if (line.trim() == "" && acc !== null) {
      res.push(acc)
      acc = null
    } else if (acc !== null) {
      acc.push(line)
    }
  }

  return res.map((x) => x.join("\n"))
}
