// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// This file contains modification of site configs only.

exports.priority = 10

exports.isActive = function () {
  return true
}

exports.setConfig = function (config) {
  switch (true) {
    case $.domain == "arstechnica.com":
      config.replaceStrings = [
        ['" data-src="', '"><img src="'],
        ['" data-responsive="', '" /><span data-responsive="'],
        ['<figure style="', '</span><figure data-style="'],
      ]
      break

    case $.domain == "bostonglobe.com":
      config.bodySelectors = ["//article"]
      config.stripIdOrClass = ["tagline", "tagline_hr"]
      break

    case $.domain == "fastcompany.com":
      config.replaceStrings = []
      break

    case $.domain == "longform.org":
      config.singlePageLinkSelectors.push(
        "//a[@href][@class='post__link']/@href",
      )
      break

    case $.domain == "mediapart.fr":
      config.bodySelectors.push(
        "//div[contains(concat(' ',normalize-space(@class),' '),' content-article ')]",
      )
      config.stripIdOrClass.push("cookie-consent-banner-content")
      break

    case $.domain == "newyorker.com":
      // 2023-12-18: the new site-config is too restrictive
      config.replaceStrings = []
      config.bodySelectors.unshift(
        "//article[contains(@class, 'main-content')]",
        "//div[@id='articleBody']",
      )
      config.stripSelectors = [
        '//button[@id="bookmark"]',
        '//div[@data-testid="BylinesWrapper"]',
        '//*[@data-testid="IframeEmbed"]',
        "//*[contains(@class, 'ContentHeaderByline')]",
        "//*[contains(@class, 'SplitScreenContentHeaderTitleBlock')]",
        "//time",
        '//ul[@data-testid="socialIconslist"]',
        "//div[contains(concat(' ', normalize-space(@class), ' '), ' responsive-cartoon ')]",
      ]
      break

    case $.domain == "nytimes.com":
      config.httpHeaders["user-agent"] =
        "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

    case $.domain == "slate.fr":
      config.stripIdOrClass.push("newsletter-container")
      config.stripIdOrClass.push("to-read")

    case $.domain == "slate.com":
      // The original replaceStrings replaces noscript by divs,
      // not the best idea.
      config.replaceStrings = []
      break

    case $.domain == "theguardian.com":
      // Do not remove figcaption tags
      config.stripSelectors = config.stripSelectors.filter(
        (x) => x != "//figcaption",
      )
      break

    case $.domain == "theverge.com":
      config.replaceStrings = []
      break

    case $.domain == "politico.eu":
      config.bodySelectors.unshift('//article')
      config.stripSelectors.push('//nav', '//aside', '//footer')
      config.stripIdOrClass.push('ad', 'listen', 'pro-pill')
      break
  }
}
