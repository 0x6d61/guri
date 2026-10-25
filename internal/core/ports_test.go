package core_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestStandardMemoryPorts(t *testing.T) {
	tests := map[string]string{
		`(list (port? 1) (input-port? (open-input-string "")) (output-port? (open-output-string)) (textual-port? (open-input-string "")) (binary-port? (open-input-bytevector #u8())))`:                                                    `(#f #t #t #t #t)`,
		`(define p (open-input-string "λ猫")) (list (peek-char p) (peek-char p) (read-char p) (read-char p) (eof-object? (read-char p)) (char-ready? p) (input-port-open? p))`:                                                              `(#\λ #\λ #\λ #\猫 #t #t #t)`,
		`(define p (open-input-bytevector #u8(0 255))) (list (peek-u8 p) (read-u8 p) (read-u8 p) (eof-object? (peek-u8 p)) (u8-ready? p) (eof-object? (eof-object)))`:                                                                      `(0 0 255 #t #t #t)`,
		`(define p (open-input-string "ab\r\ncd\ref\ngh")) (list (read-line p) (read-line p) (read-line p) (read-line p) (eof-object? (read-line p)))`:                                                                                     `("ab" "cd" "ef" "gh" #t)`,
		`(define p (open-input-string "λ猫a")) (list (read-string 0 p) (read-string 2 p) (read-string 20 p) (eof-object? (read-string 1 p)))`:                                                                                               `("" "λ猫" "a" #t)`,
		`(define p (open-output-string)) (write-char #\λ p) (write-string "a猫b" p 1 2) (newline p) (flush-output-port p) (list (get-output-string p) (begin (close-port p) (get-output-string p)) (output-port? p) (output-port-open? p))`: `("λ猫\n" "λ猫\n" #t #f)`,
		`(define p (open-output-bytevector)) (write-u8 255 p) (write-bytevector #u8(1 2 3) p 1 3) (define a (get-output-bytevector p)) (bytevector-u8-set! a 0 0) (close-port p) (get-output-bytevector p)`:                                `#u8(255 2 3)`,
		`(define p (open-input-bytevector #u8(1 2 3))) (define b (make-bytevector 5 9)) (list (read-bytevector 0 p) (read-bytevector! b p 1 3) b (read-bytevector 8 p) (eof-object? (read-bytevector 1 p)))`:                               `(#u8() 2 #u8(9 1 2 9 9) #u8(3) #t)`,
		`(define p (open-input-string "x")) (close-input-port p) (close-port p) (list (input-port? p) (input-port-open? p) (textual-port? p))`:                                                                                             `(#t #f #t)`,
	}
	for src, want := range tests {
		expected, err := reader.ReadAll("expected.scm", want, scheme.NewSymbolTable())
		if err != nil || len(expected) != 1 {
			t.Fatal(want, err)
		}
		if got := eval(t, core.NewCore(), src); !equality.Equal(got, expected[0].Datum()) {
			t.Errorf("%s: got %v want %s", src, got, want)
		}
	}
}

func TestPortParametersAndContinuations(t *testing.T) {
	e := core.NewCore()
	if got := repr(eval(t, e, `(list (input-port? (current-input-port)) (output-port? (current-output-port)) (output-port? (current-error-port)) (textual-port? (current-input-port)))`)); got != `(#t #t #t #t)` {
		t.Fatal(got)
	}
	if got := repr(eval(t, e, `(define outer (current-output-port)) (define out (open-output-string)) (define k #f) (parameterize ((current-output-port out)) (call/cc (lambda (c) (set! k c) 0)) (write-char #\x)) (list (eq? outer (current-output-port)) (get-output-string out))`)); got != `(#t "x")` {
		t.Fatal(got)
	}
	eval(t, e, `(k 1)`)
	if got := repr(eval(t, e, `(get-output-string out)`)); got != `"xx"` {
		t.Fatal(got)
	}
	if got := repr(eval(t, e, `(define p (open-input-string "a")) (guard (e (else (eq? outer (current-output-port)))) (parameterize ((current-input-port p) (current-output-port out)) (write-string "ok") (error "escape")))`)); got != `#t` {
		t.Fatal(got)
	}
	if got := eval(t, e, `(parameterize ((current-input-port p)) (char=? (read-char) #\a))`); got != scheme.True {
		t.Fatal(got)
	}
}

func TestCallWithPortLifecycleAndValues(t *testing.T) {
	e := core.NewCore()
	r, err := e.Eval(context.Background(), "port.scm", `(define p (open-output-string)) (call-with-port p (lambda (port) (write-string "ok" port) (values 20 22)))`)
	if err != nil || r.Len() != 2 || number(r.Values()[0]) != 20 || number(r.Values()[1]) != 22 {
		t.Fatal(r, err)
	}
	if eval(t, e, `(output-port-open? p)`) != scheme.False {
		t.Fatal("normal return left open")
	}
	if got := repr(eval(t, e, `(define resume #f) (define q (open-output-string)) (call/cc (lambda (exit) (call-with-port q (lambda (p) (call/cc (lambda (c) (set! resume c) (exit 1))) (write-string "r" p) 42))))`)); got != `1` {
		t.Fatal(got)
	}
	if eval(t, e, `(output-port-open? q)`) != scheme.True {
		t.Fatal("escape closed port")
	}
	if number(eval(t, e, `(resume 0)`)) != 42 || eval(t, e, `(output-port-open? q)`) != scheme.False {
		t.Fatal("resume did not close")
	}
	if _, err := e.Eval(context.Background(), "closed.scm", `(resume 0)`); err == nil {
		t.Fatal("closed continuation resumed I/O")
	}
	if got := repr(eval(t, core.NewCore(), `(define p (open-output-string)) (guard (e (else (output-port-open? p))) (call-with-port p (lambda (p) (error "escape"))))`)); got != `#t` {
		t.Fatal("exception closed port", got)
	}
}

func TestPortErrors(t *testing.T) {
	for _, src := range []string{
		`(read-char (open-input-bytevector #u8(1)))`, `(read-u8 (open-input-string "a"))`, `(get-output-string (open-output-bytevector))`, `(get-output-bytevector (open-output-string))`,
		`(write-u8 256 (open-output-bytevector))`, `(write-u8 -1 (open-output-bytevector))`, `(write-char 1 (open-output-string))`, `(close-input-port (open-output-string))`, `(close-output-port (open-input-string ""))`, `(input-port-open? 1)`,
		`(read-string -1 (open-input-string ""))`, `(write-string "a" (open-output-string) 1 0)`, `(write-bytevector #u8(1) (open-output-bytevector) 0 2)`,
		`(define p (open-input-string "a")) (close-port p) (peek-char p)`, `(define p (open-output-string)) (close-port p) (write-string "" p)`, `(parameterize ((current-output-port (open-input-string ""))) 1)`,
		`(read-bytevector! '#u8() (open-input-bytevector #u8()))`,
	} {
		if got := repr(eval(t, core.NewCore(), `(guard (e (else (error-object? e))) `+src+`)`)); got != `#t` {
			t.Errorf("%s: %s", src, got)
		}
	}
	for _, src := range []string{`(open-input-string)`, `(close-port)`, `(eof-object 1)`, `(call-with-port (open-output-string))`, `(read-char 1 2)`} {
		if _, err := core.NewCore().Eval(context.Background(), "arity.scm", src); err == nil {
			t.Error(src)
		}
	}
}

func TestPortAdapterFailuresAndAliases(t *testing.T) {
	for _, owned := range []bool{false, true} {
		r := &portTestWriter{}
		options := ports.Options{}
		if owned {
			options.Owned = r
		}
		e := core.NewCore()
		e.Bind("out", ports.Value(ports.Output(r, false, options)))
		if got := repr(eval(t, e, `(define alias out) (write-string "λ" alias) (flush-output-port out) (close-output-port alias) (list (output-port? out) (output-port-open? out))`)); got != `(#t #f)` {
			t.Fatal(got)
		}
		expected := 0
		if owned {
			expected = 1
		}
		if r.closed != expected || r.flushes != 2 || r.String() != "λ" {
			t.Fatal(owned, r)
		}
	}
	r := &portTestWriter{failure: errors.New("flush broke")}
	e := core.NewCore()
	e.Bind("out", ports.Value(ports.Output(r, false, ports.Options{Owned: r})))
	if got := repr(eval(t, e, `(guard (e (else (list (error-object? e) (output-port-open? out)))) (close-port out))`)); got != `(#t #f)` || r.closed != 1 {
		t.Fatal(got, r)
	}
	e = core.NewCore()
	e.Bind("bad", ports.Value(ports.Input(bytes.NewReader([]byte{0xff}), false, ports.Options{Ready: func() (bool, error) { return true, nil }})))
	if got := repr(eval(t, e, `(guard (e (else (error-object? e))) (read-char bad))`)); got != `#t` {
		t.Fatal(got)
	}
	_, err := e.Eval(context.Background(), "invalid-utf8.scm", "\n(read-char bad)")
	var raised *scheme.Raised
	if !errors.As(err, &raised) || raised.Source().Filename != "invalid-utf8.scm" || raised.Source().Line != 2 {
		t.Fatal("lost source or Scheme exception", err)
	}
}

type portTestWriter struct {
	bytes.Buffer
	closed, flushes int
	failure         error
}

func (w *portTestWriter) Close() error { w.closed++; return nil }
func (w *portTestWriter) Flush() error { w.flushes++; return w.failure }

func TestPortReadDestinationsAndDynamicDefaults(t *testing.T) {
	for _, src := range []string{
		`(define s (string #\a)) (define p (open-input-string s)) (string-set! s 0 #\b) (char=? (read-char p) #\a)`,
		`(define b (bytevector 1)) (define p (open-input-bytevector b)) (bytevector-u8-set! b 0 9) (= (read-u8 p) 1)`,
		`(define p (open-output-string)) (write-string "ab" p) (define s (get-output-string p)) (string-set! s 0 #\x) (string=? (get-output-string p) "ab")`,
		`(define p (open-input-string "")) (and (string=? (read-string 0 p) "") (eof-object? (read-char p)) (char-ready? p))`,
		`(define p (open-input-bytevector #u8())) (and (= (read-bytevector! (make-bytevector 0) p) 0) (equal? (read-bytevector 0 p) #u8()) (eof-object? (read-u8 p)))`,
		`(define p (open-input-bytevector #u8(42))) (guard (e (else #t)) (read-bytevector! #u8(0) p)) (= (read-u8 p) 42)`,
		`(define p (open-input-bytevector #u8(42))) (guard (e (else #t)) (read-bytevector! (make-bytevector 1) p 2)) (= (read-u8 p) 42)`,
		`(define p (open-input-bytevector #u8(42))) (read-bytevector! (make-bytevector 0) p) (= (read-u8 p) 42)`,
		`(define p (open-input-string "x")) (define alias p) (read-char alias) (and (eof-object? (peek-char p)) (input-port-open? p))`,
		`(define p (open-input-string "λ")) (string=? (read-string 10000000000000000000000000000000000000 p) "λ")`,
		`(define p (open-input-bytevector #u8(42))) (parameterize ((current-input-port p)) (= (read-u8) 42))`,
		`(define p (open-output-bytevector)) (parameterize ((current-output-port p)) (write-u8 42) (write-bytevector #u8(1 2))) (equal? (get-output-bytevector p) #u8(42 1 2))`,
		`(define p (open-output-string)) (define saved (current-error-port)) (parameterize ((current-error-port p)) (write-string "err" (current-error-port))) (and (eq? saved (current-error-port)) (string=? (get-output-string p) "err"))`,
		`(define p (open-output-string)) (parameterize ((current-output-port p)) (let ((current-output-port (lambda () #f))) (newline))) (string=? (get-output-string p) "\n")`,
	} {
		if got := eval(t, core.NewCore(), src); got != scheme.True {
			t.Fatal(src, got)
		}
	}
	e := core.NewCore()
	r, err := e.Eval(context.Background(), "zero-values.scm", `(define p (open-output-string)) (call-with-port p (lambda (p) (values)))`)
	if err != nil || r.Len() != 0 || eval(t, e, `(output-port-open? p)`) != scheme.False {
		t.Fatal(r, err)
	}
}

func TestPortWikiExamplesAndBindings(t *testing.T) {
	doc, err := os.ReadFile("../../docs/wiki/Ports.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```").FindAllStringSubmatch(string(doc), -1)
	expected := []string{`(#\λ #\λ #\猫 #t #t)`, `#u8(255 2 3)`, `"λ\n"`, `20 22`, `#t`, `("ab" "猫" #t)`, `#u8(9 1 2 9)`, `"猫λ"`}
	if len(blocks) != len(expected) {
		t.Fatal("reference example count", len(blocks))
	}
	for i, block := range blocks {
		e := core.NewCore()
		got, err := e.Eval(context.Background(), "Ports.md", block[1])
		if err != nil {
			t.Fatal(i, err)
		}
		nodes, err := reader.ReadAll("expected.scm", expected[i], scheme.NewSymbolTable())
		if err != nil {
			t.Fatal(err)
		}
		var want []scheme.Value
		for _, node := range nodes {
			want = append(want, node.Datum())
		}
		if !equality.Equal(scheme.List(got.Values()...), scheme.List(want...)) {
			t.Fatal(i, got, expected[i])
		}
	}
	names := strings.Fields(`port? input-port? output-port? textual-port? binary-port? input-port-open? output-port-open? close-port close-input-port close-output-port call-with-port open-input-string open-output-string get-output-string open-input-bytevector open-output-bytevector get-output-bytevector eof-object eof-object? read-char peek-char char-ready? read-line read-string read-u8 peek-u8 u8-ready? read-bytevector read-bytevector! write-char write-string write-u8 write-bytevector newline flush-output-port current-input-port current-output-port current-error-port`)
	if len(names) != 38 {
		t.Fatal(len(names))
	}
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
			t.Fatal("missing procedure", name)
		}
		if !strings.Contains(string(doc), "("+name+" ") && !strings.Contains(string(doc), "("+name+")") {
			t.Error("missing reference", name)
		}
		found := false
		for _, row := range rows[1:] {
			if row[0] == "(scheme base)" && row[1] == name {
				found = true
				if row[3] != "59" || row[5] != "公開CLIのimport実装済み" {
					t.Error("stale export", row)
				}
			}
		}
		if !found {
			t.Error("missing base export", name)
		}
	}
}
