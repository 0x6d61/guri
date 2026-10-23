package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestStandardControlProcedures(t *testing.T) {
	for s, want := range map[string]string{
		`(call/cc (lambda (exit) (+ 1 (exit 42))))`:                        `42`,
		`(+ 1 (call-with-current-continuation (lambda (exit) (exit 42))))`: `43`,
		`(apply + '(20 22))`: `42`, `(apply + 20 '(22))`: `42`,
		`(call-with-values (lambda () (values 20 22)) +)`:                   `42`,
		`(call-with-values (lambda () (values)) (lambda () 42))`:            `42`,
		`(call-with-values (lambda () (call/cc (lambda (k) (k 20 22)))) +)`: `42`,
		`(begin (values 1 2 3) 42)`:                                         `42`,
		`(apply (lambda args args) '())`:                                    `()`,
	} {
		t.Run(s, func(t *testing.T) {
			if got := repr(eval(t, engine(), s)); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
	for _, s := range []string{`(apply + #(1 2))`, `(apply + '(1 . 2))`, `(apply + 1)`, `(call/cc 1)`, `(call-with-values 1 (lambda () 0))`, `(+ (values 1 2) 1)`} {
		_, err := engine().Eval(context.Background(), "bad.scm", s)
		if err == nil {
			t.Fatal("accepted invalid control", s)
		}
	}
}
func TestMultipleValueBindingRegionsAndDefinitions(t *testing.T) {
	for s, want := range map[string]string{
		`(define-values (a b) (values 20 22)) (+ a b)`:                                    `42`,
		`(define-values (a . rest) (values 1 2 3)) rest`:                                  `(2 3)`,
		`(define-values args (values 1 2)) args`:                                          `(1 2)`,
		`(define-values () (values)) 42`:                                                  `42`,
		`(let ((x 10)) (let-values (((x) (values 1)) ((y) (values x))) y))`:               `10`,
		`(let*-values (((x) (values 20)) ((y) (values (+ x 2)))) (+ x y))`:                `42`,
		`(let ((x 1)) (let*-values (((f) (values (lambda () x))) ((x) (values 2))) (f)))`: `1`,
		`(let-values (((a . rest) (values 1 2 3))) rest)`:                                 `(2 3)`,
		`(let-values ((args (values 1 2))) args)`:                                         `(1 2)`,
		`(let-values ((() (values))) 42)`:                                                 `42`,
		`((lambda () (define-values (f g) (values (lambda () g) (lambda () 42))) ((f))))`: `42`,
		`(let*-values (((x) (values 1)) ((x) (values 42))) x)`:                            `42`,
		`(let ((call-with-values 1) (apply 1)) (let-values (((x) (values 42))) x))`:       `42`,
	} {
		t.Run(s, func(t *testing.T) {
			if got := repr(eval(t, engine(), s)); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
	for _, s := range []string{`(let-values (((x) (values 1)) ((x) (values 2))) x)`, `(define-values (x x) (values 1 2))`, `(let-values (((x y) (values 1))) x)`, `(define-values (x y) (values 1))`} {
		_, err := engine().Eval(context.Background(), "bad.scm", s)
		if err == nil {
			t.Fatal("invalid multiple-value binding", s)
		}
	}
}
func TestControlMultiShotAndProperTailCalls(t *testing.T) {
	e := engine()
	s := `(define k #f) (define n 0) (define result (+ 1 (call/cc (lambda (c) (set! k c) 1)))) (set! n (+ n 1)) (+ result n)`
	if number(eval(t, e, s)) != 3 || number(eval(t, e, `(k 10)`)) != 13 || number(eval(t, e, `(k 20)`)) != 24 {
		t.Fatal("multi-shot continuation")
	}
	for _, source := range []string{
		`(let loop ((n 100000)) (if (= n 0) 42 (apply loop (values (quasiquote ((unquote (- n 1))))))))`,
		`(let loop ((n 100000)) (if (= n 0) 42 (call/cc (lambda (k) (loop (- n 1))))))`,
		`(let loop ((n 100000)) (if (= n 0) 42 (call-with-values (lambda () (values (- n 1))) loop)))`,
	} {
		e := engine()
		if number(eval(t, e, source)) != 42 || e.Stats().PeakFrames > 6 {
			t.Fatal("control tail call", e.Stats())
		}
	}
	_, err := core.NewCore().Eval(context.Background(), "value.scm", `(if (values 1 2) 1 2)`)
	if !errors.Is(err, scheme.ErrValueCount) {
		t.Fatal("single-value context", err)
	}
}

func TestMultiShotProducerContinuationAndCyclicApply(t *testing.T) {
	e := engine()
	s := `(define saved #f) (define result (call-with-values (lambda () (call/cc (lambda (k) (set! saved k) (values 1 2)))) +)) result`
	if number(eval(t, e, s)) != 3 || number(eval(t, e, `(saved 20 22)`)) != 42 || number(eval(t, e, `(saved 3 4)`)) != 7 {
		t.Fatal("producer continuation lost native resume")
	}
	_, err := e.Eval(context.Background(), "bad.scm", `(saved 1 2 3)`)
	if err == nil {
		t.Fatal("consumer accepted wrong value count")
	}
	if number(eval(t, e, `(saved 20 22)`)) != 42 {
		t.Fatal("failed invocation damaged a saved continuation")
	}
	p := scheme.Cons(scheme.NewInteger(1), scheme.Null)
	if err := p.SetCdr(p); err != nil {
		t.Fatal(err)
	}
	e.Bind("cycle", p)
	_, err = e.Eval(context.Background(), "cycle.scm", `(apply + cycle)`)
	if !errors.Is(err, scheme.ErrType) {
		t.Fatal("cyclic apply list", err)
	}
}
