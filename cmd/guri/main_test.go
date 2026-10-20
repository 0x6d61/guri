package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	for _, tt := range []struct {
		source       string
		code         int
		out, errPart string
	}{
		{`(println (+ 1 2))`, 0, "3\n", ""}, {"", 0, "", ""},
		{"\n(/ 1 0)", 1, "", ":2:1: ArithmeticError:"},
		{"[", 1, "", ":1:1: ReaderError:"},
		{"missing", 1, "", ":1:1: NameError:"},
	} {
		t.Run(tt.source, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample.guri")
			if err := os.WriteFile(path, []byte(tt.source), 0600); err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			code := run([]string{path}, &out, &stderr)
			if code != tt.code || out.String() != tt.out {
				t.Fatalf("code=%d out=%q err=%q", code, out.String(), stderr.String())
			}
			if tt.errPart == "" {
				if stderr.Len() != 0 {
					t.Fatal(stderr.String())
				}
			} else if !strings.Contains(stderr.String(), path+tt.errPart) {
				t.Fatal(stderr.String())
			}
		})
	}
}

func TestRunArguments(t *testing.T) {
	for _, tt := range []struct {
		args []string
		code int
	}{
		{nil, 2}, {[]string{"one", "two"}, 2}, {[]string{"--help"}, 0},
		{[]string{filepath.Join(t.TempDir(), "absent.guri")}, 1},
	} {
		var out, stderr bytes.Buffer
		if code := run(tt.args, &out, &stderr); code != tt.code {
			t.Fatalf("args=%v code=%d", tt.args, code)
		}
		if out.Len()+stderr.Len() == 0 {
			t.Fatal("missing diagnostic")
		}
	}
}
