// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import fs from "fs"
import path from "path"
import {createRequire} from "module"
import zlib from "zlib"

const require = createRequire(import.meta.url)

import {glob} from "glob"
import gulp from "gulp"
import gulpCheerio from "gulp-cheerio"
import gulpHash from "gulp-hash-filename"
import gulpEsbuild from "gulp-esbuild"
import gulpPostcss from "gulp-postcss"
import gulpRename from "gulp-rename"
import gulpSass from "gulp-sass"
import gulpSourcemaps from "gulp-sourcemaps"
import gulpSvgStore from "gulp-svgstore"

import ordered from "ordered-read-streams"
import through from "through2"
import * as sass from "sass"

import {deleteAsync as del} from "del"

import {stimulusPlugin} from "esbuild-plugin-stimulus"

import fontCatalog from "./ui/fonts.js"

const sassCompiler = gulpSass(sass)

const DEST = path.resolve("../assets/www")

// hashName returns a gulp stream for hashing filenames with the
// same pattern.
function hashName() {
  return gulpHash({
    format: "{name}.{hash:8}{ext}",
  })
}

function toPosixPath(p) {
  return process.platform === "win32" ? p.replace(/\\/g, "/") : p
}

// destCompress returns a gulp stream that compresses the current
// stream's files in gzip or brotli.
// It pushes the resulting file to the stream with an added suffix
// (.gz or .br).
function destCompress(format) {
  return through.obj(function (file, _, done) {
    if (
      file.isNull() ||
      file._isCompressed ||
      path.extname(file.basename) == ".map"
    ) {
      return done(null, file)
    }

    if (file.isStream()) {
      done(null, file)
      return
    }

    let options, fn
    let cf = file.clone({deep: true, contents: true})
    cf.basename = `${cf.basename}.${format}`

    if (format == "gz") {
      options = {
        level: 9,
      }
      fn = zlib.gzip
    } else if (format == "br") {
      options = {
        params: {
          [zlib.constants.BROTLI_PARAM_QUALITY]: 11,
        },
      }
      fn = zlib.brotliCompress
    } else {
      done(`unknown format ${format}`)
      return
    }

    fn(cf.contents, options, (err, contents) => {
      if (err) {
        done(err)
        return
      }

      cf.contents = contents
      cf._isCompressed = true
      this.push(cf)
      done(null, file)
    })
  })
}

// cleanFiles calls del() with some default options.
function cleanFiles(...args) {
  return del(args, {cwd: DEST, force: true})
}

// clean_js remove the JS assets.
function clean_js(done) {
  return cleanFiles("*.js", "*.js.*")
}

// clean_css removes the CSS assets (and fonts).
function clean_css() {
  return cleanFiles("*.css", "*.css.*", "fonts")
}

// clean_media removes static assets like svg or images.
function clean_media() {
  return cleanFiles("img")
}

function clean_vendor() {
  return cleanFiles("vendor")
}

// clean_manifest creates an empty manifest.json file.
function clean_manifest(done) {
  let dest = path.join(DEST, "manifest.json")
  fs.writeFile(dest, "{}", done)
}

// clean delete files in the destination folder
function clean_all(done) {
  return gulp.series(
    clean_js, //
    clean_css,
    clean_media,
    clean_vendor,
    clean_manifest,
  )(done)
}

// js_bundle creates the JS bundle file using esbuild.
function js_bundle() {
  return ordered([
    gulp
      .src("src/main.js")
      .pipe(
        gulpEsbuild({
          sourcemap: "inline",
          outfile: "bundle.js",
          bundle: true,
          format: "esm",
          platform: "browser",
          metafile: false,
          minifyIdentifiers: true,
          minifyWhitespace: true,
          logLevel: "warning",
          plugins: [stimulusPlugin()],
        }),
      )
      .pipe(gulpSourcemaps.init({loadMaps: true})) // This extracts the inline sourcemap
      .pipe(hashName())
      .pipe(gulpSourcemaps.write("."))
      .pipe(destCompress("gz"))
      .pipe(destCompress("br"))
      .pipe(gulp.dest(DEST)),
    gulp
      .src("src/public.js")
      .pipe(
        gulpEsbuild({
          sourcemap: "inline",
          outfile: "public.js",
          bundle: true,
          format: "esm",
          platform: "browser",
          metafile: false,
          minifyIdentifiers: true,
          minifyWhitespace: true,
          logLevel: "warning",
          plugins: [stimulusPlugin()],
        }),
      )
      .pipe(gulpSourcemaps.init({loadMaps: true})) // This extracts the inline sourcemap
      .pipe(hashName())
      .pipe(gulpSourcemaps.write("."))
      .pipe(destCompress("gz"))
      .pipe(destCompress("br"))
      .pipe(gulp.dest(DEST)),
  ])
}

// css_bundle creates the CSS bundle.
function css_bundle() {
  const processors = [
    require("postcss-import"),
    require("./ui/plugins/prose"),
    require("./ui/plugins/palettes.js"),
    require("tailwindcss"),
    require("./ui/plugins/responsive-units.js"),
    require("./ui/plugins/trim-fonts.js"),
    require("./ui/plugins/hover-media-query.js"),
    require("postcss-copy")({
      basePath: fontCatalog.basePath(),
      dest: DEST,
      ignore: (m) => {
        return m.ext != "woff2"
      },
      template: (m) => {
        return `./fonts/${m.name}.${m.hash.substr(0, 8)}.${m.ext}`
      },
    }),
    require("autoprefixer"),
    require("cssnano"),
  ]

  return gulp
    .src([
      "ui/index.scss", //
    ])
    .pipe(
      through.obj(function (file, _, done) {
        // Push @import from the font catalog
        if (file.isBuffer()) {
          const concat = []

          for (let l of file.contents.toString().split("\n")) {
            if (l.startsWith("//--fonts--")) {
              concat.push(Buffer.from(fontCatalog.atRules().join("\n")))
              concat.push(Buffer.from("\n\n"))
              continue
            }
            concat.push(Buffer.from(l + "\n"))
          }
          file.contents = Buffer.concat(concat)
        }

        this.push(file)
        done()
      }),
    )
    .pipe(gulpSourcemaps.init())
    .pipe(sassCompiler().on("error", sassCompiler.logError))
    .pipe(gulpRename("bundle.css"))
    .pipe(gulpPostcss(processors))
    .pipe(hashName())
    .pipe(gulpSourcemaps.write("."))
    .pipe(destCompress("gz"))
    .pipe(destCompress("br"))
    .pipe(gulp.dest(DEST))
}

// css_extra create the css files used as internal assets (epub, email)
function css_extra() {
  const processors = [
    require("postcss-import"),
    require("./ui/plugins/prose"),
    require("autoprefixer"),
  ]

  return ordered(
    [
      {src: "ui/epub/stylesheet.scss", dest: "epub.css"},
      {src: "ui/email/stylesheet.scss", dest: "email.css"},
    ].map((x) => {
      return gulp
        .src(x.src)
        .pipe(gulpSourcemaps.init())
        .pipe(sassCompiler().on("error", sassCompiler.logError))
        .pipe(gulpRename(x.dest))
        .pipe(gulpPostcss(processors))
        .pipe(gulp.dest(DEST))
    }),
  )
}

function icon_sprite(src, dst) {
  // Icons are defined in this file
  const icons = JSON.parse(fs.readFileSync(src))

  return gulp
    .src(Object.values(icons))
    .pipe(
      gulpRename((file, f) => {
        // Set new filename on each entry in order to set
        // a chosen ID on each symbol.
        let p = toPosixPath(path.relative(f.cwd, f.path))
        let id = Object.entries(icons).find((x) => x[1] == p)[0]
        file.basename = id
      }),
    )
    .pipe(
      gulpCheerio({
        // Force viewBox attribute when missing
        run: ($) => {
          let e = $("svg")
          if (e.attr("viewBox") === undefined) {
            let w = e.attr("width") || "24"
            let h = e.attr("height") || "24"
            e.attr("viewBox", `0 0 ${w} ${h}`)
          }
        },
        parserOptions: {xmlMode: true},
      }),
    )
    .pipe(gulpSvgStore())
    .pipe(gulpRename(dst))
    .pipe(hashName())
    .pipe(destCompress("gz"))
    .pipe(destCompress("br"))
    .pipe(gulp.dest(DEST))
}

// icon_bundle creates the icon bundle files
function icon_bundle() {
  return ordered([
    icon_sprite("./media/icons.json", "img/icons.svg"),
    icon_sprite("./media/logos.json", "img/logos.svg"),
  ])
}

// copy_files copies some files to the destination.
function copy_files() {
  return ordered([
    gulp
      .src("media/favicons/*", {encoding: false})
      .pipe(hashName())
      .pipe(gulp.dest(path.join(DEST, "img/fi"))),
    gulp
      .src(["media/logo-text.svg", "media/logo-maskable.svg"], {
        encoding: false,
      })
      .pipe(hashName())
      .pipe(destCompress("gz"))
      .pipe(destCompress("br"))
      .pipe(gulp.dest(path.join(DEST, "img"))),

    gulp
      .src(path.join(require.resolve("hls.js/dist/hls.min.js")), {
        encoding: false,
      })
      .pipe(gulpRename("hls.js"))
      .pipe(hashName())
      .pipe(destCompress("gz"))
      .pipe(destCompress("br"))
      .pipe(gulp.dest(path.join(DEST, "vendor"))),

    gulp
      .src("vendor/*", {encoding: false})
      .pipe(hashName())
      .pipe(destCompress("gz"))
      .pipe(destCompress("br"))
      .pipe(gulp.dest(path.join(DEST, "vendor"))),
  ])
}

// write_manifest generates a manifest.json file in the destination folder.
// It's a very naive process that lists all the files in the destination
// folder and creates a mapping for all the files having a hash suffix.
async function write_manifest(done) {
  const rxFilename = new RegExp(/^(.+)(\.[a-f0-9]{8}\.)(.+)$/)
  const excluded = [".br", ".gz", ".map"]

  const files = await glob(toPosixPath(path.join(DEST, "**/*")))

  let manifest = {}
  for (let f of files) {
    let st = await fs.promises.stat(f)
    if (!st.isFile()) {
      continue
    }
    f = toPosixPath(path.relative(DEST, f))
    if (f == "manifest.json" || excluded.includes(path.extname(f))) {
      continue
    }

    let m = f.match(rxFilename)
    if (!m) {
      continue
    }

    manifest[`${m[1]}.${m[3]}`] = f
  }

  let dest = path.join(DEST, "manifest.json")
  fs.writeFile(dest, JSON.stringify(manifest, null, "  ") + "\n", done)
}

// ------------------------------------------------------------------
// Gulp pipelines
// ------------------------------------------------------------------

const full_build = gulp.series(
  clean_all,
  gulp.parallel(
    js_bundle, //
    icon_bundle,
    copy_files,
    css_bundle,
    css_extra,
  ),
  write_manifest,
)

function watch_js() {
  gulp.watch(
    ["src/**/*"],
    gulp.series(
      clean_js, //
      js_bundle,
      write_manifest,
    ),
  )
}

function watch_css() {
  gulp.watch(
    [
      "tailwind.config.js", //
      "ui/**/*",
      "../assets/templates/**/*.jet.*",
    ],
    gulp.series(
      clean_css, //
      css_bundle,
      css_extra,
      write_manifest,
    ),
  )
}

function watch_media() {
  gulp.watch(
    ["media/**/*"],
    gulp.series(
      clean_media, //
      icon_bundle,
      copy_files,
      write_manifest,
    ),
  )
}

export const clean = clean_all
export const js = js_bundle
export const css = gulp.series(clean_css, css_bundle, css_extra)
export const epub = css_extra
export const icons = icon_bundle
export const copy = copy_files

export {watch_css as "watch:css"}
export {watch_js as "watch:js"}

export const dev = gulp.series(
  full_build,
  gulp.parallel(
    watch_js, //
    watch_css,
    watch_media,
  ),
)

export default full_build
