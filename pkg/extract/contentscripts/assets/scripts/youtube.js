// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "youtube.com"
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  // there's no need for custom headers, on the contrary
  config.httpHeaders = {}
}

exports.processMeta = function () {
  const videoID = ($.properties["json-ld"] || []).find(
    (x) => x["@type"] == "VideoObject" && !!x.identifier,
  ).identifier
  if (!videoID) {
    return
  }

  const info = getVideoInfo(videoID)
  let html = ""

  // Get a better description
  const description = (info.videoDetails?.shortDescription || "").trim()
  if (description) {
    $.description = description.split("\n")[0]
    if (description.length > $.description.length) {
      html += convertDescription(description)
    }
  }

  // Get more information
  const lengthSeconds = info.videoDetails?.lengthSeconds
  if (lengthSeconds) {
    $.meta["x.duration"] = lengthSeconds
  }

  // Get transcript
  const transcript = getTranscript(info)
  if (transcript) {
    html += `<h2>Transcript</h2>\n<p>${transcript.join("<br>\n")}</p>`
  }

  if (html) {
    $.html = `<section id="main">${html}</section>`
    // we must force readability here for it to pick up the content
    // (it normally won't with a video)
    $.readability = false
  }
}

function getVideoInfo(videoID) {
  let rsp = requests.post(
    "https://youtubei.googleapis.com/youtubei/v1/player",
    JSON.stringify({
      context: {
        client: {
          hl: "en",
          clientName: "WEB",
          clientVersion: "2.20210721.00.00",
          mainAppWebInfo: {
            graftUrl: "/watch?v=" + videoID,
          },
        },
      },
      videoId: videoID,
    }),
    {
      "Content-Type": "application/json",
    },
  )
  rsp.raiseForStatus()
  return rsp.json()
}

function getTranscript(info) {
  const langPriority = ["en", undefined, null, ""]

  // Fetch caption list
  let captions =
    info.captions?.playerCaptionsTracklistRenderer?.captionTracks || []
  captions = captions.map((x) => {
    x.auto = x.kind == "asr"
    return x
  })

  // Look for a default track
  let trackIdx =
    info.captions?.playerCaptionsTracklistRenderer?.audioTracks?.find(
      (x) => x.hasDefaultTrack,
    )?.defaultCaptionTrackIndex

  let track
  if (trackIdx !== null) {
    // If we have a default track, we take this one.
    track = captions[trackIdx]
  }

  if (track === undefined && captions.length > 0) {
    // If we couldn't find a transcript by index,
    // we sort the list by automatic caption last and language code priorities.
    captions.sort((a, b) => {
      return (
        a.auto - b.auto ||
        langPriority.indexOf(b.languageCode) -
          langPriority.indexOf(a.languageCode)
      )
    })

    track = (captions || []).find(() => true)
  }

  if (!track) {
    return
  }

  console.debug("found transcript", { track })

  const rsp = requests.get(track.baseUrl)
  rsp.raiseForStatus()

  return (decodeXML(rsp.text()).transcript?.text || [])
    .map((x) => {
      return x["#text"]
    })
    .filter((x) => x)
}

function convertDescription(text) {
  text = text.replace(/\n\n/g, "</p><p>")
  text = text.replace(/\n/g, "<br>\n")
  text = text.replace("</p>", "</p>\n")
  text = text.replace(
    /(https?:\/\/[-A-Z0-9+&@#\/%?=~_|!:,.;]*[-A-Z0-9+&@#\/%=~_|])/gi,
    '<a href="$1">$1</a>',
  )

  return `<p class="main">${text}</p>`
}
