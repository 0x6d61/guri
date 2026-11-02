//go:build !windows

package core_test

import (
	"errors"
	"os"
	"strconv"
	"syscall"
	"testing"

	"github.com/0x6d61/guri/internal/scheme"
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
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("child %d still exists: %v", pid, err)
	}
}

func TestSubprocessSignalExit(t *testing.T) {
	// The Go helper kills itself, without invoking an OS utility or shell.
	if got := processRun(t, `(call-with-values (lambda () (run-process `+childCommand(t, "kill-self")+`)) (lambda (s o e) (eq? s #f)))`); got != scheme.True {
		t.Fatal(got)
	}
}
