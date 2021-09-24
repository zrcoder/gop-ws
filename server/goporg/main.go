// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// goporg serves the goplus.org web sites.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/goplus/website/internal/redirect"
	"github.com/goplus/website/internal/web"
)

var (
	httpAddr = flag.String("http", "localhost:9999", "HTTP service address")
	goroot   = flag.String("goroot", runtime.GOROOT(), "Go root directory")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: goporg\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	repoRoot := "../.."
	if _, err := os.Stat("_content"); err == nil {
		repoRoot = "."
	}
	contentDir := filepath.Join(repoRoot, "_content")

	flag.Usage = usage
	flag.Parse()

	// Check usage.
	if flag.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "Unexpected arguments.")
		usage()
	}
	if *httpAddr == "" {
		fmt.Fprintln(os.Stderr, "-http must be set")
		usage()
	}

	handler := NewHandler(contentDir, *goroot)

	// Start http server.
	fmt.Fprintf(os.Stderr, "serving http://%s\n", *httpAddr)
	if err := http.ListenAndServe(*httpAddr, handler); err != nil {
		log.Fatalf("ListenAndServe %s: %v", *httpAddr, err)
	}
}

// NewHandler returns the http.Handler for the web site,
// given the directory where the content can be found
// (can be "", in which case an internal copy is used)
// and the directory of the GOROOT.
func NewHandler(contentDir, goroot string) http.Handler {
	mux := http.NewServeMux()
	contentFS := os.DirFS(contentDir)
	gorootFS := os.DirFS(goroot)
	_, err := newSite(mux, "", contentFS, gorootFS)
	if err != nil {
		log.Fatalf("newSite: %v", err)
	}
	redirect.Register(mux)
	return mux
}

func newSite(mux *http.ServeMux, host string, content, goroot fs.FS) (*web.Site, error) {
	fsys := unionFS{content, &fixSpecsFS{goroot}}
	site := web.NewSite(fsys)
	mux.Handle(host+"/", site)
	return site, nil
}
