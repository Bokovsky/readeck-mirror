// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// package main contains the yaml-compose code.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func main() {
	usage := fmt.Sprintf("Usage: %s <input> <output>", flag.CommandLine.Name())

	var format string
	flag.StringVar(&format, "format", "json", "output format")

	flag.Parse()
	if len(flag.Args()) < 2 {
		log.Fatal(usage)
	}

	input, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	output, err := filepath.Abs(flag.Arg(1))
	if err != nil {
		log.Fatal(err)
	}

	col, err := newCollection(input)
	if err != nil {
		log.Fatal(err)
		return
	}

	afile, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := afile.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	if format == "yaml" {
		enc := yaml.NewEncoder(afile)
		err = enc.Encode(col.files[col.main])
	} else {
		enc := newJSONEncoder(afile)
		err = enc.encode(col.files[col.main])
	}

	if err != nil {
		afile.Close()  //nolint:errcheck
		log.Fatal(err) //nolint:gocritic
	}
}
