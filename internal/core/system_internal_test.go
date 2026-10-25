package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0x6d61/guri/internal/scheme"
)

func checkSystem(t *testing.T, r *Runtime, source string) {
	t.Helper()
	result, err := r.Run(context.Background(), "system.scm", `(import (scheme base) (scheme process-context) (scheme time)) `+source)
	if err != nil {
		t.Fatal(err)
	}
	v, err := result.Single()
	if err != nil || v != scheme.True {
		t.Fatal(result, err)
	}
}
func TestSystemCommandLineSnapshot(t *testing.T) {
	r := NewRuntime()
	args := []string{"guri-script", "日本語", "", "--flag"}
	if err := r.SetCommandLine(args...); err != nil {
		t.Fatal(err)
	}
	args[0] = "changed"
	checkSystem(t, r, `(define a (command-line)) (set-car! a "modified") (and (equal? (command-line) '("guri-script" "日本語" "" "--flag")) (guard (e (else #t)) (string-set! (cadr (command-line)) 0 #\x) #f))`)
	if err := r.SetCommandLine(string([]byte{255})); !errors.Is(err, scheme.ErrEncoding) {
		t.Fatal(err)
	}
	checkSystem(t, r, `(string=? (car (command-line)) "guri-script")`)
	if err := r.SetCommandLine(); err == nil {
		t.Fatal("missing command name accepted")
	}
}
func TestSystemEnvironmentEncodingAndFreshValues(t *testing.T) {
	r := NewRuntime()
	text := "日本語"
	r.system.lookup = func(name string) (string, bool) {
		switch name {
		case "found":
			return text, true
		case "empty":
			return "", true
		case "invalid":
			return string([]byte{255}), true
		}
		return "", false
	}
	entries := []string{"found=日本語", "empty=", "equals=a=b", "=C:=C:\\work"}
	r.system.environ = func() []string { return entries }
	checkSystem(t, r, `(and (string=? (get-environment-variable "found") "日本語") (eq? (get-environment-variable "missing") #f) (string=? (get-environment-variable "empty") "") (string=? (cdr (assoc "equals" (get-environment-variables))) "a=b") (string=? (cdr (assoc "=C:" (get-environment-variables))) "C:\\work") (guard (e (else (error-object? e))) (get-environment-variable "invalid") #f) (guard (e (else #t)) (set-car! (get-environment-variables) 'changed) #f))`)
	text = "next"
	checkSystem(t, r, `(string=? (get-environment-variable "found") "next")`)
	for _, entry := range []string{string([]byte{'x', '=', 255}), string([]byte{255, '=', 'x'}), "malformed"} {
		entries = []string{entry}
		checkSystem(t, r, `(guard (e (else (error-object? e))) (get-environment-variables) #f)`)
	}
}
func TestSystemTAIAndMonotonicJiffies(t *testing.T) {
	r := NewRuntime()
	wall := time.Date(2026, 10, 25, 0, 0, 0, 250_000_000, time.UTC)
	elapsed := 3*time.Second + 123*time.Nanosecond
	epoch := r.system.epoch
	r.system.now = func() time.Time { return wall }
	r.system.since = func(start time.Time) time.Duration {
		if start != epoch {
			t.Fatal("jiffy epoch changed")
		}
		return elapsed
	}
	checkSystem(t, r, `(and (= (current-second) 1792886437.25) (inexact? (current-second)) (= (current-jiffy) 3000000123) (= (jiffies-per-second) 1000000000) (exact? (current-jiffy)))`)
	// Changing the wall clock has no effect on the monotonic measurement.
	wall = wall.Add(-time.Hour)
	elapsed += time.Second
	checkSystem(t, r, `(and (= (current-second) 1792882837.25) (= (current-jiffy) 4000000123))`)
}
