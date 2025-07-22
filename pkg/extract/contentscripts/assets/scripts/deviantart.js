// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "deviantart.com"
}

exports.processMeta = function () {
  // There's a JSON URL in the collected link tags
  let link = ($.meta["link.alternate"] || []).find(function (x) {
    return x.match(/format=json$/)
  })

  if (!link) {
    return
  }

  // This link is weirdly encoded, let's fix it and fetch the
  // JSON payload.
  link = unescapeURL(link)
  const rsp = requests.get(link)
  rsp.raiseForStatus()
  const data = rsp.json()

  // Get author's name
  const author = data.author_name
  if (author) {
    $.authors = [author]
  }

  // Get the publication date
  const date = data.pubdate
  if (date) {
    $.meta["html.date"] = date
  }

  // If it's a photo, change the type and the picture URL
  const type = data.type
  if (type == "photo") {
    $.type = "photo"

    let img = data.url
    if (img) {
      img = unescapeURL(img)
      $.meta["x.picture_url"] = img
    }
  }
}
