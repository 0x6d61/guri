package core_test

import (
	"context"
	"encoding/csv"
	"errors"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func TestPromiseValuesMemoizationAndLexicalScope(t *testing.T) {
	for _, source := range []string{
		`(define count 0) (define p (delay (begin (set! count (+ count 1)) (vector 42)))) (define alias p) (define result (force p)) (vector-set! result 0 9) (and (= count 1) (eq? result (force alias)) (= (vector-ref (force p) 0) 9))`,
		`(define p (let ((x 42)) (delay x))) (= (force p) 42)`,
		`(define p (make-promise 42)) (and (promise? p) (eq? p (make-promise p)) (= (force p) 42) (not (procedure? p)) (not (promise? (lambda () 42))))`,
		`(define p (delay (delay 42))) (and (promise? (force p)) (= (force (force p)) 42))`,
		`(= (force (delay-force (delay 42))) 42)`,
		`(define p (let ((make-promise (lambda (x) 0))) (delay 42))) (= (force p) 42)`,
		`(define p (delay p)) (eq? p (force p))`,
		`(define f (lambda () 42)) (eq? (force (make-promise f)) f)`,
		`(define count 0) (define p (delay (begin (set! count (+ count 1)) 42))) (guard (e (else #t)) (+ p 1)) (and (= count 0) (= (force p) 42) (= count 1))`,
		`(define-record-type Ordinary (ordinary) ordinary?) (not (promise? (ordinary)))`,
	} {
		if got := eval(t, core.NewCore(), source); got != scheme.True {
			t.Fatal(source, got)
		}
	}
}
func TestPromiseReentrancyFirstCompletedValueWins(t *testing.T) {
	for _, source := range []string{
		`(define count 0) (define p (delay (begin (set! count (+ count 1)) (if (= count 1) (begin (force p) 999) 42)))) (and (= (force p) 42) (= (force p) 42) (= count 2))`,
		`(define count 0) (define p (delay-force (begin (set! count (+ count 1)) (if (= count 1) (begin (force p) 999) (delay 42))))) (and (= (force p) 42) (= count 2))`,
		`(define count 0) (define p (delay (begin (set! count (+ count 1)) (if (> count x) count (force p))))) (define x 5) (define a (force p)) (set! x 10) (and (= a 6) (= (force p) 6) (= count 6))`,
	} {
		if got := eval(t, core.NewCore(), source); got != scheme.True {
			t.Fatal(source, got)
		}
	}
}
func TestPromiseForceDynamicContextAndExceptions(t *testing.T) {
	for _, source := range []string{
		`(define setting (make-parameter 0)) (define p (parameterize ((setting 1)) (delay (setting)))) (define result (parameterize ((setting 42)) (force p))) (and (= result 42) (= (parameterize ((setting 99)) (force p)) 42) (= (setting) 0))`,
		`(define p (with-exception-handler (lambda (e) 1) (lambda () (delay (raise-continuable 'condition))))) (define result (with-exception-handler (lambda (e) 42) (lambda () (force p)))) (and (= result 42) (= (with-exception-handler (lambda (e) 99) (lambda () (force p))) 42))`,
		`(define count 0) (define p (delay (begin (set! count (+ count 1)) (if (= count 1) (error "first") 42)))) (guard (e (else #t)) (force p)) (and (= (force p) 42) (= (force p) 42) (= count 2))`,
		// An aborted force must not split the memoization state of existing aliases.
		`(define count 0) (define target (delay (begin (set! count (+ count 1)) (if (= count 1) (error "retry") count)))) (define q (delay-force target)) (guard (e (else #t)) (force q)) (define p (delay-force target)) (define result (force p)) (and (= result 2) (= (force q) 2) (= (force target) 2) (= count 2))`,
	} {
		if got := eval(t, core.NewCore(), source); got != scheme.True {
			t.Fatal(source, got)
		}
	}
}
func TestPromiseContinuationsAfterEscapeAndCompletion(t *testing.T) {
	e := core.NewCore()
	source := `(define k #f) (define p (delay (call/cc (lambda (resume) (set! k resume) 42)))) (force p)`
	if n := number(eval(t, e, source)); n != 42 {
		t.Fatal(n)
	}
	if n := number(eval(t, e, `(k 99)`)); n != 42 {
		t.Fatal("reentry overwrote cached value", n)
	}
	e = core.NewCore()
	source = `(define k #f) (define p #f) (call/cc (lambda (exit) (set! p (delay (call/cc (lambda (resume) (set! k resume) (exit 1))))) (force p)))`
	if n := number(eval(t, e, source)); n != 1 {
		t.Fatal(n)
	}
	if n := number(eval(t, e, `(k 42)`)); n != 42 {
		t.Fatal(n)
	}
	if n := number(eval(t, e, `(force p)`)); n != 42 {
		t.Fatal(n)
	}
	if n := number(eval(t, e, `(k 99)`)); n != 42 {
		t.Fatal(n)
	}
}
func TestPromiseErrorsAndLazyLibrary(t *testing.T) {
	for _, source := range []string{`(force 42)`, `(force (delay-force 42))`, `(force (delay (values)))`, `(force (delay (values 1 2)))`, `(force (delay-force (values)))`} {
		if got := eval(t, core.NewCore(), `(guard (e (else (error-object? e))) `+source+`)`); got != scheme.True {
			t.Fatal(source, got)
		}
	}
	for _, source := range []string{`(delay)`, `(delay 1 2)`, `(delay-force)`, `(force)`, `(promise? 1 2)`, `(make-promise)`} {
		if _, err := core.NewCore().Eval(context.Background(), "arity.scm", source); err == nil {
			t.Error(source)
		}
	}
	r := core.NewRuntime()
	if n := number(runProgram(t, r, `(import (scheme lazy)) (force (delay-force (delay 42)))`)); n != 42 {
		t.Fatal(n)
	}
	if got := runProgram(t, r, `(import (scheme base) (rename (scheme lazy) (delay later) (delay-force tail-later))) (and (= (force (tail-later (later 42))) 42) (cond-expand ((library (scheme lazy)) #t) (else #f)))`); got != scheme.True {
		t.Fatal(got)
	}
	if _, err := r.Run(context.Background(), "hidden.scm", `(import (scheme base)) (force 42)`); err == nil {
		t.Fatal("lazy leaked into base")
	}
}
func TestDelayForceTailSpace(t *testing.T) {
	e := core.NewCore()
	var baseline uint64
	samples := 0
	e.Bind("sample-heap", vm.Native("sample-heap", 0, 0, func(_ *vm.Context, _ []scheme.Value) (vm.Action, error) {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if samples == 0 {
			baseline = stats.HeapAlloc
		} else if stats.HeapAlloc > baseline+8*1024*1024 {
			t.Errorf("retained heap grew from %d to %d", baseline, stats.HeapAlloc)
		}
		samples++
		return vm.Return(scheme.Unspecified), nil
	}))
	source := `(define (skip n) (delay-force (begin (when (or (= n 99000) (= n 1)) (sample-heap)) (if (= n 0) (delay 42) (skip (- n 1)))))) (force (skip 100000))`
	if n := number(eval(t, e, source)); n != 42 || samples != 2 {
		t.Fatal(n, samples)
	}
	if e.Stats().PeakFrames > 12 {
		t.Fatal("delay-force accumulated continuations", e.Stats())
	}
	for _, source := range []string{
		`(define (loop) (delay-force (loop))) (force (loop))`,
		`(define p (delay-force p)) (force p)`,
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		_, err := core.NewCore().Eval(ctx, "loop.scm", source)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
}

func TestLazyStreamFiltering(t *testing.T) {
	e := core.NewCore()
	source := `(define (from n) (delay (cons n (from (+ n 1)))))
 (define (filter-stream pred stream)
  (delay-force
   (if (null? (force stream)) (delay '())
    (let ((head (car (force stream))) (tail (cdr (force stream))))
     (if (pred head) (delay (cons head (filter-stream pred tail))) (filter-stream pred tail))))))
 (car (force (filter-stream (lambda (n) (= n 10000)) (from 0))))`
	if n := number(eval(t, e, source)); n != 10000 {
		t.Fatal(n)
	}
	if e.Stats().PeakFrames > 20 {
		t.Fatal("lazy stream retained force continuations", e.Stats())
	}
}
func TestLazyWikiExamplesAndBindings(t *testing.T) {
	doc, err := os.ReadFile("../../docs/wiki/Lazy-Evaluation.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```").FindAllStringSubmatch(string(doc), -1)
	expected := []string{`(0 42 42 1)`, `9`, `42`, `(#t #t 42)`, `42`, `(42 42 2)`, `(42 42)`, `(42 42 2)`}
	if len(blocks) != len(expected) {
		t.Fatal(len(blocks))
	}
	for i, block := range blocks {
		got := runProgram(t, core.NewRuntime(), block[1])
		want, err := reader.ReadAll("expected.scm", expected[i], nil)
		if err != nil || len(want) != 1 {
			t.Fatal(err)
		}
		if !equality.Equal(got, want[0].Datum()) {
			t.Fatal(i, got, expected[i])
		}
	}
	f, err := os.Open("../../docs/r7rs/exports.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, row := range rows[1:] {
		if row[0] == "(scheme lazy)" {
			found[row[1]] = true
			if row[3] != "63" || row[5] != "公開CLIのimport実装済み" {
				t.Fatal(row)
			}
			if !strings.Contains(string(doc), "("+row[1]+" ") {
				t.Fatal("missing lazy reference", row[1])
			}
		}
	}
	if len(found) != 5 {
		t.Fatal(found)
	}
}
