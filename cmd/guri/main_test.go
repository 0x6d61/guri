package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func script(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.scm")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func invoke(args []string, input string) (int, string, string) {
	var out, diagnostic bytes.Buffer
	code := run(args, strings.NewReader(input), &out, &diagnostic)
	return code, out.String(), diagnostic.String()
}
func TestCLIProgramsAndDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		source          string
		code            int
		out, diagnostic string
	}{
		{`(import (scheme base) (scheme write)) (display (+ 1 2)) (newline)`, 0, "3\n", ""},
		{`(import (scheme base))`, 0, "", ""},
		{`(import (scheme base)) (+ 1 2)`, 0, "", ""},
		{"", 1, "", "initial import"},
		{"(import (scheme base))\n(/ 1 0)", 1, "", "sample.scm:2:1:"},
		{"(import (scheme base))\n[", 1, "", "sample.scm:2:1: ReaderError"},
		{"(import (scheme base))\nmissing", 1, "", "unbound identifier"},
		{"(import (scheme base))\n(error \"stop\")", 1, "", "stop"},
	} {
		code, out, diagnostic := invoke([]string{script(t, tt.source)}, "")
		if code != tt.code || out != tt.out || tt.diagnostic != "" && !strings.Contains(diagnostic, tt.diagnostic) || tt.diagnostic == "" && diagnostic != "" {
			t.Fatal(tt, code, out, diagnostic)
		}
	}
}
func TestCLIInputAndCommandLine(t *testing.T) {
	path := script(t, `(import (scheme base) (scheme read) (scheme write) (scheme process-context)) (write (command-line)) (newline) (write (read)) (newline)`)
	code, out, diagnostic := invoke([]string{path, "hello", "--flag"}, "42")
	if code != 0 || diagnostic != "" || out != "("+strconv.Quote(path)+" \"hello\" \"--flag\")\n42\n" {
		t.Fatal(code, out, diagnostic)
	}
	code, out, diagnostic = invoke([]string{"-", "arg"}, `(import (scheme base) (scheme write) (scheme process-context)) (write (command-line))`)
	if code != 0 || out != "(\"-\" \"arg\")" || diagnostic != "" {
		t.Fatal(code, out, diagnostic)
	}
}
func TestCLIREPLRecoveryAndExit(t *testing.T) {
	code, out, diagnostic := invoke(nil, "(define x 40)\n(+ x 2)\n(error \"retry\")\n(+ x 3)\n(exit 7)\n99\n")
	if code != 7 || out != "42\n43\n" || !strings.Contains(diagnostic, "retry") || strings.Contains(out, "guri>") {
		t.Fatal(code, out, diagnostic)
	}
}
func TestCLIExitAndEmergencyExit(t *testing.T) {
	for _, tt := range []struct {
		name, argument, expected string
		code                     int
	}{
		{"exit", "", "after\n", 0}, {"exit", "#f", "after\n", 1}, {"exit", "263", "after\n", 7}, {"exit", "-1", "after\n", 255}, {"emergency-exit", "7", "", 7},
	} {
		source := `(import (scheme base) (scheme process-context) (scheme write)) (dynamic-wind (lambda () #t) (lambda () (` + tt.name + ` ` + tt.argument + `)) (lambda () (display "after\n"))) (display "continued")`
		code, out, diagnostic := invoke([]string{script(t, source)}, "")
		if code != tt.code || out != tt.expected || diagnostic != "" {
			t.Fatal(tt, code, out, diagnostic)
		}
	}
}
func TestCLILibraryPathsAndScriptLocation(t *testing.T) {
	root := t.TempDir()
	envRoot := t.TempDir()
	for dir, value := range map[string]string{root: "42", envRoot: "99"} {
		if err := os.Mkdir(filepath.Join(dir, "app"), 0700); err != nil {
			t.Fatal(err)
		}
		source := `(define-library (app value) (export value) (import (scheme base)) (begin (define value ` + value + `)))`
		if err := os.WriteFile(filepath.Join(dir, "app", "value.sld"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GURI_LOAD_PATH", envRoot)
	path := script(t, `(import (scheme base) (scheme write) (app value)) (write value)`)
	for _, args := range [][]string{{"-I", root, path}, {"-I" + root, path}, {"-I", root, "--", path}} {
		code, out, diagnostic := invoke(args, "")
		if code != 0 || out != "42" || diagnostic != "" {
			t.Fatal(args, code, out, diagnostic)
		}
	}
	code, out, diagnostic := invoke([]string{path}, "")
	if code != 0 || out != "99" || diagnostic != "" {
		t.Fatal(code, out, diagnostic)
	}
}
func TestCLIOptionsAndMissingFile(t *testing.T) {
	for _, tt := range []struct {
		args []string
		code int
		part string
	}{
		{[]string{"--help"}, 0, "Usage:"}, {[]string{"--version"}, 0, "guri 0.0.3"},
		{[]string{"--unknown"}, 2, "unknown option"}, {[]string{"-I"}, 2, "directory"},
		{[]string{filepath.Join(t.TempDir(), "absent.scm")}, 1, "absent.scm"},
	} {
		code, out, diagnostic := invoke(tt.args, "")
		if code != tt.code || !strings.Contains(out+diagnostic, tt.part) {
			t.Fatal(tt, code, out, diagnostic)
		}
	}
}

type borrowedWriter struct {
	bytes.Buffer
	closed bool
}

func (w *borrowedWriter) Close() error { w.closed = true; return nil }
func TestCLIClosingStandardOutputKeepsBorrowedWriterOpen(t *testing.T) {
	var out borrowedWriter
	var diagnostic bytes.Buffer
	path := script(t, `(import (scheme base)) (close-output-port (current-output-port))`)
	code := run([]string{path}, strings.NewReader(""), &out, &diagnostic)
	if code != 0 || out.closed || diagnostic.Len() != 0 {
		t.Fatal(code, out.closed, diagnostic.String())
	}
}

type failingOutput struct{}

func (failingOutput) Write([]byte) (int, error) { return 0, fmt.Errorf("output failure") }
func TestCLIOutputErrors(t *testing.T) {
	var diagnostic bytes.Buffer
	code := run([]string{script(t, `(import (scheme base) (scheme write)) (display "hello")`)}, strings.NewReader(""), failingOutput{}, &diagnostic)
	if code != 1 || !strings.Contains(diagnostic.String(), "output failure") {
		t.Fatal(code, diagnostic.String())
	}
}
