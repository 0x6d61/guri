package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLILegacyNamesAreNotPredefined(t *testing.T) {
	for _, name := range strings.Fields("def defn fn while nil true false :key != inc len cap clone get assoc! dissoc dissoc! update update! conj nth slice reduce println") {
		t.Run(name, func(t *testing.T) {
			code, _, diagnostic := invoke([]string{script(t, "(import (scheme base))\n"+name)}, "")
			if code != 1 || !strings.Contains(diagnostic, name) {
				t.Fatalf("exit=%d diagnostic=%s", code, diagnostic)
			}
			code, out, diagnostic := invoke([]string{script(t, "(import (scheme base) (scheme write))\n(define "+name+" 42)\n(display "+name+")")}, "")
			if code != 0 || out != "42" || diagnostic != "" {
				t.Fatalf("user identifier: exit=%d stdout=%q diagnostic=%s", code, out, diagnostic)
			}
		})
	}
}

func TestCLIStandardReplacements(t *testing.T) {
	source := `(import (scheme base) (scheme write) (scheme lazy))
(write (list (append '(1) '(2))
             (assoc 'x '((x . 42)))
             (apply + 1 '(2 3))
             ((lambda args args) 1 2)
             (if '() #t #f)
             (vector-length #(1 2))
             (bytevector-u8-ref #u8(255) 0)
             (string-ref "日本語" 1)
             (/ 1 2)
             (force (delay (+ 20 22)))))`
	code, out, diagnostic := invoke([]string{script(t, source)}, "")
	if code != 0 || out != "((1 2) (x . 42) 6 (1 2) #t 2 255 #\\x672c 1/2 42)" || diagnostic != "" {
		t.Fatalf("exit=%d stdout=%q diagnostic=%s", code, out, diagnostic)
	}
	for _, source := range []string{"[1 2]", "{:x 1}", `(apply + #(1 2))`, `(append #(1) #(2))`, `b"abc"`} {
		code, _, diagnostic := invoke([]string{script(t, "(import (scheme base))\n"+source)}, "")
		if code != 1 || diagnostic == "" {
			t.Fatalf("legacy %s: exit=%d diagnostic=%s", source, code, diagnostic)
		}
	}
}

func TestCLISchemeExamples(t *testing.T) {
	for filename, want := range map[string]string{"counter.scm": "1\n2\n", "search-brackets.scm": "#(3 2 1 0)\n"} {
		t.Run(filename, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("../../examples", filename))
			if err != nil {
				t.Fatal(err)
			}
			code, out, diagnostic := invoke([]string{script(t, string(source))}, "")
			if code != 0 || out != want || diagnostic != "" {
				t.Fatalf("exit=%d stdout=%q diagnostic=%s", code, out, diagnostic)
			}
		})
	}
}
