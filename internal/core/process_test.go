package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestExitStatusMapping(t *testing.T) {
	for _, name := range []string{"exit", "emergency-exit"} {
		for _, test := range []struct {
			argument string
			code     int
		}{{"", 0}, {"#t", 0}, {"#f", 1}, {"7", 7}, {"256", 0}, {"-1", 255}, {"123456789012345678901234567890", 210}, {"\"failure\"", 1}, {"'()", 1}, {"0.0", 1}} {
			e := core.NewCore()
			_, err := e.Eval(context.Background(), "exit.scm", "("+name+" "+test.argument+")")
			var exit *core.ExitRequest
			if !errors.As(err, &exit) || exit.Code != test.code || exit.Emergency != (name == "emergency-exit") {
				t.Fatal(name, test, err)
			}
			var raised *scheme.Raised
			if errors.As(err, &raised) {
				t.Fatal("exit was raised as Scheme exception", err)
			}
		}
	}
}
func TestExitUnwindsAndEmergencySkipsAfter(t *testing.T) {
	for _, name := range []string{"exit", "emergency-exit"} {
		e := core.NewCore()
		source := `(define log '())
 (define (note x) (set! log (cons x log)))
 (guard (e (else (note 'caught)))
 (dynamic-wind (lambda () (note 'outer-before))
  (lambda () (dynamic-wind (lambda () (note 'inner-before))
   (lambda () (` + name + ` 7) (note 'returned))
   (lambda () (note 'inner-after))))
  (lambda () (note 'outer-after))))
 (note 'continued)`
		_, err := e.Eval(context.Background(), "wind-exit.scm", source)
		var exit *core.ExitRequest
		if !errors.As(err, &exit) || exit.Code != 7 {
			t.Fatal(err)
		}
		want := `'(outer-after inner-after inner-before outer-before)`
		if name == "emergency-exit" {
			want = `'(inner-before outer-before)`
		}
		if got := eval(t, e, `(equal? log `+want+`)`); got != scheme.True {
			t.Fatal(name, got)
		}
	}
}
func TestExitFromAfterOverridesEarlierRequest(t *testing.T) {
	e := core.NewCore()
	_, err := e.Eval(context.Background(), "nested-exit.scm", `(dynamic-wind (lambda () #t) (lambda () (exit 1)) (lambda () (exit 7)))`)
	var exit *core.ExitRequest
	if !errors.As(err, &exit) || exit.Code != 7 {
		t.Fatal(err)
	}
}

func TestExitCleanupContinuationKeepsTerminalOutcome(t *testing.T) {
	e := core.NewCore()
	source := `(define resume #f) (define count 0)
 (dynamic-wind (lambda () #t) (lambda () (exit 7))
  (lambda () (call/cc (lambda (k) (set! resume k))) (set! count (+ count 1)) (values 1 2)))`
	_, err := e.Eval(context.Background(), "cleanup.scm", source)
	var exit *core.ExitRequest
	if !errors.As(err, &exit) || exit.Code != 7 {
		t.Fatal(err)
	}
	for _, expected := range []int64{2, 3} {
		_, err = e.Eval(context.Background(), "resume.scm", `(resume)`)
		if !errors.As(err, &exit) || exit.Code != 7 {
			t.Fatal(err)
		}
		if n := number(eval(t, e, `count`)); n != expected {
			t.Fatal(n)
		}
	}
}
