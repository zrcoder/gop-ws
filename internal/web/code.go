// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"

	"github.com/goplus/website/internal/backport/html/template"
	"github.com/goplus/website/internal/texthtml"
)

func (s *siteDir) code(file string, arg ...interface{}) (_ template.HTML, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	_, text, _, cfg, err := s.locate("code", file, arg...)
	if err != nil {
		return "", err
	}
	cfg.GoComments = true
	if cfg.HL == "" {
		cfg.HL = "HL"
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<div class=\"code\">\n\n")
	fmt.Fprintf(&buf, "<pre>")
	// HTML-escape text and syntax-color comments like elsewhere.
	buf.Write(texthtml.Format([]byte(text), cfg))
	fmt.Fprintf(&buf, "</pre>\n")
	fmt.Fprintf(&buf, "</div>\n\n")
	return template.HTML(buf.String()), nil
}

func (s *siteDir) play(file string, arg ...interface{}) (_ template.HTML, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	before, text, after, cfg, err := s.locate("play", file, arg...)
	if err != nil {
		return "", err
	}
	cfg.Playground = true

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<div class=\"playground\">\n\n")
	if before != "" {
		fmt.Fprintf(&buf, "<pre style=\"display: none\"><span>%s</span>\n</pre>\n", html.EscapeString(before))
	}
	// HTML-escape text and syntax-color comments like elsewhere.
	fmt.Fprintf(&buf, "<pre contenteditable=\"true\" spellcheck=\"false\">")
	buf.Write(texthtml.Format([]byte(text), cfg))
	fmt.Fprintf(&buf, "</pre>\n")
	if after != "" {
		fmt.Fprintf(&buf, "<pre style=\"display: none\"><span>%s</span>\n</pre>\n", html.EscapeString(after))
	}
	fmt.Fprintf(&buf, "</div>\n\n")

	return template.HTML(buf.String()), nil
}

func (s *siteDir) locate(verb, file string, arg ...interface{}) (before, text, after string, cfg texthtml.Config, err error) {
	btext, err := s.readFile(s.dir, file)
	if err != nil {
		return
	}
	text = string(btext)
	if len(arg) > 0 {
		if s, ok := arg[len(arg)-1].(string); ok && strings.HasPrefix(s, "HL") {
			cfg.HL = s
			arg = arg[:len(arg)-1]
		}
	}
	if len(arg) > 0 {
		if n, ok := arg[len(arg)-1].(int); ok {
			if n == 0 {
				n = -1
			}
			cfg.Line = n
			arg = arg[:len(arg)-1]
		}
	}
	switch len(arg) {
	case 0:
		// text is already whole file.
		if cfg.Line == -1 {
			cfg.Line = 1
		}
	case 1:
		var n int
		before, text, after, n = s.oneLine(file, text, arg[0])
		if cfg.Line == -1 {
			cfg.Line = n
		}
	case 2:
		var n int
		before, text, after, n = s.multipleLines(file, text, arg[0], arg[1])
		if cfg.Line == -1 {
			cfg.Line = n
		}
	default:
		err = fmt.Errorf("incorrect code invocation: %s %q [%v, ...] (%d arguments)", verb, file, arg[0], len(arg))
		return
	}

	// Trim leading and trailing blank lines from output.
	text = strings.Trim(text, "\n")
	if text != "" {
		text += "\n"
	}
	// Replace tabs by spaces, which work better in HTML.
	text = strings.Replace(text, "\t", "    ", -1)

	return
}

// Functions in this file panic on error, but the panic is recovered
// to an error by 'code'.

// stringFor returns a textual representation of the arg, formatted according to its nature.
func stringFor(arg interface{}) string {
	switch arg := arg.(type) {
	case int:
		return fmt.Sprintf("%d", arg)
	case string:
		if len(arg) > 2 && arg[0] == '/' && arg[len(arg)-1] == '/' {
			return fmt.Sprintf("%#q", arg)
		}
		return fmt.Sprintf("%q", arg)
	default:
		log.Panicf("unrecognized argument: %v type %T", arg, arg)
	}
	return ""
}

// oneLine returns the single line generated by a two-argument code invocation.
func (s *Site) oneLine(file, body string, arg interface{}) (before, text, after string, num int) {
	lines := strings.SplitAfter(body, "\n")
	line, pattern, isInt := parseArg(arg, file, len(lines))
	if !isInt {
		line = match(file, 0, lines, pattern)
	}
	line--
	return strings.Join(lines[:line], ""), lines[line], strings.Join(lines[line+1:], ""), line
}

// multipleLines returns the text generated by a three-argument code invocation.
func (s *Site) multipleLines(file, body string, arg1, arg2 interface{}) (before, text, after string, num int) {
	lines := strings.SplitAfter(body, "\n")
	line1, pattern1, isInt1 := parseArg(arg1, file, len(lines))
	line2, pattern2, isInt2 := parseArg(arg2, file, len(lines))
	if !isInt1 {
		line1 = match(file, 0, lines, pattern1)
	}
	if !isInt2 {
		line2 = match(file, line1, lines, pattern2)
	} else if line2 < line1 {
		log.Panicf("lines out of order for %q: %d %d", file, line1, line2)
	}
	for k := line1 - 1; k < line2; k++ {
		if strings.HasSuffix(lines[k], "OMIT\n") {
			lines[k] = ""
		}
	}
	line1--
	return strings.Join(lines[:line1], ""),
		strings.Join(lines[line1:line2], ""),
		strings.Join(lines[line2:], ""), line1
}

// parseArg returns the integer or string value of the argument and tells which it is.
func parseArg(arg interface{}, file string, max int) (ival int, sval string, isInt bool) {
	switch n := arg.(type) {
	case int:
		if n <= 0 || n > max {
			log.Panicf("%q:%d is out of range", file, n)
		}
		return n, "", true
	case string:
		return 0, n, false
	}
	log.Panicf("unrecognized argument %v type %T", arg, arg)
	return
}

// match identifies the input line that matches the pattern in a code invocation.
// If start>0, match lines starting there rather than at the beginning.
// The return value is 1-indexed.
func match(file string, start int, lines []string, pattern string) int {
	// $ matches the end of the file.
	if pattern == "$" {
		if len(lines) == 0 {
			log.Panicf("%q: empty file", file)
		}
		return len(lines)
	}
	// /regexp/ matches the line that matches the regexp.
	if len(pattern) > 2 && pattern[0] == '/' && pattern[len(pattern)-1] == '/' {
		re, err := regexp.Compile("(?m)" + pattern[1:len(pattern)-1])
		if err != nil {
			log.Panic(err)
		}
		for i := start; i < len(lines); i++ {
			if re.MatchString(lines[i]) {
				return i + 1
			}
		}
		log.Panicf("%s: no match for %#q", file, pattern)
	}
	log.Panicf("unrecognized pattern: %q", pattern)
	return 0
}
