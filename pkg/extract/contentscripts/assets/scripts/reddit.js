// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "reddit.com"
}

exports.processMeta = function () {
  // Any reddit post provides a JSON payload when you add
  // a ".json" extension to it. Pretty neat.
  const url = new URL($.url)
  url.pathname += ".json"

  let rsp = requests.get(url)
  rsp.raiseForStatus()
  const data = rsp.json()
  const basePost = data[0]?.data?.children[0]?.data
  if (!basePost) {
    return
  }
  const postID = basePost.name

  const postURL = `https://gateway.reddit.com/desktopapi/v1/postcomments/${postID}`
  rsp = requests.get(postURL)
  rsp.raiseForStatus()
  const post = rsp.json().posts?.[postID]

  let html = ""

  // Get the title
  if (post.title) {
    $.title = post.title
  }

  // Get the author
  if (post.author) {
    $.author = post.author
  }

  // Get any content first
  if (post.media?.content) {
    html += post.media.content
  }

  // Check if there is a preview first
  const preview = findPreview(post)
  if (preview) {
    $.meta["x.picture_url"] = [preview]
  }

  if (post.media?.type == "image" && preview) {
    // We have a picture !
    // Since we fetched the preview first, we only need to set the bookmark type.
    $.type = "photo"
    $.description = ""
    html = ""
  } else if (post.media?.type == "gallery" && post.media?.gallery?.items) {
    // Picture gallery. We'll create an HTML content with a link to all images.
    // They'll be saved during the extraction process
    const items = post.media.gallery.items
      .map((item) => {
        const img = (
          post.media.mediaMetadata?.[item.mediaId]?.p || []
        ).findLast(() => true)
        if (!img) {
          return ""
        }
        return img.u
      })
      .filter((src) => src)

    if (items.length > 0) {
      $.meta["x.picture_url"] = items[0]
    }

    html += items
      .map((src) => {
        return `<figure><img alt="" src="${src}"></figure>`
      })
      .join("\n")
    $.readability = false
  } else if (post.media?.type == "video" && post.media?.hlsUrl && preview) {
    // Set an HLS embed when we've got an HLS URL and a preview picture
    $.type = "video"
    $.meta["oembed.html"] = `<hls src="${post.media?.hlsUrl}"></hls>`
  } else if (post.media?.type == "video") {
    // Fallback to a link to the post
    $.type = "video"
    html += `<p><a href="${post.permalink}">Original Reddit Video</a></p>`
    $.readability = false
  } else if (post.source?.url) {
    html += `<p>Link to <a href="${post.source.url}">${
      post.source.displayText || post.source.url
    }</a></p>`
  }

  if (html != "") {
    $.html = `<section id="main">${html}</section>`
  }
}

function findPreview(post) {
  // The last preview is the biggest
  let img = (post.media?.resolutions || []).findLast(() => true)
  if (!img) {
    img = post.preview
  }

  if (img) {
    return unescapeURL(img.url)
  }
}
