// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

declare global {
  var $: contentScript.Message
  var requests: http.Requests
  var console: console.Console

  var unescapeURL: contentScript.unescapeURL
  var decodeXML: contentScript.decodeXML
  var escapeHTML: contentScript.escapeHTML

  var URL = url.URL
  var URLSearchParams = url.URLSearchParams

  type Config = contentScript.Config
}

export {}

declare namespace contentScript {
  type unescapeURL = (url: string) => string
  type decodeXML = (s: string) => {[key:string]: any}
  type escapeHTML = (s: string) => string

  interface Config {
    /** XPath selectors for the document title. */
    titleSelectors: string[]

    /** XPath selectors for the document body. */
    bodySelectors: string[]

    /** XPath selectors for the document date. */
    dateSelectors: string[]

    /** XPath selectors for the document authors. */
    authorSelectors: string[]

    /** XPath selectors of elements that must be removed. */
    stripSelectors: string[]

    /** List of IDs or classes that belong to elements that must be removed. */
    stripIdOrClass: string[]

    /** List of strings that, when present in an `src` attribute of an image
     *  will trigger the element removal.
     */
    stripImageSrc: string[]

    /** XPath selectors of elements whose `href` attribute refers
     *  to a link to the full document.
     */
    singlePageLinkSelectors: string[]

    /** XPath selectors of elements whose `href` attribute refers
     *  to a link to the next page.
     */
    nextPageLinkSelectors: string[]

    /** List of pairs of string replacement. */
    replaceStrings: string[][]

    /** An object that contain HTTP headers being sent to every subsequent requests. */
    httpHeaders: { [key: string]: string }
  }

  interface Message {
    meta: { [key: string]: string[] }
    properties: Readonly<{[key:string]: any}>

    /**
     * The domain of the current extraction. Note that it's different from the host name.
     * For example, if the host name is `www.example.co.uk`,
     * the value of `$.domain` is `example.co.uk`.
     *
     * The value is always in its Unicode form regardless of the initial input.
     */
    domain: Readonly<string>

    /**
     * The host name of the current extraction.
     *
     * The value is always in its Unicode form regardless of the initial input.
     */
    host: Readonly<string>

    /**
     * The URL of the current extraction. The value is a string that you can parse
     * with `new URL($.url)` when needed.
     */
    url: Readonly<string>

    /**
     * A list of found authors in the document.
     *
     * **Note**: When setting this value, it must be a list and you can
     * **not** use `$.authors.push()` to add new values.
     */
    authors: string[]

    /**
     * Document's description.
     */
    description: string

    /**
     * The site name
     */
    site: string

    /**
     * Document's title
     */
    title: string

    /**
     * Document's type. When settings this value, it must be one of "article", "photo" or "video".
     */
    type: "article" | "photo" | "video"

    /**
     * Whether readability is enabled for this content. It can be useful to set it to false when
     * setting an HTML content with `$.html`.
     *
     * Please note that even though readability can be disabled, it won't disable the last cleaning
     * pass that removes unwanted tags and attributes.
     */
    readability: boolean

    /**
     * When settings a string to this variable, the whole extracted content is replaced.
     *
     * This is an advanced option and should only be used for content
     * that are not articles (photos or videos).
     */
    set html(s: string)

    /**
     * This overrides the site's configuration. It can be used in a context
     * where a pages is retrieved from an archive mirror but you want to apply
     * its original configuration.
     *
     * @param cfg Original configuration
     * @param src New configuration match
     */
    overrideConfig(cfg: Config, src: string): null
  }
}

declare namespace console {
  interface Console {
    debug(...data: any[]): void
    error(...data: any[]): void
    info(...data: any[]): void
    log(...data: any[]): void
    warn(...data: any[]): void
  }
}

declare namespace url {
  interface URL {
    hash: string
    host: string
    hostname: string
    href: string
    toString(): string
    readonly origin: string
    password: string
    pathname: string
    port: string
    protocol: string
    search: string
    readonly searchParams: URLSearchParams
    username: string
    toJSON(): string
  }

  declare var URL: {
    prototype: URL
    new (url: string | URL, base?: string | URL): URL
    canParse(url: string | URL, base?: string | URL): boolean
    createObjectURL(obj: Blob | MediaSource): string
    parse(url: string | URL, base?: string | URL): URL | null
    revokeObjectURL(url: string): void
  }

  interface URLSearchParams {
    readonly size: number
    append(name: string, value: string): void
    delete(name: string, value?: string): void
    get(name: string): string | null
    getAll(name: string): string[]
    has(name: string, value?: string): boolean
    set(name: string, value: string): void
    sort(): void
    toString(): string
    forEach(
      callbackfn: (value: string, key: string, parent: URLSearchParams) => void,
      thisArg?: any,
    ): void
  }

  declare var URLSearchParams: {
    prototype: URLSearchParams
    new (
      init?: string[][] | Record<string, string> | string | URLSearchParams,
    ): URLSearchParams
  }
}

declare namespace http {
  type Response = {
    status: number
    headers: { string: string[] }
    raiseForStatus(): null
    json(): any
    text(): string
  }

  export type Requests = {
    get(url: string | url.URL, headers?: { [key: string]: string } | null): Response
    post(
      url: string | url.URL,
      data: string,
      headers?: { [key: string]: string } | null,
    ): Response
  }
}
