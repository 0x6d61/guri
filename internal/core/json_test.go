package core_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"encoding/json"
	"errors"
	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/writer"
	"io"
)

func jsonCheck(t *testing.T, body string) {
	t.Helper()
	if got := runProgram(t, core.NewRuntime(), `(import (scheme base) (scheme inexact) (srfi 180)) `+body); got != scheme.True {
		t.Fatal(got)
	}
}

func TestJSONRead(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`null`, `'null`}, {`true`, `#t`}, {`false`, `#f`},
		{`123456789012345678901234567890`, `123456789012345678901234567890`},
		{`-0`, `0`}, {`1.25e2`, `125.0`}, {`[]`, `'#()`}, {`{}`, `'()`},
		{`{"a":1,"a":2,"b":[null,true,{"日本語":"😀"}]}`, `'((a . 1) (a . 2) (b . #(null #t ((日本語 . "😀")))))`},
		{`"\u65e5\ud83d\ude00\n\t\/\\\""`, `"日😀\n\t/\\\""`},
	} {
		t.Run(tc.input, func(t *testing.T) {
			jsonCheck(t, `(equal? (json-read (open-input-string `+jsonQuote(tc.input)+`)) `+tc.want+`)`)
		})
	}
	jsonCheck(t, `(define p (open-input-string "42 [1] true null \"end\""))
 (and (= (json-read p) 42) (equal? (json-read p) '#(1)) (json-read p)
      (json-null? (json-read p)) (equal? (json-read p) "end") (eof-object? (json-read p)) (input-port-open? p))`)
	jsonCheck(t, `(define p (open-input-string "0.0 -0.0 1e-9999"))
 (and (inexact? (json-read p)) (negative? (/ 1.0 (json-read p))) (= (json-read p) 0.0))`)
}

func TestJSONInvalidInput(t *testing.T) {
	for _, input := range []string{`[`, `{`, `[1,]`, `{"a":1,}`, `{"a" 1}`, `{"a":}`, `{1:2}`, `[1 2]`, `[truefalse]`, `tru`, `NULL`, `+1`, `01`, `1.`, `.1`, `1e`, `--1`, `1e9999`, `"\x20"`, `"\ud800"`, `"\udc00"`, `"\ud800\u0020"`, "\"a\nb\"", `"unterminated`, "\x00", "\ufeffnull", `1x`, `-Infinity`} {
		t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
			jsonCheck(t, `(guard (e (else (and (json-error? e) (string? (json-error-reason e))))) (json-read (open-input-string `+jsonQuote(input)+`)) #f)`)
		})
	}
}

func TestJSONGeneratorAndFold(t *testing.T) {
	jsonCheck(t, `(define p (open-input-string "{\"a\":[1,null]} 99"))
 (define g (json-generator p))
 (define (drain g) (let ((x (g))) (if (eof-object? x) '() (cons x (drain g)))))
 (and (equal? (drain g) '(object-start "a" array-start 1 null array-end object-end))
      (eof-object? (g)) (= (json-read p) 99))`)
	jsonCheck(t, `(define p (open-input-string "[1,{\"x\":2}]"))
 (define chars (lambda () (read-char p)))
 (equal? (json-read chars) '#(1 ((x . 2))))`)
	jsonCheck(t, `(equal? (json-fold cons (lambda (s) '()) reverse (lambda (s) '()) reverse '()
 (open-input-string "[1,{\"x\":2},[3]]")) '((1 ("x" 2) (3))))`)
	jsonCheck(t, `(eq? (json-fold cons values values values values 'empty (open-input-string "  ")) 'empty)`)
}

func TestJSONLimits(t *testing.T) {
	jsonCheck(t, `(and (= (json-nesting-depth-limit) +inf.0) (= (json-number-of-character-limit) +inf.0)
 (parameterize ((json-nesting-depth-limit 2)) (equal? (json-read (open-input-string "[0]")) '#(0)))
 (guard (e (else (json-error? e))) (parameterize ((json-nesting-depth-limit 2)) (json-read (open-input-string "[[]]"))) #f)
 (guard (e (else (json-error? e))) (parameterize ((json-number-of-character-limit 4)) (json-read (open-input-string "null"))) #f)
 (parameterize ((json-number-of-character-limit 5)) (json-null? (json-read (open-input-string "null"))))
 (= (json-nesting-depth-limit) +inf.0))`)
}

func jsonQuote(text string) string {
	v, err := scheme.NewString(text)
	if err != nil {
		panic(err)
	}
	var out strings.Builder
	if err := writer.Write(context.Background(), &out, v, writer.Normal); err != nil {
		panic(err)
	}
	return out.String()
}

func TestJSONWrite(t *testing.T) {
	jsonCheck(t, `(define p (open-output-string))
 (json-write '((a . #(1 null #t "日本語😀")) (a . 2)) p)
 (and (output-port-open? p) (equal? (json-read (open-input-string (get-output-string p))) '((a . #(1 null #t "日本語😀")) (a . 2))))`)
	jsonCheck(t, `(define p (open-output-string))
 (parameterize ((current-output-port p)) (json-write 2.0))
 (and (inexact? (json-read (open-input-string (get-output-string p))))
 (let ((p (open-input-string "[0]")))
 (parameterize ((current-input-port p)) (equal? (json-read) '#(0)))))`)
	jsonCheck(t, `(define chunks '())
 (json-write '#("line\n\t\x0;" -0.0 999999999999999999999999999999999999) (lambda (x) (set! chunks (cons x chunks))))
 (define text (apply string-append (reverse chunks)))
 (equal? (json-read (open-input-string text)) '#("line\n\t\x0;" -0.0 999999999999999999999999999999999999))`)
}

func TestJSONWriteValidation(t *testing.T) {
	for _, value := range []string{`1/3`, `1+2i`, `+inf.0`, `+nan.0`, `'other`, `#\a`, `'#u8(1)`, `'(("key" . 1))`, `'(1 2)`, `'(a . b)`, `(vector 1 (cons 'a 2))`, `(lambda () 1)`, `(let ((v (vector 0))) (vector-set! v 0 v) v)`, `(let ((p (cons (cons 'a 1) '()))) (set-cdr! p p) p)`} {
		t.Run(value, func(t *testing.T) {
			jsonCheck(t, `(define p (open-output-string))
   (and (guard (e (else (json-error? e))) (json-write `+value+` p) #f) (equal? (get-output-string p) ""))`)
		})
	}
	jsonCheck(t, `(define shared '#(1 2)) (define p (open-output-string))
 (json-write (vector shared shared) p) (equal? (get-output-string p) "[[1,2],[1,2]]")`)
}

func TestJSONAccumulator(t *testing.T) {
	jsonCheck(t, `(define p (open-output-string)) (define put (json-accumulator p))
 (for-each put '(object-start "x" array-start 1 #f null array-end "x" "v" object-end))
 (equal? (json-read (open-input-string (get-output-string p))) '((x . #(1 #f null)) (x . "v")))`)
	for _, events := range []string{`'(array-end)`, `'(object-end)`, `'(object-start 1)`, `'(object-start "x" object-end)`, `'(array-start object-end)`, `'(1 2)`, `'(object-start "x" invalid)`, `'(array-start 1/3)`} {
		t.Run(events, func(t *testing.T) {
			jsonCheck(t, `(define put (json-accumulator (open-output-string)))
  (guard (e (else (json-error? e))) (for-each put `+events+`) #f)`)
		})
	}
	jsonCheck(t, `(define out (open-output-string))
 (define put (json-accumulator (lambda (x) (if (char? x) (write-char x out) (write-string x out)))))
 (for-each put '(array-start 1 array-end)) (equal? (get-output-string out) "[1]")`)
}

func TestJSONLinesAndSequences(t *testing.T) {
	jsonCheck(t, `(define p (open-input-string "1\n{\"a\":2}\n[3]\n"))
 (define g (json-lines-read p))
 (and (= (g) 1) (equal? (g) '((a . 2))) (equal? (g) '#(3)) (eof-object? (g)) (eof-object? (g)) (input-port-open? p))`)
	jsonCheck(t, `(define p (open-input-string "\x1e;1\n\x1e;{\"a\":2}\n\x1e;[3]\n"))
 (define g (json-sequence-read (lambda () (read-char p))))
 (and (= (g) 1) (equal? (g) '((a . 2))) (equal? (g) '#(3)) (eof-object? (g)) (input-port-open? p))`)
	jsonCheck(t, `(define g (json-lines-read (open-input-string "1\n[bad]\n")))
 (and (= (g) 1) (guard (e (else (json-error? e))) (g) #f))`)
}

func TestJSONArgumentErrors(t *testing.T) {
	for _, call := range []string{
		`(json-read 1)`, `(json-read (open-input-bytevector #u8(49)))`, `(json-read (open-output-string))`,
		`(json-read (lambda () 1))`, `(json-read (lambda () (values)))`, `(json-read (lambda () (values #\a #\b)))`,
		`(json-generator 1)`, `(json-write 1 2)`, `(json-write 1 (open-input-string ""))`,
		`(json-accumulator 1)`, `(json-fold 1 values values values values 0)`,
		`(json-error-reason 1)`, `(json-lines-read 1)`, `(json-sequence-read 1)`,
		`(parameterize ((json-nesting-depth-limit -1)) #t)`, `(parameterize ((json-number-of-character-limit +nan.0)) #t)`,
		`(parameterize ((json-nesting-depth-limit 'bad)) #t)`,
	} {
		t.Run(call, func(t *testing.T) { jsonCheck(t, `(guard (e (else (json-error? e))) `+call+` #f)`) })
	}
	for _, name := range []string{"json-null?", "json-error?", "json-error-reason", "json-write", "json-accumulator", "json-fold"} {
		t.Run(name+" arity", func(t *testing.T) { jsonCheck(t, `(guard (e (else (json-error? e))) (`+name+`) #f)`) })
	}
	jsonCheck(t, `(and (not (json-error? 1)) (not (json-null? #f)) (not (json-null? '()))
  (let ((p (open-input-string "1"))) (close-port p) (guard (e (else (json-error? e))) (json-read p) #f)))`)
}

func TestJSONStreamingAndRecovery(t *testing.T) {
	jsonCheck(t, `(define calls 0)
 (define g (json-generator (lambda () (set! calls (+ calls 1)) (if (= calls 1) #\[ (error "read too far")))))
 (and (= calls 0) (eq? (g) 'array-start) (= calls 1))`)
	jsonCheck(t, `(define p (open-input-string "\x1e;1\n"))
 (define g (json-sequence-read (lambda () (if (eof-object? (peek-char p)) (error "read too far") (read-char p)))))
 (= (g) 1)`)
	jsonCheck(t, `(define g (json-sequence-read (open-input-string "junk\x1e;[bad]\x1e;123\x1e;trueX\x1e;[\x1e;[1]\x1e;2\n\x1e;null")))
 (and (equal? (g) '#(1)) (= (g) 2) (eof-object? (g)) (eof-object? (g)))`)
	jsonCheck(t, `(define g (json-sequence-read (open-input-string "\x1e;{\n\"a\":1,\n\"b\":2}\n")))
 (equal? (g) '((a . 1) (b . 2)))`)
	jsonCheck(t, `(define p (open-input-string "1 2 [3]")) (define chars (lambda () (read-char p)))
 (and (= (json-read chars) 1) (= (json-read chars) 2) (equal? (json-read chars) '#(3)) (eof-object? (json-read chars)))`)
	jsonCheck(t, `(guard (e (else (eq? e 'source-error))) (json-read (lambda () (raise 'source-error))) #f)`)
	jsonCheck(t, `(guard (e (else (eq? e 'sink-error))) (json-write 1 (lambda (s) (raise 'sink-error))) #f)`)
	jsonCheck(t, `(define g (json-sequence-read (open-input-string "\x1e;123456789")))
 (guard (e (else (json-error? e))) (parameterize ((json-number-of-character-limit 5)) (g)) #f)`)
	jsonCheck(t, `(define g (json-sequence-read (open-input-string "\x1e;[[]]\n")))
 (guard (e (else (json-error? e))) (parameterize ((json-nesting-depth-limit 2)) (g)) #f)`)
	jsonCheck(t, `(and (parameterize ((json-number-of-character-limit 4)) (equal? (json-read (open-input-string "\"日\"")) "日"))
  (guard (e (else (json-error? e))) (parameterize ((json-number-of-character-limit 3)) (json-read (open-input-string "\"日\""))) #f))`)
}

func TestJSONLargeAndDeepValues(t *testing.T) {
	const count = 12000
	input := "[" + strings.Repeat("0,", count-1) + "0]"
	jsonCheck(t, fmt.Sprintf(`(= (vector-length (json-read (open-input-string %s))) %d)`, jsonQuote(input), count))
	deep := strings.Repeat("[", count) + "0" + strings.Repeat("]", count)
	jsonCheck(t, `(define text `+jsonQuote(deep)+`) (define out (open-output-string))
 (json-write (json-read (open-input-string text)) out) (equal? text (get-output-string out))`)
	jsonCheck(t, `(define n 0) (define size 40001)
 (define (chars) (set! n (+ n 1)) (cond ((= n 1) #\[) ((= n size) #\]) ((> n size) (eof-object)) ((even? n) #\0) (else #\,)))
 (= (json-fold (lambda (x s) (+ x s)) (lambda (s) 0) values values values 0 chars) 0)`)
}

type jsonBrokenIO struct {
	closes int
	short  bool
}

func (b *jsonBrokenIO) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (b *jsonBrokenIO) Write(p []byte) (int, error) {
	if b.short {
		return 0, nil
	}
	return 0, io.ErrClosedPipe
}
func (b *jsonBrokenIO) Close() error { b.closes++; return nil }
func TestJSONBorrowedIOFailures(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		short        bool
	}{
		{"read", `(json-read)`, false}, {"write", `(json-write 1)`, false}, {"short write", `(json-write 1)`, true},
		{"generator", `((json-generator))`, false}, {"accumulator", `((json-accumulator (current-output-port)) 1)`, false},
		{"lines", `((json-lines-read))`, false}, {"sequence", `((json-sequence-read))`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &jsonBrokenIO{short: tc.short}
			in := ports.Input(adapter, false, ports.Options{})
			out := ports.Output(adapter, false, ports.Options{})
			diagnostic := ports.MemoryOutput(false)
			runtime := core.NewRuntime()
			if err := runtime.SetStandardPorts(in, out, diagnostic); err != nil {
				t.Fatal(err)
			}
			if got := runProgram(t, runtime, `(import (scheme base) (srfi 180)) (guard (e (else (json-error? e))) `+tc.source+` #f)`); got != scheme.True {
				t.Fatal(got)
			}
			if adapter.closes != 0 || !in.Open() || !out.Open() {
				t.Fatal("borrowed resource closed")
			}
			data, err := diagnostic.OutputBytes(false)
			if err != nil || len(data) != 0 {
				t.Fatal(string(data), err)
			}
		})
	}
}

type jsonCancelReader struct {
	cancel context.CancelFunc
	calls  int
}

func (r *jsonCancelReader) Read(p []byte) (int, error) {
	r.calls++
	if r.calls == 8 {
		r.cancel()
	}
	p[0] = ' '
	return 1, nil
}
func TestJSONCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input := &jsonCancelReader{cancel: cancel}
	runtime := core.NewRuntime()
	if err := runtime.SetStandardPorts(ports.Input(input, false, ports.Options{}), ports.MemoryOutput(false), ports.MemoryOutput(false)); err != nil {
		t.Fatal(err)
	}
	_, err := runtime.Run(ctx, "cancel.scm", `(import (srfi 180)) (json-read)`)
	if !errors.Is(err, context.Canceled) || input.calls < 8 {
		t.Fatal(input.calls, err)
	}
}

func TestJSONWriteSnapshotAndSharing(t *testing.T) {
	jsonCheck(t, `(define tail (list (cons 'b 2))) (define obj (cons (cons 'a tail) tail))
 (define p (open-output-string)) (json-write obj p)
 (equal? (json-read (open-input-string (get-output-string p))) '((a . ((b . 2))) (b . 2)))`)
	jsonCheck(t, `(define v (vector (make-string 5000 #\a) "original"))
 (define p (open-output-string))
 (json-write v (lambda (s) (vector-set! v 1 'invalid) (write-string s p)))
 (equal? (vector-ref (json-read (open-input-string (get-output-string p))) 1) "original")`)
	jsonCheck(t, `(define put (json-accumulator (open-output-string)))
 (and (guard (e (else (json-error? e))) (put 'invalid) #f)
      (begin (put 1) #t))`)
}

func TestJSONLinesEOFIsFinal(t *testing.T) {
	jsonCheck(t, `(define calls 0)
 (define next (json-lines-read (lambda () (set! calls (+ calls 1)) (if (= calls 2) #\0 (eof-object)))))
 (and (eof-object? (next)) (eof-object? (next)))`)
}

func FuzzJSONReadWrite(f *testing.F) {
	for _, text := range []string{`null`, `{"x":[1,2.0,true,null],"x":"日本語"}`, `"\ud83d\ude00"`, `[`, `1e9999`, `1e-9999`, `"\ud800"`, `truefalse`, "\xff"} {
		f.Add([]byte(text))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			return
		}
		runtime := core.NewRuntime()
		output := ports.MemoryOutput(false)
		if err := runtime.SetStandardPorts(ports.Input(strings.NewReader(string(data)), false, ports.Options{}), output, ports.MemoryOutput(false)); err != nil {
			t.Fatal(err)
		}
		result, err := runtime.Run(context.Background(), "fuzz.scm", `(import (scheme base) (srfi 180))
   (guard (e (else #f)) (let ((v (json-read))) (if (or (eof-object? v) (not (eof-object? (json-read)))) #f (begin (json-write v) #t))))`)
		if err != nil {
			t.Fatal(err)
		}
		value, err := result.Single()
		if err != nil {
			t.Fatal(err)
		}
		if value == scheme.True {
			encoded, err := output.OutputBytes(false)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(data) || !json.Valid(encoded) {
				t.Fatalf("accepted %q; wrote %q", data, encoded)
			}
		}
	})
}
