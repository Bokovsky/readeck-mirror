{*
SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>

SPDX-License-Identifier: AGPL-3.0-only
*}
{{- import "/_libs/common" -}}
<?xml version="1.0" encoding="utf-8"?>
<xsl:stylesheet
  version="1.0"
  xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
  xmlns:atom="http://www.w3.org/2005/Atom"
  exclude-result-prefixes="atom"
>
  <xsl:output method="html" version="1.0" encoding="UTF-8" indent="yes"/>
  <xsl:template match="/">
    <html xmlns="http://www.w3.org/1999/xhtml">
      <head>
        <title>Feed - <xsl:value-of select="atom:feed/atom:title"/></title>
        <meta http-equiv="X-UA-Compatible" content="IE=edge" />
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
        <link rel="icon" href="{{ assetURL(`img/fi/favicon.ico`) }}" sizes="48x48" />
        <link rel="stylesheet" href="{{ assetURL(`bundle.css`) }}" />
      </head>
      <body class="font-sans leading-tight tracking-normal max-w-col-14 mt-0 mb-12 mx-auto">
        <section>
          <div class="my-8 p-4 rounded bg-yellow-200">
            <p><strong>This is a web feed</strong>, also known as an Atom feed.
            <strong>Subscribe</strong> by following the instructions below.</p>
          </div>
        </section>
        <section class="mb-8">
          <xsl:apply-templates select="atom:feed" />
        </section>
        <section class="border-t pt-4">
          <h2 class="title text-h3">Feed's Items</h2>
          <xsl:apply-templates select="atom:feed/atom:entry" />
        </section>
      </body>
    </html>
  </xsl:template>

  <xsl:template match="atom:feed">
    <h1 class="flex items-center gap-2 title text-h2">
      <img alt="" class="inline h-6 w-6 mb-0">
        <xsl:attribute name="src">
          <xsl:value-of select="atom:icon"/>
        </xsl:attribute>
      </img>
      <span><xsl:value-of select="atom:title"/> • Web Feed Preview</span>
    </h1>

    <p class="mb-2">This feed provides the latest entries from <xsl:value-of select="atom:title"/>.</p>
    <p class="mb-2">Subscribe to this feed by copying the URL below:</p>

    <p class="mb-4">
      <code>
        <a class="link text-sm font-semibold">
          <xsl:attribute name="href">
            <xsl:value-of select="atom:link[@rel='self']/@href"/>
          </xsl:attribute>
          <xsl:value-of select="atom:link[@rel='self']/@href"/>
        </a>
      </code>
    </p>

    <p class="my-4 p-4 rounded bg-red-100">
      <strong>Access to this feed is restricted.</strong><br/>
      You must provide your newsreader app a username and an <a class="link" href="{{ urlFor(`/profile/tokens`) }}">API Token</a> to load this feed.
    </p>

    <p class="text-center">
      <a class="btn btn-outlined btn-primary">
        <xsl:attribute name="href">
          <xsl:value-of select="atom:link[@rel='alternate']/@href"/>
        </xsl:attribute>
        {{ yield icon(name="o-chevron-l") }}
        Go back
      </a>
    </p>

  </xsl:template>

  <xsl:template match="atom:entry">
    <div class="mb-4">
      <h3 class="mb-1.5 text-lg font-semibold leading-none">
        <a class="link">
          <xsl:attribute name="href">
            <xsl:value-of select="atom:link[@rel='alternate']/@href"/>
          </xsl:attribute>
          <xsl:value-of select="atom:title"/>
        </a>
      </h3>
      <p>
        <xsl:value-of select="atom:summary"  disable-output-escaping="yes" />
      </p>
      <small>
        Published: <xsl:value-of select="atom:updated" />
      </small>
    </div>
  </xsl:template>

</xsl:stylesheet>
