// SPDX-FileCopyrightText: © 2024 Joachim Robert <joachim.robert@protonmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-only

const rxPost = /^\/profile\/(.+)?\/post\/([a-zA-Z0-9]+)$/

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "bsky.app" && rxPost.test(new URL($.url).pathname)
}

exports.processMeta = function () {
  const m = new URL($.url).pathname.match(
    /^\/profile\/(.+)?\/post\/([a-zA-Z0-9]+)$/,
  )
  const userName = m[1]
  const postID = m[2]

  loadThread(userName, postID)
}

/**
 *
 * @param {string} userName
 * @param {string} postID
 */
function loadThread(userName, postID) {
  let rsp = requests.get(
    `https://public.api.bsky.app/xrpc/com.atproto.identity.resolveHandle?handle=${userName}`,
  )
  rsp.raiseForStatus()
  const did = rsp.json().did

  const postURI = `at://${did}/app.bsky.feed.post/${postID}`
  rsp = requests.get(
    `https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread?uri=${postURI}&depth=20`,
  )
  rsp.raiseForStatus()
  const data = rsp.json()
  const replies = data.thread?.replies || []

  const notes = []

  const post = getNoteData(data.thread?.post)

  notes.push(post)

  let next = getNextPostFromUser(replies, userName)

  while (true) {
    if (!next) {
      break
    }

    notes.push(getNoteData(next.post))
    next = getNextPostFromUser(next.replies, userName)
  }

  if ("parent" in data.thread) {
    let parent = data.thread.parent

    while (true) {
      if (!parent) {
        break
      }

      if (parent?.author?.handle !== userName) {
        break
      }

      notes.unshift(getNoteData(parent.post))
      parent = parent.parent
    }
  }

  // Bluesky doesn't handle video upload yet
  // const isVideo =
  //   notes.length == 1 &&
  //   (notes[0].videos || []).length > 0

  const isPicture = notes.length == 1 && (notes[0].images || []).length == 1

  switch (true) {
    // Bluesky doesn't handle video upload yet
    // case isVideo:
    //   $.type = "video"
    //   $.meta[
    //     "oembed.html"
    //   ] = `<video src="${notes[0].videos[0].url}"></video>`
    //   notes[0].videos.shift()
    //   break
    case isPicture:
      $.type = "photo"
      $.meta["x.picture_url"] = notes[0].images[0].fullsize
      notes[0].images.shift()
      break
    default:
      if ("images" in notes[0]) {
        $.meta["x.picture_url"] = notes[0]?.images[0].fullsize
        break
      }

      $.meta["x.picture_url"] = data.thread.post.author.avatar
      break
  }

  const html = notes.map((n) => {
    let noteHtml = `<p>${n.html}</p>`
    noteHtml = noteHtml.replace(/\n\n/g, "</p><p>")
    noteHtml = noteHtml.replace(/\n/g, "<br>")

    if ("link" in n) {
      noteHtml += `<p><a href="${n.link.uri}">${n.link.title}</a></p>`
    }

    if ("images" in n) {
      noteHtml += n.images
        .map((image) => {
          return `<figure><img alt="${image.alt}" src="${image.fullsize}"></figure>`
        })
        .join("\n")
    }

    return `<article>${noteHtml}</article>`
  })

  $.description = ""
  $.html = `<div>${html.join("<hr>")}</div>`
  $.readability = false
}

function getNextPostFromUser(replies, userName) {
  return replies
    ?.filter((reply) => reply.post.author.handle == userName)
    .reverse()[0]
}

function getNoteData(note) {
  const noteData = {
    published: note.record.createdAt,
    html: note.record.text,
  }

  if (!!note?.embed?.external) {
    noteData.link = {
      uri: note.embed.external.uri,
      title: note.embed.external.title,
    }
  }

  if (!!note?.embed?.images) {
    noteData.images = note.embed.images
  }

  for (const key in note.record.facets) {
    const facet = note.record.facets[key]
    if (facet?.features[0].uri === noteData.link?.uri) {
      // There's a discrepancy between byteStart/byteEnd and the content of `note.record.text`
      const difference = note.record.text.length - facet.index.byteEnd

      if (difference <= 0) {
        // If the link is at the end of the post, remove it
        noteData.html =
          note.record.text.slice(0, facet.index.byteStart + difference) +
          note.record.text.slice(facet.index.byteEnd + difference)
      } else {
        // If the link is inside the post, replace it with a clickable link
        // Hoping there's no discrepancy
        noteData.html = note.record.text.slice(0, facet.index.byteStart)
        noteData.html += `<a href="${facet.features[0].uri}">`
        noteData.html += note.record.text.slice(
          facet.index.byteStart,
          facet.index.byteEnd,
        )
        noteData.html += "</a>"
        noteData.html += note.record.text.slice(facet.index.byteEnd)
      }
    }
  }

  return noteData
}
