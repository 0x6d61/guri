package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func runProgram(t *testing.T, r *core.Runtime, source string) scheme.Value {
	t.Helper()
	values, err := r.Run(context.Background(), "main.scm", source)
	if err != nil {
		t.Fatal(err)
	}
	value, err := values.Single()
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func registerLibrary(t *testing.T, r *core.Runtime, source string) {
	t.Helper()
	if err := r.Register("library.sld", source); err != nil {
		t.Fatal(err)
	}
}
func TestProgramImportsAndEmptyEnvironment(t *testing.T) {
	r := core.NewRuntime()
	for _, src := range []string{
		`(import (scheme base)) (= (+ 20 22) 42)`,
		`(import (only (scheme base) + =)) (= (+ 20 22) 42)`,
		`(import (prefix (only (scheme base) + =) p:)) (p:= (p:+ 20 22) 42)`,
		`(import (rename (only (scheme base) + =) (+ add))) (= (add 20 22) 42)`,
		`(import (scheme base) (scheme char) (scheme cxr)) (and (char=? #\a (cadr '(1 #\a))) (procedure? +))`,
		`(import (scheme base)) (define (f) x) (define x 42) (= (f) 42)`,
	} {
		if got := runProgram(t, r, src); got != scheme.True {
			t.Fatal(src, got)
		}
	}
	for _, src := range []string{
		`(+ 1 2)`, `(import (only (scheme base) +)) (if #t 1 2)`,
		`(import (scheme base)) (display "hidden")`, `(import (scheme base)) unknown`,
		`(import (scheme base)) (set! + 1)`, `(import (scheme base)) (define + 1)`,
		`(import (scheme base)) (define-syntax if (syntax-rules () ((_ x ...) 1)))`,
		`(import (scheme base)) 1 (import (scheme write))`,
	} {
		if _, err := r.Run(context.Background(), "bad.scm", src); err == nil {
			t.Error(src)
		}
	}
	if got := number(runProgram(t, r, `(import) 42`)); got != 42 {
		t.Fatal(got)
	}
}
func TestLibraryIsolationSharedBindingsAndInitialization(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (counter) (export count next (rename count current)) (import (scheme base)) (begin (define count 0) (define (next) (set! count (+ count 1)) count)))`)
	if got := number(runProgram(t, r, `(import (counter)) (next)`)); got != 1 {
		t.Fatal(got)
	}
	if got := runProgram(t, r, `(import (scheme base) (only (counter) count) (only (counter) current next)) (next) (and (= count 2) (= current 2))`); got != scheme.True {
		t.Fatal(got)
	}
	registerLibrary(t, r, `(define-library (facade) (export (rename next step)) (import (counter)))`)
	if got := number(runProgram(t, r, `(import (facade)) (step)`)); got != 3 {
		t.Fatal(got)
	}
	for _, src := range []string{`(import (counter) (scheme base)) (set! count 9)`, `(import (counter) (scheme base)) (define next 9)`, `(import (counter)) +`} {
		if _, err := r.Run(context.Background(), "private.scm", src); err == nil {
			t.Error(src)
		}
	}
	registerLibrary(t, r, `(define-library (other) (export count) (import (scheme base)) (begin (define count 99)))`)
	if _, err := r.Run(context.Background(), "collision.scm", `(import (counter) (other)) count`); err == nil {
		t.Fatal("import conflict")
	}
}
func TestImportedMacrosAndRenamedLiteralIdentity(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (macros) (export result token bump count) (import (scheme base)) (begin (define count 0) (define token 1) (define private 42) (define-syntax result (syntax-rules (token) ((_ token) private) ((_ x) 0))) (define-syntax bump (syntax-rules () ((_) (set! count (+ count 1)))))))`)
	if got := runProgram(t, r, `(import (scheme base) (rename (macros) (result pick) (token renamed))) (define private 99) (bump) (and (= (pick renamed) 42) (= count 1))`); got != scheme.True {
		t.Fatal(got)
	}
	if got := number(runProgram(t, r, `(import (scheme base) (rename (macros) (result pick))) (let ((token 9)) (pick token))`)); got != 0 {
		t.Fatal(got)
	}
}
func TestLibraryInitializationFailureAndCycles(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (bad) (export x) (import (scheme base)) (begin (define x (error "initialization failed"))))`)
	for range 2 {
		_, err := r.Run(context.Background(), "bad.scm", `(import (bad)) x`)
		var raised *scheme.Raised
		if !errors.As(err, &raised) {
			t.Fatal(err)
		}
	}
	registerLibrary(t, r, `(define-library (a) (import (b))) (define-library (b) (import (a)))`)
	if _, err := r.Run(context.Background(), "cycle.scm", `(import (a)) 1`); err == nil {
		t.Fatal("dependency cycle")
	}
	registerLibrary(t, r, `(define-library (undefined) (export missing))`)
	if _, err := r.Run(context.Background(), "export.scm", `(import (undefined)) missing`); err == nil {
		t.Fatal("undefined export")
	}
}

func TestLibraryInitializerContinuationUsesProgramVM(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (resume-init) (export resume marker) (import (scheme base)) (begin (define resume #f) (define marker (call/cc (lambda (k) (set! resume k) 1)))))`)
	if n := number(runProgram(t, r, `(import (resume-init)) marker`)); n != 1 {
		t.Fatal(n)
	}
	if n := number(runProgram(t, r, `(import (resume-init)) (resume 42)`)); n != 42 {
		t.Fatal("initializer continuation is not on the program VM", n)
	}
	if n := number(runProgram(t, r, `(import (resume-init)) marker`)); n != 42 {
		t.Fatal(n)
	}
}
func TestFailedInitializerIsNotAutomaticallyRetried(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (meter) (export count tick) (import (scheme base)) (begin (define count 0) (define (tick) (set! count (+ count 1)))))`)
	registerLibrary(t, r, `(define-library (broken) (import (scheme base) (meter)) (begin (tick) (error "failed")))`)
	for range 2 {
		if _, err := r.Run(context.Background(), "broken.scm", `(import (broken)) 1`); err == nil {
			t.Fatal("expected initialization error")
		}
	}
	if n := number(runProgram(t, r, `(import (meter)) count`)); n != 1 {
		t.Fatal("failed initializer retried", n)
	}
}
func TestLibrarySearchIncludesAndFeatures(t *testing.T) {
	root := t.TempDir()
	write := func(path, source string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("myapp/util.sld", `(define-library (myapp util) (include-library-declarations "decls.scm") (cond-expand ((library (scheme char)) (include-ci "body.scm")) (else (begin (define result 0)))))`)
	write("myapp/decls.scm", `(export result) (import (scheme base))`)
	write("myapp/body.scm", `(DEFINE RESULT 42)`)
	r := core.NewRuntime(root)
	if n := number(runProgram(t, r, `(import (myapp util)) result`)); n != 42 {
		t.Fatal(n)
	}
	if got := runProgram(t, r, `(import (scheme base)) (and (memq 'guri (features)) (not (memq 'r7rs (features))) (cond-expand ((and (library (myapp util)) (not (library (scheme process-context)))) #t) (else #f)))`); got != scheme.True {
		t.Fatal(got)
	}
	// Program include uses the including source file, not the library search root.
	write("program/parts.scm", `(define included 9)`)
	got, err := r.Run(context.Background(), filepath.Join(root, "program/main.scm"), `(import (scheme base)) (include "parts.scm") included`)
	if err != nil || number(got.Values()[0]) != 9 {
		t.Fatal(got, err)
	}
	write("cycle.sld", `(define-library (cycle) (include-library-declarations "cycle-decls.scm"))`)
	write("cycle-decls.scm", `(include-library-declarations "cycle-decls.scm")`)
	if _, err := r.Run(context.Background(), "cycle.scm", `(import (cycle)) 1`); err == nil {
		t.Fatal("include cycle")
	}
}
func TestLibrarySearchDoesNotAssumeWorkingDirectoryLayout(t *testing.T) {
	t.Setenv("GURI_LOAD_PATH", "")
	root := t.TempDir()
	t.Chdir(root)
	for _, dir := range []string{"lib", "examples", "other"} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	put := func(dir, name string, n int) {
		t.Helper()
		src := fmt.Sprintf(`(define-library (%s) (export n) (import (scheme base)) (begin (define n %d)))`, name, n)
		if err := os.WriteFile(filepath.Join(dir, name+".sld"), []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("lib", "test", 42)
	put("other", "test", 99)
	put("examples", "legacy", 7)
	if _, err := core.NewRuntime().Run(context.Background(), "main.scm", `(import (test)) n`); err == nil {
		t.Fatal("implicitly searched cwd/lib")
	}
	if n := number(runProgram(t, core.NewRuntime("lib"), `(import (test)) n`)); n != 42 {
		t.Fatal(n)
	}
	if n := number(runProgram(t, core.NewRuntime("other", "lib"), `(import (test)) n`)); n != 99 {
		t.Fatal(n)
	}
	if _, err := core.NewRuntime().Run(context.Background(), "bad.scm", `(import (legacy)) n`); err == nil {
		t.Fatal("searched legacy examples")
	}
}
func TestLibraryDeclarationErrorsAndIsolation(t *testing.T) {
	for _, decl := range []string{
		`(define-library (bad) (unknown))`,
		`(define-library (bad) (export (rename x)))`,
		`(define-library (bad) (include 1))`,
		`(define-library (bad) (include-library-declarations 1))`,
		`(define-library (bad) (import (scheme base)) (begin (define-syntax + (syntax-rules () ((_ x ...) 1)))))`,
		`(define-library (bad) (import (scheme base)) (begin (define-record-type T (+ x) p (x get))))`,
	} {
		r := core.NewRuntime()
		registerLibrary(t, r, decl)
		if _, err := r.Run(context.Background(), "bad.scm", `(import (bad)) 1`); err == nil {
			t.Error(decl)
		}
	}
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (shared) (export item) (import (scheme base)) (begin (define item (vector 1))))`)
	if got := runProgram(t, r, `(import (scheme base) (shared)) (vector-set! item 0 42) (= (vector-ref item 0) 42)`); got != scheme.True {
		t.Fatal(got)
	}
	if _, err := r.Run(context.Background(), "scope.scm", `(import (shared)) (vector-ref item 0)`); err == nil {
		t.Fatal("leaked core procedure")
	}
	if got := number(runProgram(t, r, `(import (scheme base) (shared)) (vector-ref item 0)`)); got != 42 {
		t.Fatal(got)
	}
}

func TestLibrarySupportProcedures(t *testing.T) {
	r := core.NewRuntime()
	for _, src := range []string{
		`(import (scheme base)) (and (procedure? +) (procedure? (lambda () 1)) (procedure? (make-parameter 1)) (not (procedure? #f)) (call/cc (lambda (k) (procedure? k))))`,
		`(import (scheme base)) (and (memq 'guri (features)) (memq 'full-unicode (features)) (not (memq 'r7rs (features))) #t)`,
	} {
		if got := runProgram(t, r, src); got != scheme.True {
			t.Fatal(src, got)
		}
	}
	root := t.TempDir()
	path := filepath.Join(root, "delete-me")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	src := fmt.Sprintf(`(import (scheme base) (scheme file)) (define path %q) (define existed (file-exists? path)) (delete-file path) (and existed (not (file-exists? path)) (guard (e ((file-error? e) #t) (else #f)) (delete-file path)))`, path)
	if got := runProgram(t, r, src); got != scheme.True {
		t.Fatal(got)
	}
	for _, source := range []string{`(import (scheme base) (scheme file)) (guard (e (else (error-object? e))) (file-exists? 1))`, `(import (scheme base) (scheme file)) (guard (e (else (error-object? e))) (delete-file 1))`} {
		if got := runProgram(t, r, source); got != scheme.True {
			t.Fatal(got)
		}
	}
}

func TestLibraryInitializerEscapeAndExplicitResumption(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (hub) (export save-exit jump save-k resume) (import (scheme base)) (begin (define exit #f) (define saved #f) (define (save-exit k) (set! exit k)) (define (jump) (exit 7)) (define (save-k k) (set! saved k)) (define (resume x) (saved x))))`)
	if n := number(runProgram(t, r, `(import (scheme base) (hub)) (call/cc (lambda (k) (save-exit k) 1))`)); n != 1 {
		t.Fatal(n)
	}
	registerLibrary(t, r, `(define-library (suspended) (export value) (import (scheme base) (hub)) (begin (define value (call/cc (lambda (k) (save-k k) (jump))))))`)
	if n := number(runProgram(t, r, `(import (suspended)) value`)); n != 7 {
		t.Fatal(n)
	}
	if _, err := r.Run(context.Background(), "again.scm", `(import (suspended)) value`); err == nil {
		t.Fatal("automatically retried escaped initializer")
	}
	if n := number(runProgram(t, r, `(import (hub)) (resume 42)`)); n != 42 {
		t.Fatal(n)
	}
	if n := number(runProgram(t, r, `(import (suspended)) value`)); n != 42 {
		t.Fatal(n)
	}
}

func TestLibraryWikiExamples(t *testing.T) {
	doc, err := os.ReadFile("../../docs/wiki/Libraries.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```").FindAllStringSubmatch(string(doc), -1)
	expected := []string{"1", "#t", "#t", "42", "#t", "42", "#t"}
	if len(blocks) != len(expected) {
		t.Fatal("example count", len(blocks))
	}
	for i, block := range blocks {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Chdir(t.TempDir())
			r := core.NewRuntime()
			source := block[1]
			if i == 0 {
				registerLibrary(t, r, source)
				source = `(import (myapp counter)) (step)`
			}
			got := runProgram(t, r, source)
			nodes, err := reader.ReadAll("expected.scm", expected[i], nil)
			if err != nil || len(nodes) != 1 {
				t.Fatal(err)
			}
			if !equality.Equal(got, nodes[0].Datum()) {
				t.Fatal(i, got, expected[i])
			}
		})
	}
}

func TestLibraryRequirementDoesNotInitializeBody(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (probe-meter) (export count tick) (import (scheme base)) (begin (define count 0) (define (tick) (set! count (+ count 1)))))`)
	registerLibrary(t, r, `(define-library (probe-target) (import (probe-meter)) (begin (tick)))`)
	if got := runProgram(t, r, `(import (scheme base) (probe-meter)) (cond-expand ((library (probe-target)) (= count 0)) (else #f))`); got != scheme.True {
		t.Fatal("library requirement ran an initializer", got)
	}
	if got := runProgram(t, r, `(import (scheme base) (probe-meter) (probe-target)) (= count 1)`); got != scheme.True {
		t.Fatal(got)
	}
}

func TestExplicitLibraryPathsAreIndependentOfScriptLocation(t *testing.T) {
	// Three unrelated directories: working directory, script, and library root.
	t.Chdir(t.TempDir())
	scriptDir := t.TempDir()
	libraryDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(libraryDir, "myapp"), 0755); err != nil {
		t.Fatal(err)
	}
	source := `(define-library (myapp util) (export answer) (import (scheme base)) (begin (define answer 42)))`
	if err := os.WriteFile(filepath.Join(libraryDir, "myapp/util.sld"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r := core.NewRuntime(libraryDir)
	result, err := r.Run(context.Background(), filepath.Join(scriptDir, "arbitrary-name.scm"), `(import (myapp util)) answer`)
	if err != nil || result.Len() != 1 || number(result.Values()[0]) != 42 {
		t.Fatal(result, err)
	}
}

func TestLibraryEnvironmentSearchPaths(t *testing.T) {
	environmentRoot := t.TempDir()
	explicitRoot := t.TempDir()
	for root, value := range map[string]int{environmentRoot: 42, explicitRoot: 99} {
		source := fmt.Sprintf(`(define-library (env-example) (export value) (import (scheme base)) (begin (define value %d)))`, value)
		if err := os.WriteFile(filepath.Join(root, "env-example.sld"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GURI_LOAD_PATH", environmentRoot)
	if n := number(runProgram(t, core.NewRuntime(), `(import (env-example)) value`)); n != 42 {
		t.Fatal(n)
	}
	if n := number(runProgram(t, core.NewRuntime(explicitRoot), `(import (env-example)) value`)); n != 99 {
		t.Fatal(n)
	}
}
