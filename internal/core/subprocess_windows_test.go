package core_test

import (
	"github.com/0x6d61/guri/internal/scheme"
	"os"
	"strconv"
	"testing"
)

func assertProcessReaped(t *testing.T, marker string) {
	t.Helper()
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	} // The process no longer has a handle to open.
	defer p.Release()
	state, err := p.Wait()
	if err != nil || !state.Exited() {
		t.Fatalf("child %d is not terminated: %v %v", pid, state, err)
	}
}

func TestSubprocessWindowsUnsignedExit(t *testing.T) {
	if got := processRun(t, `(call-with-values (lambda () (run-process `+childCommand(t, "exit", "4294967295")+`)) (lambda (s o e) (= s 4294967295)))`); got != scheme.True {
		t.Fatal(got)
	}
}

func TestSubprocessWindowsEnvironmentNames(t *testing.T) {
	if got := processRun(t, `(guard (e (else (error-object? e))) (run-process '("x") '((environment . (("Name" . "1") ("NAME" . "2"))))) #f)`); got != scheme.True {
		t.Fatal(got)
	}
	t.Setenv("guri_process_test", "parent")
	command := childCommand(t, "environment")
	source := `(call-with-values (lambda () (run-process ` + command + ` '((environment . (("GURI_PROCESS_TEST" . "child")))))) (lambda (s o e) (and (= s 0) (let ((text (utf8->string o))) (let loop ((i 0)) (cond ((> (+ i 7) (string-length text)) #f) ((string=? (substring text i (+ i 7)) "\nchild\n") #t) (else (loop (+ i 1)))))))))`
	if got := processRun(t, source); got != scheme.True {
		t.Fatal(got)
	}
}
