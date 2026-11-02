package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestCLIProcessChild(t *testing.T) {
	for _, arg := range os.Args {
		if arg == "--guri-cli-process-helper" {
			if _, err := os.Stdout.WriteString("child output"); err != nil {
				os.Exit(91)
			}
			if _, err := os.Stderr.WriteString("child diagnostic"); err != nil {
				os.Exit(92)
			}
			os.Exit(7)
		}
	}
}

func TestCLIProcess(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := `'(` + strconv.Quote(exe) + ` "-test.run=^TestCLIProcessChild$" "--" "--guri-cli-process-helper")`
	src := `(import (scheme base) (scheme write) (guri process))
 (call-with-values (lambda () (run-process ` + command + `))
  (lambda (s o e) (write s) (newline) (display (utf8->string o)) (newline) (display (utf8->string e)) (newline)))`
	for _, mode := range []string{"program", "stdin", "repl"} {
		t.Run(mode, func(t *testing.T) {
			var args []string
			input := src
			if mode == "program" {
				args = []string{script(t, src)}
				input = ""
			} else if mode == "stdin" {
				args = []string{"-"}
			}
			code, out, diag := invoke(args, input)
			want := "7\nchild output\nchild diagnostic\n"
			if code != 0 || out != want || diag != "" {
				t.Fatal(code, strconv.Quote(out), diag)
			}
		})
	}
	code, out, diag := invoke([]string{script(t, `(import (scheme base) (guri process)) (run-process '("guri-missing-executable-02b2f37b"))`)}, "")
	if code != 1 || out != "" || !strings.Contains(diag, "run-process") {
		t.Fatal(code, out, diag)
	}
}

func TestCLIProcessWikiExample(t *testing.T) {
	doc, err := os.ReadFile("../../docs/wiki/Process.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```").FindAllStringSubmatch(string(doc), -1)
	if len(blocks) != 1 {
		t.Fatal(len(blocks))
	}
	example, err := os.ReadFile("../../examples/process-pipeline.scm")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(example)) != strings.TrimSpace(blocks[0][1]) {
		t.Fatal("wiki example differs from executable example")
	}
	exe := filepath.Join(t.TempDir(), "guri")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if output, err := exec.Command("go", "build", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	path := script(t, blocks[0][1])
	cmd := exec.Command(exe, path, exe)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "(0 0)\nhello\nchild diagnostic\nchild diagnostic\n" {
		t.Fatal(string(out), err)
	}
}
