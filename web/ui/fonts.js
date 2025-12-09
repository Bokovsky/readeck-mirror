// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

const path = require("path")

// fontCatalog is the main Readeck's font catalog.
// It declares all the needed information to build the CSS file
// and add the font families to tailwind.
const fontCatalog = {
  // Main fonts
  "public-sans": {
    name: "Public Sans Variable",
    family: "sans-serif",
    path: path.dirname(
      require.resolve("@fontsource-variable/public-sans/package.json"),
    ),
    fontFiles: "files",
    css: ["wght.css", "wght-italic.css"],
  },
  lora: {
    name: "Lora Variable",
    family: "serif",
    path: path.dirname(
      require.resolve("@fontsource-variable/lora/package.json"),
    ),
    fontFiles: "files",
    css: ["wght.css", "wght-italic.css"],
  },
  "jetbrains-mono": {
    name: "JetBrains Mono Variable",
    family: "monospace",
    path: path.dirname(
      require.resolve("@fontsource-variable/jetbrains-mono/package.json"),
    ),
    fontFiles: "files",
    css: ["wght.css", "wght-italic.css"],
  },
  "dejavu-math": {
    name: "DejaVu Math",
    family: "serif",
    path: path.dirname(require.resolve("@fontsource/dejavu-math/package.json")),
    fontFiles: "files",
    css: ["index.css"],
  },

  // Extra fonts
  "atkinson-hyperlegible": {
    name: "Atkinson Hyperlegible",
    family: "serif",
    path: path.dirname(
      require.resolve("@fontsource/atkinson-hyperlegible/package.json"),
    ),
    fontFiles: "files",
    css: ["400.css", "400-italic.css", "700.css", "700-italic.css"],
  },
  inter: {
    name: "Inter Variable",
    family: "sans-serif",
    path: path.dirname(
      require.resolve("@fontsource-variable/inter/package.json"),
    ),
    fontFiles: "files",
    css: ["wght.css", "wght-italic.css"],
  },
  literata: {
    name: "Literata Variable",
    family: "serif",
    path: path.dirname(
      require.resolve("@fontsource-variable/literata/package.json"),
    ),
    fontFiles: "files",
    css: ["wght.css", "wght-italic.css"],
  },
  luciole: {
    name: "Luciole",
    family: "sans-serif",
    path: path.join(__dirname, "fonts/luciole"),
    fontFiles: "",
    css: ["index.css"],
  },
  merriweather: {
    name: "Merriweather",
    family: "serif",
    path: path.dirname(
      require.resolve("@fontsource/merriweather/package.json"),
    ),
    fontFiles: "files",
    css: ["400.css", "400-italic.css", "700.css", "700-italic.css"],
  },
  "plex-serif": {
    name: "IBM Plex Serif",
    family: "serif",
    path: path.dirname(
      require.resolve("@fontsource/ibm-plex-serif/package.json"),
    ),
    fontFiles: "files",
    css: ["400.css", "400-italic.css", "700.css", "700-italic.css"],
  },
}

const emojiFonts = [
  "Apple Color Emoji",
  "Segoe UI Emoji",
  "Segoe UI Symbol",
  "Noto Color Emoji",
]

module.exports = {
  // atRules returns a list of @import rules
  // that are inserted during the CSS bundle creation.
  atRules: () => {
    return Object.values(fontCatalog).reduce((acc, cur) => {
      acc.push(...cur.css.map((x) => `@import "${path.join(cur.path, x)}";`))
      return acc
    }, [])
  },

  // basePath return a list of all the paths used by the fonts.
  // We use it to copy the font files to the assets folder.
  basePath: () => {
    return Object.values(fontCatalog).map((x) => {
      return path.join(x.path, x.fontFiles)
    })
  },

  // families return a tailwind declaration of font families.
  families: () => {
    return Object.entries(fontCatalog).reduce(
      (acc, [k, v]) => {
        acc[k] = [v.name, v.family].concat(emojiFonts)
        return acc
      },
      // Default aliases
      {
        sans: [
          fontCatalog["public-sans"].name,
          fontCatalog["public-sans"].family,
        ].concat(emojiFonts),
        mono: [
          fontCatalog["jetbrains-mono"].name,
          fontCatalog["jetbrains-mono"].family,
        ].concat(emojiFonts),
      },
    )
  },
}
