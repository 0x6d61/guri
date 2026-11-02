package core_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/scheme"
)

// The test executable doubles as a portable child, without a shell or OS tools.
func TestSubprocessHelper(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--guri-process-helper" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	switch args[0] {
	case "echo":
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			os.Exit(91)
		}
		fmt.Fprint(os.Stderr, "diagnostic")
	case "args":
		fmt.Fprint(os.Stdout, strings.Join(args[1:], "\x00"))
	case "flood":
		fmt.Fprint(os.Stderr, strings.Repeat("e", 256*1024))
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			os.Exit(91)
		}
	case "environment":
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(92)
		}
		fmt.Fprint(os.Stdout, filepath.Clean(cwd), "\n", os.Getenv("GURI_PROCESS_TEST"), "\n", os.Getenv("GURI_PROCESS_INHERITED"))
	case "exit":
		n, _ := strconv.Atoi(args[1])
		os.Exit(n)
	case "hold":
		if err := os.WriteFile(args[1], []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(93)
		}
		for {
			time.Sleep(time.Second)
		}
	case "kill-self":
		p, err := os.FindProcess(os.Getpid())
		if err != nil {
			os.Exit(95)
		}
		if err := p.Kill(); err != nil {
			os.Exit(96)
		}
		for {
			time.Sleep(time.Second)
		}
	case "early": // Deliberately ignore all input.
	default:
		os.Exit(94)
	}
	os.Exit(0)
}

func childCommand(t *testing.T, args ...string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	parts := []string{strconv.Quote(exe), `"-test.run=^TestSubprocessHelper$"`, `"--"`, `"--guri-process-helper"`}
	for _, a := range args {
		parts = append(parts, strconv.Quote(a))
	}
	return "'(" + strings.Join(parts, " ") + ")"
}
func processRun(t *testing.T, source string) scheme.Value {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := core.NewRuntime().Run(ctx, "process.scm", `(import (scheme base) (guri process)) `+source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Len() != 1 {
		t.Fatal(result)
	}
	v, _ := result.Single()
	return v
}
func TestSubprocessCapture(t *testing.T) {
	cmd := childCommand(t, "echo")
	src := fmt.Sprintf(`(call-with-values (lambda () (run-process %s '((input . #u8(0 255 65)))))
  (lambda (status output diagnostic) (and (= status 0) (equal? output #u8(0 255 65)) (string=? (utf8->string diagnostic) "diagnostic"))))`, cmd)
	if got := processRun(t, src); got != scheme.True {
		t.Fatal(got)
	}
	// The default input is EOF and a nonzero exit remains an ordinary result.
	for _, tc := range []struct {
		mode  string
		args  []string
		check string
	}{
		{"echo", nil, `(and (= status 0) (= (bytevector-length output) 0))`},
		{"exit", []string{"7"}, `(and (= status 7) (= (bytevector-length output) 0) (= (bytevector-length diagnostic) 0))`},
		{"args", []string{"a b", `"quoted"`, "$HOME; | *", ""}, `(equal? output (string->utf8 (string-append "a b" (string #\null) "\"quoted\"" (string #\null) "$HOME; | *" (string #\null))))`},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			if got := processRun(t, `(call-with-values (lambda () (run-process `+childCommand(t, append([]string{tc.mode}, tc.args...)...)+`)) (lambda (status output diagnostic) `+tc.check+`))`); got != scheme.True {
				t.Fatal(got)
			}
		})
	}
}
func TestSubprocessPipeline(t *testing.T) {
	cmd := childCommand(t, "flood")
	input := strings.Repeat("i", 512*1024)
	src := fmt.Sprintf(`(call-with-values (lambda () (run-pipeline (list %s %s %s) (list (cons 'input (string->utf8 %s)))))
  (lambda (statuses output diagnostics) (and (equal? statuses '(0 0 0)) (string=? (utf8->string output) %s)
   (= (length diagnostics) 3) (apply = (map bytevector-length diagnostics)) (= (bytevector-length (car diagnostics)) 262144))))`, cmd, cmd, cmd, strconv.Quote(input), strconv.Quote(input))
	if got := processRun(t, src); got != scheme.True {
		t.Fatal(got)
	}
	if got := processRun(t, `(call-with-values (lambda () (run-pipeline (list `+childCommand(t, "exit", "3")+` `+childCommand(t, "echo")+`))) (lambda (s o e) (equal? s '(3 0))))`); got != scheme.True {
		t.Fatal(got)
	}
	if got := processRun(t, `(call-with-values (lambda () (run-pipeline (list `+childCommand(t, "echo")+`))) (lambda (s o e) (equal? s '(0))))`); got != scheme.True {
		t.Fatal(got)
	}
}
func TestSubprocessEnvironment(t *testing.T) {
	t.Setenv("GURI_PROCESS_INHERITED", "inherited")
	t.Setenv("GURI_PROCESS_TEST", "parent")
	dir := t.TempDir()
	// Compare directory identity: Windows path casing and macOS symlinks can
	// give getcwd a different spelling than the supplied directory.
	src := fmt.Sprintf(`(call-with-values (lambda () (run-process %s '((directory . %s) (environment . (("GURI_PROCESS_TEST" . "child"))))))
  (lambda (s o e) (vector s o e)))`, childCommand(t, "environment"), strconv.Quote(dir))
	values := processRun(t, src).(*scheme.Vector).Values()
	output := string(values[1].(*scheme.Bytevector).Bytes())
	if values[0].(*scheme.Integer).BigInt().Sign() != 0 || values[2].(*scheme.Bytevector).Len() != 0 {
		t.Fatal(values[0], output, values[2])
	}
	fields := strings.Split(output, "\n")
	if len(fields) != 3 || fields[1] != "child" || fields[2] != "inherited" {
		t.Fatalf("child environment: %q", output)
	}
	actual, err := os.Stat(fields[0])
	if err != nil {
		t.Fatal(output, err)
	}
	expected, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(actual, expected) {
		t.Fatalf("child directory %q does not identify %q", fields[0], dir)
	}

	if os.Getenv("GURI_PROCESS_TEST") != "parent" {
		t.Fatal("parent environment changed")
	}
}
func TestSubprocessValidation(t *testing.T) {
	for _, call := range []string{
		`(run-process)`, `(run-process '("x") '() '())`, `(run-process 1)`, `(run-process '())`, `(run-process '(""))`, `(run-process '("x" . "y"))`, `(run-process '("x" 1))`,
		`(run-process (list (string #\null)))`, `(run-process '("x") 1)`, `(run-process '("x") '(input))`, `(run-process '("x") '(("input" . #u8())))`, `(run-process '("x") (list (cons 'directory (string #\null))))`, `(run-process '("x") '((unknown . 1)))`,
		`(run-process '("x") '((input . "text")))`, `(run-process '("x") '((input . #u8()) (input . #u8())))`, `(run-process '("x") '((directory . 1)))`,
		`(run-process '("x") '((environment . (("" . "value")))))`, `(run-process '("x") '((environment . (("x=y" . "value")))))`,
		`(run-process '("x") '((environment . (("x" . 1)))))`, `(run-process '("x") '((environment . (("x" . "1") ("x" . "2")))))`,
		`(run-process '("x") (list (cons 'environment (list (cons "x" (string #\null))))))`,
		`(let ((x (list (cons "x" "value")))) (set-cdr! x x) (run-process '("x") (list (cons 'environment x))))`, `(run-process '("x") '((environment . ("x"))))`,
		`(run-pipeline)`, `(run-pipeline '())`, `(run-pipeline '(()))`, `(run-pipeline '("x"))`, `(run-pipeline '(("x") . 1))`,
		`(let ((x (list "x"))) (set-cdr! x x) (run-process x))`,
		`(let ((x (list (cons 'input #u8())))) (set-cdr! x x) (run-process '("x") x))`,
		`(run-process '("guri-missing-executable-02b2f37b"))`,
	} {
		t.Run(call, func(t *testing.T) {
			if got := processRun(t, `(guard (e (else (error-object? e))) `+call+` #f)`); got != scheme.True {
				t.Fatal(got)
			}
		})
	}
}
func TestSubprocessLibraryImports(t *testing.T) {
	if got := processRun(t, `(and (procedure? run-process) (procedure? run-pipeline) (cond-expand ((library (guri process)) #t) (else #f)))`); got != scheme.True {
		t.Fatal(got)
	}
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (child-client) (export run) (import (scheme base) (guri process)) (begin (define run run-process)))`)
	if got := runProgram(t, r, `(import (scheme base) (child-client)) (call-with-values (lambda () (run `+childCommand(t, "exit", "0")+`)) (lambda (s o e) (= s 0)))`); got != scheme.True {
		t.Fatal(got)
	}
	if got := runProgram(t, core.NewRuntime(), `(import (scheme base) (scheme eval)) (procedure? (eval 'run-process (environment '(guri process))))`); got != scheme.True {
		t.Fatal(got)
	}
}
func TestSubprocessCancellation(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pid")
	second := marker + "-second"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := core.NewRuntime().Run(ctx, "cancel.scm", `(import (scheme base) (guri process)) (run-pipeline (list `+childCommand(t, "hold", marker)+` `+childCommand(t, "hold", second)+`))`)
		done <- err
	}()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if _, err := os.Stat(marker); err == nil {
			if _, err := os.Stat(second); err == nil {
				break
			}
		}
		select {
		case <-tick.C:
		case err := <-done:
			t.Fatal("returned before cancellation", err)
		case <-deadline.C:
			t.Fatal("child did not start")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled children were not reaped")
	}
	assertProcessReaped(t, marker)
	assertProcessReaped(t, second)
}
func TestSubprocessStartFailureAndBorrowedPorts(t *testing.T) {
	r := core.NewRuntime()
	input := ports.InputString("borrowed")
	output, diagnostic := ports.MemoryOutput(false), ports.MemoryOutput(false)
	if err := r.SetStandardPorts(input, output, diagnostic); err != nil {
		t.Fatal(err)
	}
	source := `(import (scheme base) (scheme read) (scheme write) (guri process))
 (guard (e (else #t)) (run-pipeline (list ` + childCommand(t, "echo") + ` '("guri-missing-executable-02b2f37b"))))
 (display "still-open") (and (input-port-open? (current-input-port)) (output-port-open? (current-output-port)) (eqv? (read-char) #\b))`
	if got := runProgram(t, r, source); got != scheme.True {
		t.Fatal(got)
	}
	out, err := output.OutputBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	diag, err := diagnostic.OutputBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "still-open" || len(diag) != 0 {
		t.Fatal(string(out), string(diag))
	}
}

func TestSubprocessEarlyInputClose(t *testing.T) {
	src := `(call-with-values (lambda () (run-process ` + childCommand(t, "early") + ` (list (cons 'input (make-bytevector 1048576 65))))) (lambda (s o e) (= s 0)))`
	if got := processRun(t, src); got != scheme.True {
		t.Fatal(got)
	}
	// A downstream process closing its pipe does not erase any stage's status.
	src = `(call-with-values (lambda () (run-pipeline (list ` + childCommand(t, "flood") + ` ` + childCommand(t, "early") + `) (list (cons 'input (make-bytevector 1048576 65))))) (lambda (s o e) (and (= (length s) 2) (= (cadr s) 0))))`
	if got := processRun(t, src); got != scheme.True {
		t.Fatal(got)
	}
}

func TestSubprocessInvalidLaterStageHasNoSideEffects(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pid")
	src := `(guard (e (else (error-object? e))) (run-pipeline (list ` + childCommand(t, "hold", marker) + ` '("x" 1))) #f)`
	if got := processRun(t, src); got != scheme.True {
		t.Fatal(got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("child started before argument validation", err)
	}
}

func TestSubprocessDeadline(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := core.NewRuntime().Run(ctx, "deadline.scm", `(import (scheme base) (guri process)) (run-process `+childCommand(t, "hold", marker)+`)`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		assertProcessReaped(t, marker)
	}
}

func TestSubprocessResultsAreFresh(t *testing.T) {
	cmd := childCommand(t, "echo")
	source := `(define input (bytevector 65 66))
 (define options (list (cons 'input input)))
 (define first (call-with-values (lambda () (run-process ` + cmd + ` options)) list))
 (define second (call-with-values (lambda () (run-process ` + cmd + ` options)) list))
 (bytevector-u8-set! (cadr first) 0 90)
 (and (equal? input #u8(65 66)) (equal? (cadr second) #u8(65 66)) (eq? (cdar options) input))`
	if got := processRun(t, source); got != scheme.True {
		t.Fatal(got)
	}
}

func TestSubprocessMissingDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "missing")
	source := `(guard (e (else (error-object? e))) (run-process ` + childCommand(t, "echo") + ` (list (cons 'directory ` + strconv.Quote(directory) + `))) #f)`
	if got := processRun(t, source); got != scheme.True {
		t.Fatal(got)
	}
}
