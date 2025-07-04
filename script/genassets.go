// Copyright (C) 2014 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

var tpl = template.Must(template.New("assets").Parse(`// Code generated by genassets.go - DO NOT EDIT.

package auto

import (
	"time"

	"github.com/weeniemount/syncthing/lib/assets"
)

func Assets() map[string]assets.Asset {
	var ret = make(map[string]assets.Asset, {{.Assets | len}})
	t := time.Unix({{.Generated}}, 0)

{{range $asset := .Assets}}
	ret["{{$asset.Name}}"] = assets.Asset{
		Content:  {{$asset.Data}},
		Gzipped:  {{$asset.Gzipped}},
		Length:   {{$asset.Length}},
		Filename: {{$asset.Name | printf "%q"}},
		Modified: t,
	}
{{end}}
	return ret
}

`))

type asset struct {
	Name    string
	Data    string
	Length  int
	Gzipped bool
}

var assets []asset

func walkerFor(basePath string) filepath.WalkFunc {
	return func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasPrefix(filepath.Base(name), ".") {
			// Skip dotfiles
			return nil
		}

		if info.Mode().IsRegular() {
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			length := len(data)

			var buf bytes.Buffer
			gw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			gw.Write(data)
			gw.Close()

			// Only replace asset by gzipped version if it is smaller.
			// In practice, this means HTML, CSS, SVG etc. get compressed,
			// while PNG and WOFF files are left uncompressed.
			// lib/assets detects gzip and sets headers/decompresses.
			gzipped := buf.Len() < len(data)
			if gzipped {
				data = buf.Bytes()
			}

			name, _ = filepath.Rel(basePath, name)
			assets = append(assets, asset{
				Name:    filepath.ToSlash(name),
				Data:    fmt.Sprintf("%q", string(data)),
				Length:  length,
				Gzipped: gzipped,
			})
		}

		return nil
	}
}

type templateVars struct {
	Assets    []asset
	Generated int64
}

func main() {
	outfile := flag.String("o", "", "Name of output file (default stdout)")
	flag.Parse()

	filepath.Walk(flag.Arg(0), walkerFor(flag.Arg(0)))
	var buf bytes.Buffer

	// Generated time is now, except if the SOURCE_DATE_EPOCH environment
	// variable is set (for reproducible builds).
	generated := time.Now().Unix()
	if s, _ := strconv.ParseInt(os.Getenv("SOURCE_DATE_EPOCH"), 10, 64); s > 0 {
		generated = s
	}

	tpl.Execute(&buf, templateVars{
		Assets:    assets,
		Generated: generated,
	})
	bs, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	out := io.Writer(os.Stdout)
	if *outfile != "" {
		out, err = os.Create(*outfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	out.Write(bs)
}
