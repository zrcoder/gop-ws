package main

import (
	"bytes"
	"io/fs"
	"path"
	"strings"
	"time"
)

var _ fs.ReadDirFS = unionFS{}

// A unionFS is an FS presenting the union of the file systems in the slice.
// If multiple file systems provide a particular file, Open uses the FS listed earlier in the slice.
// If multiple file systems provide a particular directory, ReadDir presents the
// concatenation of all the directories listed in the slice (with duplicates removed).
type unionFS []fs.FS

func (fsys unionFS) Open(name string) (fs.File, error) {
	var errOut error
	for _, sub := range fsys {
		f, err := sub.Open(name)
		if err == nil {
			// Note: Should technically check for directory
			// and return a synthetic directory that merges
			// reads from all the matching directories,
			// but all the directory reads in internal/godoc
			// come from fsys.ReadDir, which does that for us.
			// So we can ignore direct f.ReadDir calls.
			return f, nil
		}
		if errOut == nil {
			errOut = err
		}
	}
	return nil, errOut
}

func (fsys unionFS) ReadDir(name string) ([]fs.DirEntry, error) {
	var all []fs.DirEntry
	var seen map[string]bool // seen[name] is true if name is listed in all; lazily initialized
	var errOut error
	for _, sub := range fsys {
		list, err := fs.ReadDir(sub, name)
		if err != nil {
			errOut = err
		}
		if len(all) == 0 {
			all = append(all, list...)
		} else {
			if seen == nil {
				// Initialize seen only after we get two different directory listings.
				seen = make(map[string]bool)
				for _, d := range all {
					seen[d.Name()] = true
				}
			}
			for _, d := range list {
				name := d.Name()
				if !seen[name] {
					seen[name] = true
					all = append(all, d)
				}
			}
		}
	}
	if len(all) > 0 {
		return all, nil
	}
	return nil, errOut
}

// A fixSpecsFS is an FS mapping /ref/mem.html and /ref/spec.html to
// /doc/go_mem.html and /doc/go_spec.html.
var _ fs.FS = &fixSpecsFS{}

type fixSpecsFS struct {
	fs fs.FS
}

func (fsys fixSpecsFS) Open(name string) (fs.File, error) {
	switch name {
	case "ref/mem.html", "ref/spec.html":
		if f, err := fsys.fs.Open(name); err == nil {
			// Let Go distribution win if they move.
			return f, nil
		}
		// Otherwise fall back to doc/go_*.html
		name = "doc/go_" + strings.TrimPrefix(name, "ref/")
		return fsys.fs.Open(name)

	case "doc/go_mem.html", "doc/go_spec.html":
		data := []byte("<!--{\n\t\"Redirect\": \"/ref/" + strings.TrimPrefix(strings.TrimSuffix(name, ".html"), "doc/go_") + "\"\n}-->\n")
		return &memFile{path.Base(name), bytes.NewReader(data)}, nil
	}

	return fsys.fs.Open(name)
}

// A memFile is an fs.File implementation backed by in-memory data.
type memFile struct {
	name string
	*bytes.Reader
}

func (f *memFile) Stat() (fs.FileInfo, error) { return f, nil }
func (f *memFile) Name() string               { return f.name }
func (*memFile) Mode() fs.FileMode            { return 0444 }
func (*memFile) ModTime() time.Time           { return time.Time{} }
func (*memFile) IsDir() bool                  { return false }
func (*memFile) Sys() interface{}             { return nil }
func (*memFile) Close() error                 { return nil }
