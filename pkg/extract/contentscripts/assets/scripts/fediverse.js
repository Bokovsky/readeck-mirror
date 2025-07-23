// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return (
    ($.properties?.link || []).find(
      (x) => x["@type"] == "application/activity+json" && x["@href"],
    ) != null
  )
}

exports.processMeta = function () {
  const link = ($.properties.link || []).find(
    (x) => x["@type"] == "application/activity+json",
  )["@href"]

  const notes = []
  loadNotes(link, notes)

  if (notes.length == 0) {
    return
  }

  // Store the date
  $.meta["html.date"] = notes[0].published

  if ($.readability === false) {
    return
  }

  // Filter notes by their context, everything that does not announce a toot
  // is to be removed in order to avoid fetching wordpress "notes".
  if (!Array.isArray(notes[0].context) || notes[0].context.length == 0) {
    return
  }
  const tootContext = notes[0].context.find((x) => {
    if (typeof x != "object") {
      return false
    }
    return x.toot !== undefined
  })
  if (!tootContext) {
    return
  }

  // Only one post and the first attachment is a video, document is a video.
  const isVideo =
    notes.length == 1 &&
    (notes[0].attachment || []).length > 0 &&
    (notes[0].attachment[0].mediaType || "").startsWith("video/")

  // Only one post and one attachment that is an image, document is a picture.
  // This isn't always true but that's a best effort.
  const isPicture =
    notes.length == 1 &&
    (notes[0].attachment || []).length == 1 &&
    (notes[0].attachment[0].mediaType || "").startsWith("image/") &&
    Math.max(notes[0].attachment[0].width, notes[0].attachment[0].height) > 800

  switch (true) {
    case isVideo:
      $.type = "video"
      $.meta[
        "oembed.html"
      ] = `<video src="${notes[0].attachment[0].url}"></video>`
      notes[0].attachment.shift()
      break
    case isPicture:
      $.type = "photo"
      $.meta["x.picture_url"] = notes[0].attachment[0].url
      notes[0].attachment.shift()
      break
  }

  // Set the final html
  const html = notes
    .map((n) => {
      return [n.html]
        .concat(
          n.attachment
            .map((x) => {
              if (x.mediaType.startsWith("image/")) {
                return `<p><img src="${x.url}" alt="${escapeHTML(
                  x.name,
                )}" width="${x.width}" height="${x.height}"></p>`
              }
            })
            .filter((x) => x),
        )
        .join("\n")
    })
    .filter((x) => x.trim() != "")

  $.description = ""
  $.html = `<div>${html.join("\n")}</div>`
  $.readability = false
}

function loadNotes(url, notes) {
  const rsp = requests.get(url, { Accept: "application/json" })
  rsp.raiseForStatus()
  const data = rsp.json()

  const note = {
    context: data["@context"],
    published: data.published,
    attachment: data.attachment || [],
    html: data.content,
  }

  notes.push(note)

  // From here, we'll try to fetch the whole thread.
  // A thread is only direct answer to the current message, from the same user.
  if (!data.attributedTo) {
    return
  }
  if (!data.replies?.first?.items) {
    return
  }

  const next = data.replies.first.items.find((x) =>
    x.startsWith(data.attributedTo),
  )
  if (!next) {
    return
  }

  // Found a consecutive answer, we can load it.
  console.debug("next item in thread", { url: next })
  loadNotes(next, notes)
}
