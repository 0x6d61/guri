package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestCLIFilesystem(t *testing.T) {
	for _, mode := range []string{"program", "stdin", "repl", "eval", "library"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			q := func(name string) string { return strconv.Quote(filepath.Join(root, name)) }
			body := `(create-directory ` + q("dir") + `) (write (file-directory? ` + q("dir") + `)) (newline) (delete-directory ` + q("dir") + `)`
			src := `(import (scheme base) (scheme write) (guri filesystem)) ` + body
			var args []string
			input := ""
			switch mode {
			case "program":
				args = []string{script(t, src)}
			case "stdin":
				args = []string{"-"}
				input = src
			case "repl":
				input = src
			case "eval":
				args = []string{script(t, `(import (scheme base) (scheme write) (scheme eval)) (write (eval '(file-directory? `+strconv.Quote(root)+`) (environment '(guri filesystem)))) (newline)`)}
			case "library":
				if err := os.WriteFile(filepath.Join(root, "fs-client.sld"), []byte(`(define-library (fs-client) (export check) (import (scheme base) (guri filesystem)) (begin (define check file-directory?)))`), 0600); err != nil {
					t.Fatal(err)
				}
				args = []string{"-I", root, script(t, `(import (scheme base) (scheme write) (fs-client)) (write (check `+strconv.Quote(root)+`)) (newline)`)}
			}
			code, out, diagnostic := invoke(args, input)
			if code != 0 || out != "#t\n" || diagnostic != "" {
				t.Fatal(code, out, diagnostic)
			}
		})
	}
	t.Run("repl explicit import", func(t *testing.T) {
		code, out, diagnostic := invoke(nil, `(file-directory? ".") (import (guri filesystem)) (file-directory? ".")`)
		if code != 0 || out != "#t\n" || !strings.Contains(diagnostic, "uninitialized binding") {
			t.Fatal(code, out, diagnostic)
		}
	})
}

func TestCLIFilesystemWikiExamples(t *testing.T) {
	doc, err := os.ReadFile("../../docs/wiki/Filesystem.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```").FindAllStringSubmatch(string(doc), -1)
	if len(blocks) != 2 {
		t.Fatal(len(blocks))
	}
	for i, block := range blocks {
		root := t.TempDir()
		code, out, diagnostic := invoke([]string{script(t, block[1]), root}, "")
		expected := "(#t #t 5 2)\n"
		if i == 1 {
			expected = "#t\n"
		}
		if code != 0 || out != expected || diagnostic != "" {
			t.Fatal(i, code, out, diagnostic)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatal(entries, err)
		}
	}
}
