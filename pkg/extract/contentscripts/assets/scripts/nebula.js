// SPDX-FileCopyrightText: © 2026 Xavier Vello <xavier.vello@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
    return $.domain === "nebula.tv" && /^\/videos\//.test(new URL($.url).pathname)
}

exports.processMeta = function () {
    try {
        processContentAPI()
    } catch (e) {
        console.warn("Failed to query content API, falling back to html metadata:", e.message)

        if (!!$.meta["oembed.title"]) {
            $.title = $.meta["oembed.title"]
        }
        for (let meta of $.properties.meta) {
            if (meta["@property"] == "video:duration" && !!meta["@content"]) {
                $.meta["x.duration"] = meta["@content"]
                continue
            }
            if (meta["@property"] == "video:release_date" && !!meta["@content"]) {
                $.meta["html.date"] = meta["@content"]
            }
        }
    }
}

/**
 * Queries the content API, available without authentication.
 * Function will throw if any field is missing to trigger the fallback to html metadata.
 */
function processContentAPI() {
    const video_path = new URL($.url).pathname
    let url = `https://content.api.nebula.app/content${video_path}`
    const rsp = requests.get(url)
    rsp.raiseForStatus()

    const {
        title = (() => { throw new Error('Missing title') })(),
        description = (() => { throw new Error('Missing description') })(),
        duration = (() => { throw new Error('Missing duration') })(),
        published_at = (() => { throw new Error('Missing published_at') })()
    } = rsp.json()

    $.meta["x.duration"] = String(duration)
    $.meta["html.date"] = published_at
    $.title = title
    $.type = "video"
    $.html = `<section id="main">${convertDescription(description)}</section>`
    $.readability = false
}

/**
 * Copied from youtube.js without modification.
 * @param text
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
