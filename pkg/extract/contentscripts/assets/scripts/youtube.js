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
  /** @type {string} */
  let videoID = ($.properties["json-ld"] || []).find(
    // @ts-ignore
    (x) => x["@type"] == "VideoObject" && !!x.identifier,
  )?.identifier

  if (!videoID && $.meta["graph.url"]?.length) {
    const videoURL = new URL($.meta["graph.url"][0], $.url)
    videoID = videoURL.searchParams.get("v")
  }

  if (!videoID) {
    console.warn("could not determine video ID")
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
    $.meta["x.duration"] = [lengthSeconds]
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

/**
 *
 * @param {string} videoID Video ID
 * @returns {VideoInfo}
 */
function getVideoInfo(videoID) {
  let rsp = requests.post(
    "https://www.youtube.com/youtubei/v1/player",
    JSON.stringify({
      context: {
        client: {
          hl: "en",
          clientName: "ANDROID",
          clientVersion: "20.10.38",
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

/**
 *
 * @param {VideoInfo} info
 * @returns
 */
function getTranscript(info) {
  const langPriority = ["en", undefined, null, ""]

  // Fetch caption list
  let captions =
    info.captions?.playerCaptionsTracklistRenderer?.captionTracks || []
  captions = captions.map((x) => {
    x.auto = x.kind == "asr" ? 1 : 0
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

  const doc = new DOMParser().parseFromString(rsp.text(), "text/html")
  return doc
    .querySelectorAll("p")
    .map((n) => {
      return n.textContent.trim()
    })
    .filter((x) => x)
}

/**
 *
 * @param {string} text Text to convert
 * @returns {string}
 */
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

/**
 * @typedef {{
 *  videoDetails?: {
 *    shortDescription: string,
 *    lengthSeconds: string,
 *  },
 *  captions?: {
 *    playerCaptionsTracklistRenderer?: {
 *      captionTracks: Array<{
 *        kind?: string,
 *        auto?: number,
 *        languageCode: string,
 *        baseUrl: string,
 *      }>,
 *      audioTracks: Array<{
 *        hasDefaultTrack: boolean,
 *        defaultCaptionTrackIndex: number,
 *      }>
 *    }
 * }
 * }} VideoInfo
 */
