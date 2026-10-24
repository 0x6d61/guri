package core_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestEvalDatumAndImmutableEnvironments(t *testing.T) {
	for _, source := range []string{
		`(import (scheme base) (scheme eval)) (= (eval '(* 7 6) (environment '(scheme base))) 42)`,
		`(import (scheme base) (scheme eval)) (= (let ((x 100)) (eval '(let ((x 40)) (+ x 2)) (environment '(scheme base)))) 42)`,
		`(import (scheme base) (scheme eval)) (= (eval '(sum 40 2) (environment '(rename (only (scheme base) +) (+ sum)))) 42)`,
		`(import (scheme base) (scheme eval)) (define e (environment '(scheme base))) (and (guard (x (else (error-object? x))) (eval '(define foo 32) e) #f) (guard (x (else #t)) (eval '(set! + 1) e) #f) (= (eval '(+ 40 2) e) 42))`,
		`(import (scheme base) (scheme eval)) (guard (x (else (error-object? x))) (eval '(define-syntax m (syntax-rules () ((_ x) x))) (environment '(scheme base))) #f)`,
		`(import (scheme base) (scheme eval)) (guard (x (else #t)) (eval '(+ 1 2) (environment)) #f)`,
		`(import (scheme base) (scheme eval)) (= (eval '(let ((x 40)) (define y 2) (+ x y)) (environment '(scheme base))) 42)`,
	} {
		got := runProgram(t, core.NewRuntime(), source)
		if got != scheme.True {
			t.Fatal(source, got)
		}
	}
}
func TestEvalMultipleValuesDynamicContextAndContinuations(t *testing.T) {
	for _, source := range []string{
		`(import (scheme base) (scheme eval)) (= (call-with-values (lambda () (eval '(values 20 22) (environment '(scheme base)))) +) 42)`,
		`(import (scheme base) (scheme eval)) (= (call-with-values (lambda () (eval '(values) (environment '(scheme base)))) (lambda () 42)) 42)`,
		`(import (scheme base) (scheme eval)) (= (with-exception-handler (lambda (x) 42) (lambda () (eval '(raise-continuable 'test) (environment '(scheme base))))) 42)`,
		`(import (scheme base) (scheme eval)) (= (call/cc (lambda (exit) ((eval '(lambda (k) (k 42)) (environment '(scheme base))) exit))) 42)`,
		`(import (scheme base) (scheme eval)) (define x (list 'quote (vector 42))) (eq? (eval x (environment '(scheme base))) (cadr x))`,
		`(import (scheme base) (scheme eval)) (define x (cons '+ '())) (set-cdr! x x) (guard (e (else #t)) (eval x (environment '(scheme base))) #f)`,
	} {
		if got := runProgram(t, core.NewRuntime(), source); got != scheme.True {
			t.Fatal(source, got)
		}
	}
}
func TestR5RSEnvironments(t *testing.T) {
	for _, source := range []string{
		`(import (scheme base) (scheme r5rs)) (= (eval '(* 6 7) (scheme-report-environment 5)) 42)`,
		`(import (scheme base) (scheme r5rs)) (define f (eval '(lambda (f x) (f x x)) (null-environment 5))) (= (f + 21) 42)`,
		`(import (scheme base) (scheme r5rs)) (and (guard (e (else #t)) (null-environment 7) #f) (guard (e (else #t)) (scheme-report-environment 7) #f) (guard (e (else #t)) (eval '+ (null-environment 5)) #f))`,
	} {
		if got := runProgram(t, core.NewRuntime(), source); got != scheme.True {
			t.Fatal(source, got)
		}
	}
	// Strings are data, not source text to execute.
	r := core.NewRuntime()
	result, err := r.Run(context.Background(), "datum.scm", `(import (scheme eval)) (eval "(+ 1 2)" (environment))`)
	if err != nil || result.Len() != 1 || result.Values()[0].(*scheme.String).Text() != "(+ 1 2)" {
		t.Fatal(result, err)
	}
}

func TestInteractionEnvironmentIsolationAndRecovery(t *testing.T) {
	r := core.NewRuntime()
	for _, source := range []string{
		`(import (scheme base) (scheme eval) (scheme repl)) (define e (interaction-environment)) (eval '(define answer 40) e) (= (eval '(begin (set! answer (+ answer 2)) answer) e) 42)`,
		`(import (scheme base) (scheme eval) (scheme repl)) (= (eval 'answer (interaction-environment)) 42)`,
		`(import (scheme base) (scheme eval) (scheme repl)) (eval '(set! + (lambda args 99)) (interaction-environment)) (and (= (+ 20 22) 42) (= (eval '(+ 20 22) (interaction-environment)) 99) (= (eval '(+ 20 22) (environment '(scheme base))) 42))`,
		`(import (scheme base) (scheme eval) (scheme repl)) (define e (interaction-environment)) (guard (x (else #t)) (eval '(begin (define broken 1) (if)) e)) (eval '(define broken 42) e) (= (eval 'broken e) 42)`,
	} {
		if got := runProgram(t, r, source); got != scheme.True {
			t.Fatal(source, got)
		}
	}
	if _, err := core.NewRuntime().Run(context.Background(), "fresh.scm", `(import (scheme base) (scheme eval) (scheme repl)) (eval 'answer (interaction-environment))`); err == nil {
		t.Fatal("interaction bindings leaked across sessions")
	}

}
func TestEvalSavedContinuation(t *testing.T) {
	r := core.NewRuntime()
	if n := number(runProgram(t, r, `(import (scheme base) (scheme eval) (scheme repl)) (eval '(begin (define resume #f) (+ 10 (call/cc (lambda (k) (set! resume k) 32)))) (interaction-environment))`)); n != 42 {
		t.Fatal(n)
	}
	for _, n := range []int{33, 34} {
		source := fmt.Sprintf(`(import (scheme base) (scheme eval) (scheme repl)) (eval '(resume %d) (interaction-environment))`, n)
		if got := number(runProgram(t, r, source)); got != int64(n+10) {
			t.Fatal(got)
		}
	}
}
func TestLoadSequentialEvaluationAndErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.scm")
	put := func(s string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r := core.NewRuntime()
	call := func(s string) scheme.Value {
		return runProgram(t, r, strings.ReplaceAll(s, "PATH", strconv.Quote(path)))
	}
	put(`(define answer 40) (define-syntax plus-two (syntax-rules () ((_ x) (+ x 2)))) (set! answer (plus-two answer))`)
	if got := call(`(import (scheme base) (scheme load) (scheme eval) (scheme repl)) (define input (current-input-port)) (define output (current-output-port)) (load PATH) (and (= (eval 'answer (interaction-environment)) 42) (eq? input (current-input-port)) (eq? output (current-output-port)))`); got != scheme.True {
		t.Fatal(got)
	}
	put(`(set! answer 50) (error "stop") (set! answer 99)`)
	if got := call(`(import (scheme base) (scheme load) (scheme eval) (scheme repl)) (guard (x (else #t)) (load PATH)) (= (eval 'answer (interaction-environment)) 50)`); got != scheme.True {
		t.Fatal(got)
	}
	put(`(set! answer 60) (`)
	if got := call(`(import (scheme base) (scheme load) (scheme eval) (scheme repl)) (and (guard (x (else (read-error? x))) (load PATH) #f) (= (eval 'answer (interaction-environment)) 60))`); got != scheme.True {
		t.Fatal(got)
	}
	put(`(define illegal 1)`)
	if got := call(`(import (scheme base) (scheme load) (scheme eval)) (guard (x (else (error-object? x))) (load PATH (environment '(scheme base))) #f)`); got != scheme.True {
		t.Fatal(got)
	}
	put(`(+ 20 22) (values 1 2)`)
	call(`(import (scheme base) (scheme load) (scheme eval)) (load PATH (environment '(scheme base)))`)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := call(`(import (scheme base) (scheme load)) (guard (x (else (file-error? x))) (load PATH) #f)`); got != scheme.True {
		t.Fatal(got)
	}
}

func TestREPLPersistentDefinitionsValuesAndRecovery(t *testing.T) {
	r := core.NewRuntime()
	var output, diagnostic bytes.Buffer
	input := strings.NewReader("(define answer 40)\n(set! answer (+ answer 2))\nanswer\n(values 1 2)\n(error \"retry\")\n(if)\n)\n(+ answer 1)\n")
	if err := r.REPL(context.Background(), "repl.scm", input, &output, &diagnostic); err != nil {
		t.Fatal(err)
	}
	if output.String() != "42\n1\n2\n43\n" {
		t.Fatal(output.String())
	}
	if !strings.Contains(diagnostic.String(), "retry") || !strings.Contains(diagnostic.String(), "SyntaxError") || strings.Count(diagnostic.String(), "ReaderError") != 1 {
		t.Fatal(diagnostic.String())
	}
	output.Reset()
	diagnostic.Reset()
	if err := r.REPL(context.Background(), "second.scm", strings.NewReader("answer\n"), &output, &diagnostic); err != nil || output.String() != "42\n" {
		t.Fatal(err, output.String())
	}
}
func TestREPLSharedCurrentPortsAndCancellation(t *testing.T) {
	r := core.NewRuntime()
	var out, diagnostic bytes.Buffer
	if err := r.REPL(context.Background(), "ports.scm", strings.NewReader("(write-string \"hello\")\n(read-char)Z\n"), &out, &diagnostic); err != nil {
		t.Fatal(err)
	}
	// read-char consumes the delimiter after the form, so the input deliberately
	// puts Z directly after the closing parenthesis.
	if out.String() != "hello#\\x5a\n" || diagnostic.Len() != 0 {
		t.Fatal(out.String(), diagnostic.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := r.REPL(ctx, "loop.scm", strings.NewReader("(let loop () (loop))"), &out, &diagnostic)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestEnvironmentLibraryInitializationAndSharing(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (state) (export counter next) (import (scheme base)) (begin (define counter 0) (define (next) (set! counter (+ counter 1)) counter)))`)
	if got := runProgram(t, r, `(import (scheme base) (scheme eval) (state)) (define e (environment '(scheme base) '(state))) (and (= (eval '(next) e) 1) (= counter 1) (= (next) 2) (= (eval 'counter e) 2) (guard (x (else #t)) (eval '(set! counter 99) e) #f))`); got != scheme.True {
		t.Fatal(got)
	}
}
func TestLoadContinuationRepeatsRemainingForms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "continuation.scm")
	if err := os.WriteFile(path, []byte(`(define k #f) (define value (call/cc (lambda (resume) (set! k resume) 1))) (set! count (+ count 1))`), 0600); err != nil {
		t.Fatal(err)
	}
	r := core.NewRuntime()
	setup := `(import (scheme base) (scheme load) (scheme eval) (scheme repl)) (eval '(define count 0) (interaction-environment)) (load PATH) (eval '(list value count) (interaction-environment))`
	got := runProgram(t, r, strings.ReplaceAll(setup, "PATH", strconv.Quote(path)))
	if n := number(got.(*scheme.Pair).Car()); n != 1 {
		t.Fatal(got)
	}
	for _, n := range []int{2, 3} {
		source := fmt.Sprintf(`(import (scheme base) (scheme eval) (scheme repl)) (eval '(k %d) (interaction-environment))`, n)
		got = runProgram(t, r, source)
		pair := got.(*scheme.Pair)
		if number(pair.Car()) != int64(n) || number(pair.Cdr().(*scheme.Pair).Car()) != int64(n) {
			t.Fatal(got)
		}
	}
}
func TestREPLImportsAndForwardDefinitions(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (numbers) (export value) (import (scheme base)) (begin (define value 42)))`)
	var output, diagnostic bytes.Buffer
	input := `(import (scheme base)) (define (f) later) (define later 42) (f) (define value 0) (import (numbers)) value (set! value 43) value`
	if err := r.REPL(context.Background(), "imports.scm", strings.NewReader(input), &output, &diagnostic); err != nil || diagnostic.Len() != 0 || output.String() != "42\n42\n43\n" {
		t.Fatal(err, output.String(), diagnostic.String())
	}
}

func TestEnvironmentErrorsAndLiteralGraphs(t *testing.T) {
	for _, source := range []string{
		`(import (scheme base) (scheme eval)) (define x (list 'only #f)) (set-car! (cdr x) x) (guard (e (else (error-object? e))) (environment x) #f)`,
		`(import (scheme base) (scheme eval)) (guard (e (else (error-object? e))) (eval 42 '()) #f)`,
		`(import (scheme base) (scheme eval)) (guard (e (else (error-object? e))) (environment '(only (scheme base) nonexistent)) #f)`,
		`(import (scheme base) (scheme eval)) (define x (list 42)) (set-cdr! x x) (define result (eval (list 'quote x) (environment '(scheme base)))) (and (eq? x result) (eq? result (cdr result)))`,
		`(import (scheme base) (scheme eval)) (= (eval '(let ((x 1)) (set! x 42) x) (environment '(scheme base))) 42)`,
		`(import (scheme base) (scheme eval)) (guard (e (else (error-object? e))) (eval '(define-values (x) 42) (environment '(scheme base))) #f)`,
	} {
		if got := runProgram(t, core.NewRuntime(), source); got != scheme.True {
			t.Fatal(source, got)
		}
	}
}

func TestREPLRenamedStandardImportsKeepLibraryCellsIsolated(t *testing.T) {
	r := core.NewRuntime()
	var output, diagnostic bytes.Buffer
	source := `(import (prefix (only (scheme base) +) b:))
 (set! b:+ (lambda args 99))
 (b:+ 1 2)
 (+ 1 2)
 (eval '(+ 20 22) (environment '(scheme base)))`
	if err := r.REPL(context.Background(), "aliases.scm", strings.NewReader(source), &output, &diagnostic); err != nil || diagnostic.Len() != 0 || output.String() != "99\n99\n42\n" {
		t.Fatal(err, output.String(), diagnostic.String())
	}
}

func TestEvalDynamicParameters(t *testing.T) {
	got := runProgram(t, core.NewRuntime(), `(import (scheme base) (scheme eval) (scheme repl))
 (define e (interaction-environment))
 (eval '(define setting (make-parameter 1)) e)
 (define setting (eval 'setting e))
 (and (= (parameterize ((setting 42)) (eval '(setting) e)) 42) (= (eval '(setting) e) 1))`)
	if got != scheme.True {
		t.Fatal(got)
	}
}
func TestLoadUsesWorkingDirectoryAndUTF8(t *testing.T) {
	cwd := t.TempDir()
	libraries := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("GURI_LOAD_PATH", libraries)
	if err := os.WriteFile(filepath.Join(cwd, "source.scm"), []byte(`(define text "日本語")`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libraries, "source.scm"), []byte(`(error "wrong directory")`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libraries, "missing.scm"), []byte(`42`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "bad.scm"), []byte{255}, 0600); err != nil {
		t.Fatal(err)
	}
	source := `(import (scheme base) (scheme load) (scheme eval) (scheme repl))
 (load "source.scm")
 (and (string=? (eval 'text (interaction-environment)) "日本語")
 (guard (e (else (file-error? e))) (load "missing.scm") #f)
 (guard (e (else (read-error? e))) (load "bad.scm") #f))`
	result, err := core.NewRuntime().Run(context.Background(), filepath.Join(t.TempDir(), "unrelated.scm"), source)
	if err != nil || result.Len() != 1 || result.Values()[0] != scheme.True {
		t.Fatal(result, err)
	}
}
func TestEvaluationWikiExamplesAndBindings(t *testing.T) {
	doc, err := os.ReadFile("../../docs/wiki/Evaluation-REPL.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```").FindAllStringSubmatch(string(doc), -1)
	expected := []string{`42`, `42`, `"(+ 1 2)"`, `42`, `#t`, `42`, `42`, `(99 42)`, `42`}
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
	selected := map[string]bool{"(scheme eval)": true, "(scheme load)": true, "(scheme repl)": true, "(scheme r5rs)": true}
	count := 0
	for _, row := range rows[1:] {
		if !selected[row[0]] {
			continue
		}
		count++
		if row[5] != "内部Runtimeのimport実装済み・公開CLI未接続" {
			t.Fatal(row)
		}
		if row[3] == "64" && !strings.Contains(string(doc), "("+row[1]+" ") && !strings.Contains(string(doc), "("+row[1]+")") {
			t.Fatal("missing reference", row[1])
		}
	}
	if count != 226 {
		t.Fatal(count)
	}
}

func TestEnvironmentRejectsRecursiveLibraryInitialization(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (recursive) (export value) (import (scheme base) (scheme eval)) (begin (define value (environment '(recursive)))))`)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := r.Run(ctx, "recursive.scm", `(import (recursive)) value`)
	if err == nil || errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "initializ") {
		t.Fatal("expected recursive initializer diagnostic", err)
	}
}
