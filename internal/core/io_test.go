package core_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestDatumPortReadingAndMixedInput(t *testing.T) {
	for _, src := range []string{
		`(define p (open-input-string "(1 . 2) \"猫\" #u8(1 2)")) (and (equal? (read p) '(1 . 2)) (char=? (read-char p) #\space) (string=? (read p) "猫") (equal? (read p) #u8(1 2)) (eof-object? (read p)) (input-port-open? p))`,
		`(define p (open-input-string "#0=(1 . #0#) #0=#(#0#)")) (define a (read p)) (define b (read p)) (and (eq? a (cdr a)) (eq? b (vector-ref b 0)))`,
		`(define p (open-input-string "#!fold-case FOO Bar #!no-fold-case BAZ")) (and (eq? (read p) 'foo) (eq? (read p) 'bar) (eq? (read p) 'BAZ))`,
		`(define p (open-input-string "#; (ignored) #| comment |# 42")) (parameterize ((current-input-port p)) (and (= (read) 42) (eof-object? (read))))`,
		`(define p (open-input-string "(a)z")) (read p) (char=? (peek-char p) #\z)`,
		`(define p (open-input-string "(1) (2)")) (define a (read p)) (set-car! a 9) (= (car a) 9)`,
		`(define p (open-input-string "#0=(1) #0#")) (read p) (guard (e ((read-error? e) #t) (else #f)) (read p))`,
	} {
		if got := eval(t, core.NewCore(), src); got != scheme.True {
			t.Fatal(src, got)
		}
	}
	for _, bad := range []string{`(`, `"unfinished`, `#| unfinished`, `#;`, `#u8(256)`, `#0=#0#`} {
		e := core.NewCore()
		s, err := scheme.NewString(bad)
		if err != nil {
			t.Fatal(err)
		}
		e.Bind("bad", s)
		if got := eval(t, e, `(guard (e ((read-error? e) #t) (else #f)) (read (open-input-string bad)))`); got != scheme.True {
			t.Fatal(bad, got)
		}
	}
}
func TestDatumPortWriting(t *testing.T) {
	for src, want := range map[string]string{
		`(define p (open-output-string)) (write '(a "x" #\space) p) (get-output-string p)`:                                                  `(a "x" #\space)`,
		`(define p (open-output-string)) (display '(a "x" #\space) p) (get-output-string p)`:                                                `(a x  )`,
		`(define p (open-output-string)) (define a (list 1)) (write-shared (list a a) p) (get-output-string p)`:                             `(#0=(1) #0#)`,
		`(define p (open-output-string)) (define a (list 1)) (write-simple (list a a) p) (get-output-string p)`:                             `((1) (1))`,
		`(define p (open-output-string)) (define a (list 1)) (write (list a a) p) (get-output-string p)`:                                    `((1) (1))`,
		`(define p (open-output-string)) (parameterize ((current-output-port p)) (display "猫") (write 42) (newline)) (get-output-string p)`: "猫42\n",
	} {
		if got := eval(t, core.NewCore(), src).(*scheme.String).Text(); got != want {
			t.Error(src, got, want)
		}
	}
	for _, name := range []string{"write", "write-shared", "display"} {
		src := `(define p (open-output-string)) (define a (cons "x" '())) (set-cdr! a a) (` + name + ` a p) (define r (read (open-input-string (get-output-string p)))) (eq? r (cdr r))`
		// Display writes symbols for strings, but must preserve a finite cyclic representation.
		if got := eval(t, core.NewCore(), src); got != scheme.True {
			t.Fatal(name, got)
		}
	}
	for _, src := range []string{`(read (open-input-bytevector #u8()))`, `(read (open-output-string))`, `(write 1 (open-output-bytevector))`, `(define p (open-output-string)) (close-port p) (display "" p)`} {
		if got := eval(t, core.NewCore(), `(guard (e (else (error-object? e))) `+src+`)`); got != scheme.True {
			t.Fatal(src, got)
		}
	}
	for _, src := range []string{`(read 1 2)`, `(write)`, `(display)`, `(write-shared)`, `(write-simple 1 2 3)`} {
		if _, err := core.NewCore().Eval(context.Background(), "arity.scm", src); err == nil {
			t.Error(src)
		}
	}
}

func fileEngine(t *testing.T, path string) *core.Engine {
	t.Helper()
	e := core.NewCore()
	name, err := scheme.NewString(path)
	if err != nil {
		t.Fatal(err)
	}
	e.Bind("path", name)
	return e
}
func TestSchemeFilePorts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "猫.scm")
	e := fileEngine(t, path)
	if got := eval(t, e, `(define out (open-output-file path)) (display "猫 " out) (write '(1 2) out) (close-port out) (define in (open-input-file path)) (list (read in) (read in))`); !equality.Equal(got, scheme.List(scheme.NewSymbolTable().Intern("猫"), scheme.List(scheme.NewInteger(1), scheme.NewInteger(2)))) {
		t.Fatal(got)
	}
	eval(t, e, `(close-port in)`)
	if got, err := os.ReadFile(path); err != nil || string(got) != "猫 (1 2)" {
		t.Fatal(string(got), err)
	}
	eval(t, e, `(define out2 (open-output-file path)) (display "x" out2) (close-port out2)`)
	if got, err := os.ReadFile(path); err != nil || string(got) != "x" {
		t.Fatal("truncate", string(got), err)
	}
	e = fileEngine(t, filepath.Join(t.TempDir(), "binary"))
	if got := eval(t, e, `(define out (open-binary-output-file path)) (write-bytevector #u8(0 255 206 187) out) (close-port out) (define in (open-binary-input-file path)) (define b (read-bytevector 20 in)) (close-port in) (equal? b #u8(0 255 206 187))`); got != scheme.True {
		t.Fatal(got)
	}
	e = fileEngine(t, filepath.Join(t.TempDir(), "missing"))
	for _, src := range []string{`(open-input-file path)`, `(open-binary-input-file path)`} {
		if got := eval(t, e, `(guard (e ((file-error? e) #t) (else #f)) `+src+`)`); got != scheme.True {
			t.Fatal(src, got)
		}
	}
	e = fileEngine(t, filepath.Join(t.TempDir(), "missing-dir", "out"))
	for _, src := range []string{`(open-output-file path)`, `(open-binary-output-file path)`} {
		if got := eval(t, e, `(guard (e ((file-error? e) #t) (else #f)) `+src+`)`); got != scheme.True {
			t.Fatal(src, got)
		}
	}
}
func TestFileProceduresDynamicPortsAndContinuations(t *testing.T) {
	e := fileEngine(t, filepath.Join(t.TempDir(), "out"))
	r, err := e.Eval(context.Background(), "file.scm", `(define saved #f) (call-with-output-file path (lambda (p) (set! saved p) (display "ok" p) (values 20 22)))`)
	if err != nil || r.Len() != 2 || number(r.Values()[0]) != 20 || number(r.Values()[1]) != 22 || eval(t, e, `(output-port-open? saved)`) != scheme.False {
		t.Fatal(r, err)
	}
	if got := eval(t, e, `(call-with-input-file path (lambda (p) (eq? (read p) 'ok)))`); got != scheme.True {
		t.Fatal(got)
	}
	if got := eval(t, e, `(define old (current-output-port)) (with-output-to-file path (lambda () (display "42"))) (and (eq? old (current-output-port)) (= (with-input-from-file path (lambda () (read))) 42))`); got != scheme.True {
		t.Fatal(got)
	}
	e = fileEngine(t, filepath.Join(t.TempDir(), "continuation"))
	if got := number(eval(t, e, `(define k #f) (define port #f) (define old (current-output-port)) (call/cc (lambda (escape) (with-output-to-file path (lambda () (set! port (current-output-port)) (call/cc (lambda (resume) (set! k resume) (escape 1))) (display "x") 42))))`)); got != 1 {
		t.Fatal(got)
	}
	if got := eval(t, e, `(and (output-port-open? port) (eq? old (current-output-port)))`); got != scheme.True {
		t.Fatal("escape", got)
	}
	if got := number(eval(t, e, `(k 0)`)); got != 42 || eval(t, e, `(output-port-open? port)`) != scheme.False {
		t.Fatal("resume", got)
	}
	if _, err := e.Eval(context.Background(), "closed.scm", `(k 0)`); err == nil {
		t.Fatal("reopened closed file")
	}
	e = fileEngine(t, filepath.Join(t.TempDir(), "raised"))
	if got := eval(t, e, `(define p #f) (define old (current-output-port)) (guard (e (else (and (eq? old (current-output-port)) (output-port-open? p)))) (with-output-to-file path (lambda () (set! p (current-output-port)) (error "escape"))))`); got != scheme.True {
		t.Fatal(got)
	}
	eval(t, e, `(close-port p)`)
}

func TestDatumIOErrorsSourcesAndCancellation(t *testing.T) {
	e := core.NewCore()
	e.Bind("bad", ports.Value(ports.Input(bytes.NewReader(append([]byte("(1)"), 0xff)), false, ports.Options{Filename: "bad-utf8.scm"})))
	if v := eval(t, e, `(equal? (read bad) '(1))`); v != scheme.True {
		t.Fatal(v)
	}
	if v := eval(t, e, `(guard (e ((read-error? e) #t) (else #f)) (read bad))`); v != scheme.True {
		t.Fatal(v)
	}
	e = core.NewCore()
	e.Bind("bad-out", ports.Value(ports.Output(ioFailWriter{}, false, ports.Options{})))
	if v := eval(t, e, `(guard (e (else (error-object? e))) (display "x" bad-out))`); v != scheme.True {
		t.Fatal(v)
	}
	e = core.NewCore()
	_, err := e.Eval(context.Background(), "caller.scm", "\n(read (open-input-string \"(\"))")
	var raised *scheme.Raised
	if !errors.As(err, &raised) || raised.Source().Line != 2 || !strings.Contains(err.Error(), "<string-port>:1:1") {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bad.scm")
	if err := os.WriteFile(path, []byte("\n(unfinished"), 0600); err != nil {
		t.Fatal(err)
	}
	e = fileEngine(t, path)
	_, err = e.Eval(context.Background(), "caller.scm", `(call-with-input-file path (lambda (p) (guard (e (else (close-port p) (raise e))) (read p))))`)
	if !errors.As(err, &raised) || !strings.Contains(err.Error(), path+":2:1") {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = core.NewCore().Eval(ctx, "cancel.scm", `(define p (open-output-string)) (define a (cons 1 '())) (set-cdr! a a) (write-simple a p)`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

type ioFailWriter struct{}

func (ioFailWriter) Write(b []byte) (int, error) { return 0, io.ErrShortWrite }

func TestInputFileContinuationAndNormalClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("42 43"), 0600); err != nil {
		t.Fatal(err)
	}
	e := fileEngine(t, path)
	if n := number(eval(t, e, `(define k #f) (define p #f) (call/cc (lambda (escape) (call-with-input-file path (lambda (port) (set! p port) (call/cc (lambda (resume) (set! k resume) (escape 1))) (read port)))))`)); n != 1 {
		t.Fatal(n)
	}
	if v := eval(t, e, `(input-port-open? p)`); v != scheme.True {
		t.Fatal(v)
	}
	if n := number(eval(t, e, `(k 0)`)); n != 42 {
		t.Fatal(n)
	}
	if v := eval(t, e, `(input-port-open? p)`); v != scheme.False {
		t.Fatal(v)
	}
	if _, err := e.Eval(context.Background(), "reenter.scm", `(k 0)`); err == nil {
		t.Fatal("reopened input")
	}
	e = fileEngine(t, path)
	r, err := e.Eval(context.Background(), "multi.scm", `(define old (current-input-port)) (with-input-from-file path (lambda () (values 20 22)))`)
	if err != nil || r.Len() != 2 || eval(t, e, `(eq? old (current-input-port))`) != scheme.True {
		t.Fatal(r, err)
	}
	e = fileEngine(t, filepath.Join(t.TempDir(), "out"))
	r, err = e.Eval(context.Background(), "zero.scm", `(with-output-to-file path (lambda () (values)))`)
	if err != nil || r.Len() != 0 {
		t.Fatal(r, err)
	}
}

func TestFilePathRules(t *testing.T) {
	t.Chdir(t.TempDir())
	e := fileEngine(t, "relative-猫")
	eval(t, e, `(define p (open-output-file path)) (display "ok" p) (close-port p)`)
	if b, err := os.ReadFile("relative-猫"); err != nil || string(b) != "ok" {
		t.Fatal(string(b), err)
	}
	for _, src := range []string{`(open-input-file 1)`, `(open-output-file 1)`, `(call-with-output-file path 1)`, `(with-input-from-file path 1)`} {
		if v := eval(t, e, `(guard (e (else (and (error-object? e) (not (file-error? e))))) `+src+`)`); v != scheme.True {
			t.Fatal(src, v)
		}
	}
	if v := eval(t, e, `(guard (e ((file-error? e) #t) (else #f)) (open-input-file (string #\null)))`); v != scheme.True {
		t.Fatal(v)
	}
	// Invalid callbacks must be rejected before truncating an existing file.
	if b, err := os.ReadFile("relative-猫"); err != nil || string(b) != "ok" {
		t.Fatal(string(b), err)
	}
}

func TestDatumIOWikiExamplesAndBindings(t *testing.T) {
	doc, err := os.ReadFile("../../docs/wiki/Datum-File-IO.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```").FindAllStringSubmatch(string(doc), -1)
	expected := []string{`((1 . 2) #\space x #t)`, `#t`, `(foo bar BAZ)`, `#t`, `"猫(1 2)\n"`, `"(#0=(1) #0#)"`, `"#0=(x . #0#)"`, `(1 2 3)`, `42`, `#u8(0 255)`}
	if len(blocks) != len(expected) {
		t.Fatal(len(blocks))
	}
	for i, block := range blocks {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Chdir(t.TempDir())
			got := eval(t, core.NewCore(), block[1])
			nodes, err := reader.ReadAll("expected.scm", expected[i], nil)
			if err != nil || len(nodes) != 1 {
				t.Fatal(err)
			}
			if !equality.Equal(got, nodes[0].Datum()) {
				t.Fatal(i, got, expected[i])
			}
		})
	}
	names := strings.Fields(`read write write-shared write-simple display open-input-file open-output-file open-binary-input-file open-binary-output-file call-with-input-file call-with-output-file with-input-from-file with-output-to-file`)
	catalog, err := os.Open("../../docs/r7rs/exports.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close()
	rows, err := csv.NewReader(catalog).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if _, ok := eval(t, core.NewCore(), name).(*scheme.Procedure); !ok {
			t.Fatal(name)
		}
		if !strings.Contains(string(doc), "("+name+" ") {
			t.Error("missing doc", name)
		}
		found := false
		for _, row := range rows[1:] {
			if row[1] == name {
				found = true
				expectedStatus := "内部Runtimeのimport実装済み・公開CLI未接続"
				if row[3] != "60" || row[5] != expectedStatus {
					t.Error("stale export", row)
				}
			}
		}
		if !found {
			t.Error("missing export", name)
		}
	}
}
