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
  type Node = dom.Node
}

export {}

declare namespace contentScript {
  type unescapeURL = (url: string) => string
  type decodeXML = (s: string) => { [key: string]: any }
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
    properties: Readonly<{ [key: string]: any }>

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
    get(
      url: string | url.URL,
      headers?: { [key: string]: string } | null,
    ): Response
    post(
      url: string | url.URL,
      data: string,
      headers?: { [key: string]: string } | null,
    ): Response
  }
}

declare namespace dom {
  type Attribute = {
    name: string
    value: string
  }

  interface Node {
    get TEXT_NODE(): number
    get DOCUMENT_NODE(): number
    get ELEMENT_NODE(): number
    get COMMENT_NODE(): number
    get DOCTYPE_NODE(): number

    /**
     * Returns a string appropriate for the type of node.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/nodeName)
     */
    get nodeName(): string

    /**
     * Returns the type of node.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/nodeType)
     */
    get nodeType(): number

    /** [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/nodeValue) */
    get nodeValue(): string

    /** [MDN Reference](https://developer.mozilla.org/docs/Web/API/Element/attributes) */
    get attributes(): Attribute[]

    /**
     * Specifies the beginning and end of the document body.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Document/body)
     */
    get body(): Node | undefined

    /**
     * Returns the children.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/childNodes)
     */
    get childNodes(): Node[]

    /**
     * Returns the child elements.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Document/children)
     */
    get children(): Node[]

    /**
     * Returns the first child.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/firstChild)
     */
    get firstChild(): Node | null

    /**
     * Returns the first child that is an element, and null otherwise.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Document/firstElementChild)
     */
    get firstElementChild(): Node | null

    /**
     * Returns the value of element's id content attribute. Can be set to change it.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Element/id)
     */
    get id(): string
    set id(id: string)

    /** [MDN Reference](https://developer.mozilla.org/docs/Web/API/Element/innerHTML) */
    get innerHTML(): string
    set innerHTML(html: string)

    /**
     * Returns the last child.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/lastChild)
     */
    get lastChild(): Node | null

    /**
     * Returns the next sibling.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/nextSibling)
     */
    get nextSibling(): Node | null

    /** [MDN Reference](https://developer.mozilla.org/docs/Web/API/Element/outerHTML) */
    get outerHTML(): string

    /**
     * Returns the parent.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/parentNode)
     */
    get parentNode(): Node | null

    /**
     * Returns the parent element.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/parentElement)
     */
    get parentElement(): Node | null

    /**
     * Returns the previous sibling.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/previousSibling)
     */
    get previousSibling(): Node | null

    /** [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/textContent) */
    get textContent(): string

    /** [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/appendChild) */
    appendChild(node: Node): undefined

    /**
     * Inserts nodes after the last child of node, while replacing strings in nodes with equivalent Text nodes.
     *
     * Throws a "HierarchyRequestError" DOMException if the constraints of the node tree are violated.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Document/append)
     */
    append(node: Node): undefined

    /**
     * Returns a copy of node. If deep is true, the copy also includes the node's descendants.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/cloneNode)
     */
    cloneNode(): Node

    /**
     * Returns true if other is an inclusive descendant of node, and false otherwise.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/contains)
     */
    contains(node: Node): boolean

    /**
     * Creates an instance of the element for the specified tag.
     * @param name The name of an element.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Document/createElement)
     */
    createElement(name: string): Node

    /**
     * Creates a text string from the specified value.
     * @param data String that specifies the nodeValue property of the text node.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Document/createTextNode)
     */
    createTextNode(data: string): Node

    /**
     * Returns element's first attribute whose qualified name is qualifiedName, and null if there is no such attribute otherwise.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Element/getAttribute)
     */
    getAttribute(name: string): string

    /**
     * Returns true if element has an attribute whose qualified name is qualifiedName, and false otherwise.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Element/hasAttribute)
     */
    hasAttribute(name: string): boolean

    /**
     * Returns true if element has attributes, and false otherwise.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Element/hasAttributes)
     */
    hasAttributes(): boolean

    /**
     * Returns whether node has children.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/hasChildNodes)
     */
    hasChildNodes(): boolean

    /** [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/insertBefore) */
    insertBefore(newChild: Node, oldChild: Node): Node

    /** [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/removeChild) */
    removeChild(node: Node): Node

    /**
     * Returns the first element that is a descendant of node that matches selectors.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Document/querySelector)
     */
    querySelector(s: string): Node

    /**
     * Returns all element descendants of node that match selectors.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Document/querySelectorAll)
     */
    querySelectorAll(s: string): Node[]

    /** [MDN Reference](https://developer.mozilla.org/docs/Web/API/Node/replaceChild) */
    replaceChild(newChild: Node, oldChild: Node): Node

    /**
     * Replaces node with nodes, while replacing strings in nodes with equivalent Text nodes.
     *
     * Throws a "HierarchyRequestError" DOMException if the constraints of the node tree are violated.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/CharacterData/replaceWith)
     */
    replaceWith(...nodes: (Node | string)[]): undefined

    /**
     * Sets the value of element's first attribute whose qualified name is qualifiedName to value.
     *
     * [MDN Reference](https://developer.mozilla.org/docs/Web/API/Element/setAttribute)
     */
    setAttribute(name: string, value: string): undefined
  }
}
