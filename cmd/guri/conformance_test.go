package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLIR7RSDeclaration(t *testing.T) {
	expression := `(and (pair? (memq 'r7rs (features)))
   (cond-expand (r7rs #t) (else #f)))`
	t.Run("program", func(t *testing.T) {
		code, out, diagnostic := invoke([]string{script(t, `(import (scheme base) (scheme write)) (write `+expression+`) (newline)`)}, "")
		if code != 0 || out != "#t\n" || diagnostic != "" {
			t.Fatal(code, out, diagnostic)
		}
	})
	t.Run("repl", func(t *testing.T) {
		code, out, diagnostic := invoke(nil, expression)
		if code != 0 || out != "#t\n" || diagnostic != "" {
			t.Fatal(code, out, diagnostic)
		}
	})
	t.Run("library", func(t *testing.T) {
		root := t.TempDir()
		source := `(define-library (declaration)
   (export supported?) (import (scheme base))
   (cond-expand (r7rs (begin (define supported? #t)))
                (else (begin (define supported? #f)))))`
		if err := os.WriteFile(filepath.Join(root, "declaration.sld"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		path := script(t, `(import (scheme base) (scheme write) (declaration)) (write supported?) (newline)`)
		code, out, diagnostic := invoke([]string{"-I", root, path}, "")
		if code != 0 || out != "#t\n" || diagnostic != "" {
			t.Fatal(code, out, diagnostic)
		}
	})
	t.Run("eval", func(t *testing.T) {
		source := `(import (scheme base) (scheme write) (scheme eval))
    (write (eval '(cond-expand (r7rs #t) (else #f)) (environment '(scheme base)))) (newline)`
		code, out, diagnostic := invoke([]string{script(t, source)}, "")
		if code != 0 || out != "#t\n" || diagnostic != "" {
			t.Fatal(code, out, diagnostic)
		}
	})
}
