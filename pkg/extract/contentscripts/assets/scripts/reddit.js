// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Subset of data received by reddit JSON endpoint.
 *
 * @typedef {{
 *  author: string,
 *  created_utc: number,
 *  domain: string,
 *  is_self: boolean,
 *  selftext_html: string,
 *  title: string,
 *  url: string,
 *  gallery_data?: {
 *    items: Array<{
 *      id: number,
 *      media_id: string,
 *    }>,
 *  },
 *  media?: {
 *    reddit_video?: {
 *      duration: number,
 *      hls_url: string,
 *    }
 *  },
 *  media_metadata?: Record<string, {
 *    e: string,
 *    s: {
 *      u: string,
 *      x: number,
 *      y: number,
 *    },
 *  }>,
 *  preview?: {
 *   images: Array<{
 *    id: string,
 *    source: {
 *     url: string,
 *    },
 *   }>
 *  },
 * }} redditData
 */

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "reddit.com" && !!new URL($.url).pathname.match(/^\/r\//)
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  // Discard the reddit site config
  $.overrideConfig(config, "")
}

exports.documentDone = () => {
  $.site = "Reddit"
}

/**
 * @param {Node} document
 */
exports.documentLoaded = (document) => {
  const url = new URL($.url)
  url.pathname += ".json"
  const rsp = requests.get(url)
  rsp.raiseForStatus()

  /** @type {redditData} */
  const data = rsp.json()[0]?.data?.children[0]?.data

  $.properties["json"] = data

  // Get the title
  if (data.title) {
    $.title = data.title
    $.meta["graph.title"] = [$.title]
  }

  // Get the author
  if (data.author) {
    $.authors = [data.author]
  }

  // Set date
  if (data.created_utc) {
    $.meta["html.date"] = [new Date(data.created_utc * 1000).toISOString()]
  }

  // Load HTML content first
  let html = "<!-- -->"
  if (data.selftext_html) {
    html = unescapeHTML(data.selftext_html)
  }

  // Get preview image
  const preview = findPreview(data)
  if (preview) {
    $.meta["x.picture_url"] = [unescapeHTML(preview)]
  }

  if (data.domain == "i.redd.it") {
    // Single image
    // https://www.reddit.com/r/catstairs/comments/1nhmpt9/a_pet_as_you_pass_pls/
    // https://www.reddit.com/r/EarthPorn/comments/1nj04yr/golden_layers_southern_californiaoc_3000x1839/
    $.type = "photo"
    $.meta["x.picture_url"] = [data.url]
  } else if (data.media?.reddit_video?.hls_url) {
    // Video
    // https://www.reddit.com/r/cats/comments/1nid3kk/michelle/
    // https://www.reddit.com/r/nextfuckinglevel/comments/1nidkkb/incredible_pirates_of_the_caribbean_parade_float/
    $.type = "video"
    $.meta["oembed.html"] = [
      `<hls src="${data.media.reddit_video.hls_url}"></hls>`,
    ]
    $.meta["x.duration"] = [String(data.media.reddit_video.duration)]
  } else if (!!data.gallery_data?.items) {
    // Image gallery
    // https://www.reddit.com/r/pics/comments/1nciozc/oc_leaflets_ordering_residents_to_immediately/
    // https://www.reddit.com/r/cats/comments/1neucd9/i_made_bio_sheets_for_our_catsitter/
    // https://www.reddit.com/r/cats/comments/1m8fhyq/is_my_cat_okay/
    const images = data.gallery_data.items
      .map((item) => {
        const img = data.media_metadata?.[item.media_id]?.s?.u
        return img
      })
      .filter((item) => !!item)

    for (let img of images) {
      html += `\n<figure><img src="${img}" alt=""></figure>`
    }

    if (images.length > 0) {
      $.meta["x.picture_url"] = [unescapeHTML(images[0])]
    }
  } else if (!data.is_self) {
    // Not a self post, get the link
    // https://www.reddit.com/r/UplifitingNews/comments/8mh3pt/dog_adopts_nine_orphaned_ducklings/
    html += `<p>Link to <a href="${data.url}">${data.url}</a></p>`
  }

  document.body.innerHTML = html
  $.readability = false
}

/**
 *
 * @param {redditData} data
 * @returns {string | undefined}
 */
function findPreview(data) {
  if (data.preview?.images[0]?.source?.url) {
    return unescapeHTML(data.preview?.images[0]?.source?.url)
  }

  if (data.media_metadata) {
    const img = Object.entries(data.media_metadata).find(([_, o]) => {
      return !!o.e && o.e.toLowerCase() == "image" && !!o.s?.u
    })[1].s.u
    if (img) {
      return unescapeHTML(img)
    }
  }
}
