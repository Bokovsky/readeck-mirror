// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return (
    $.domain == "unsplash.com" && !!new URL($.url).pathname.match(/^\/photos\//)
  )
}

exports.processMeta = function () {
  $.type = "photo"

  // There's a JSON endpoint with plenty of goodies
  const url = new URL($.url)
  url.pathname = "/napi" + url.pathname
  const rsp = requests.get(url)
  rsp.raiseForStatus()
  const data = rsp.json()

  let img = data.urls?.regular
  if (img) {
    img = unescapeURL(img)
    $.meta["x.picture_url"] = img
  }

  const author = data.user?.name
  if (author) {
    $.authors = [author]
  }

  const title = data.description
  if (title) {
    $.title = title
  }

  const desc = data.alt_description
  if (desc) {
    $.description = desc
  }

  const date = data.created_at
  if (date) {
    $.meta["html.date"] = date
  }
}
