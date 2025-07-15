# Readeck Archiver

This package is a fork of [Obelisk](https://github.com/go-shiori/obelisk) by Radhi Fadlillah.

What started as a soft fork with few changes is now an independent package that retains most of Obelisk's logic for finding resources but introduces a modular and less memory consumming way to store resources after downloading them.

## The Archiver

The Archiver is a structure that provides public methods to archive a document.
It then visits all the images, stylesheets, scripts, etc.

The archiver doesn't store any content but only provides private utilities: 

- fetchInfo: fetch a resource but only sniff its content-type and dimensions when it's an image
- fetch: retrieve an io.ReadCloser (the response's body)
- saveResource: saves an io.ReadCloser into the Collector

## The Collector

Upon each visit, it may call a Collector that takes care of the following:

- Give it a name (default to an UUID) that is used as a new attribute value (or CSS URL)
- Provide an io.Writer in which the resource's content can be saved

In the most simple cases, this design provides a direct connection between an http.Response.Body and the provided io.Writer, without any intermediate storage.

## DownloadCollector

DownloadCollector is a partial Collector that keeps a resource inventory and takes care of retrieving remote documents.

## FileCollector

FileCollector is a full Collector that renames resources to an UUID (URL namespace) and saves them into an `os.Root` filesystem.

## ZipCollector

ZipCollector is a full Collector that saves resources inside a `zip.Writer`.

## SingleFileCollector

SingleFileCollector is a Collector that doesn't save files but replaces all URLs with `data:` URIs. As one can imagine, it is memory intensive.
