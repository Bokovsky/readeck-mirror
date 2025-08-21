// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
// SPDX-FileCopyrightText: © 2025 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return true
}

const rxYtEmbedded = /^\/embed\/([a-zA-Z0-9_-]+)/

/**
 *  @param {Node} document
 */
exports.documentDone = function (document) {
  // Transform span elements with an x-data-link-href
  // attribute to proper <a> elements.
  document
    .querySelectorAll("figcaption>span[x-data-link-href]")
    .forEach((e) => {
      const link = document.createElement("a")
      link.setAttribute("href", e.getAttribute("x-data-link-href"))
      link.appendChild(e.firstChild.cloneNode())
      e.replaceWith(link)
    })
}

/**
 *  @param {Node} document
 */
exports.documentReady = function (document) {
  document.querySelectorAll("iframe[src]").forEach((iframe) => {
    const src = new URL(iframe.getAttribute("src"))
    if (src.protocol != "http:" && src.protocol != "https:") {
      return
    }

    let width = parseInt(iframe.getAttribute("width")) || 0
    let height = parseInt(iframe.getAttribute("height")) || 0

    let pageURL = ""
    let imgSrc = []
    let title = ""

    switch (src.host.toLowerCase()) {
      case "www.youtube.com":
      case "youtube.com":
      case "www.youtube-nocookie.com":
        const m = rxYtEmbedded.exec(src.pathname)
        if (m == null) {
          return
        }
        const videoID = m[1]
        pageURL = "https://www.youtube.com/watch?v=" + videoID
        const start = src.searchParams.get("start")
        if (!!start) {
          pageURL += "&t=" + start + "s"
        }
        src.host = "www.youtube-nocookie.com"
        imgSrc.push(
          "https://i.ytimg.com/vi/" + videoID + "/maxresdefault.jpg",
          "https://i.ytimg.com/vi/" + videoID + "/hqdefault.jpg",
        )

        try {
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
          const info = rsp.json()
          if (!!info.videoDetails?.title) {
            title = `Youtube - ${info.videoDetails.title}`
          }
          if (!!info.microformat?.playerMicroformatRenderer?.embed) {
            width = info.microformat.playerMicroformatRenderer.embed.width
            height = info.microformat.playerMicroformatRenderer.embed.height
          }
        } catch (e) {
          //
        }

        break
    }

    if (pageURL == "") {
      return
    }

    console.debug("embedded video", { src, pageURL, imgSrc })

    const figure = document.createElement("figure")
    figure.setAttribute("class", "content")

    // Link is a span so readability doesn remove the whole
    // block because it only contains a link.
    const link = document.createElement("span")
    link.setAttribute("x-data-link-href", pageURL)

    const params = new URLSearchParams()
    params.set("src", src.toString())
    params.set("w", width.toString())
    params.set("h", height.toString())

    if (title != "") {
      link.appendChild(document.createTextNode(title))
    } else {
      link.appendChild(document.createTextNode(pageURL))
    }

    const img = document.createElement("img")
    img.setAttribute("srcset", imgSrc.join(", "))
    img.setAttribute("alt", title)
    img.setAttribute("x-data-iframe-params", params.toString())

    const caption = document.createElement("figcaption")
    caption.appendChild(link)

    figure.appendChild(img)
    figure.appendChild(caption)

    iframe.replaceWith(figure)
  })
}
