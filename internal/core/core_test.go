package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func number(v scheme.Value) int64 { return v.(*scheme.Integer).BigInt().Int64() }
func engine() *core.Engine {
	return core.NewCore()
}

func eval(t *testing.T, e *core.Engine, source string) scheme.Value {
	t.Helper()
	r, err := e.Eval(context.Background(), "test.scm", source)
	if err != nil {
		t.Fatal(err)
	}
	v, err := r.Single()
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestCoreSyntaxAndLexicalClosures(t *testing.T) {
	tests := map[string]int64{
		`((lambda (x) x) 42)`: 42,
		`(define make (lambda (x) (lambda (y) (+ x y)))) (define f (make 10)) (f 5)`:                   15,
		`(define (counter) (let ((n 0)) (lambda () (set! n (+ n 1)) n))) (define f (counter)) (f) (f)`: 2,
		`(define x 1) (define (f) x) (define x 2) (f)`:                                                 2,
		`(if '() 1 2)`: 1, `(if #f missing 2)`: 2,
		`(let ((x 1)) (let ((x 2) (y x)) (+ x y)))`:               3,
		`(let ((x 1)) (let* ((f (lambda () x)) (x 2)) (f)))`:      1,
		`(let* ((x 1) (x (+ x 1))) x)`:                            2,
		`(letrec* ((x 1) (y (+ x 1))) y)`:                         2,
		`((lambda () (define x 1) (begin (define y (+ x 1))) y))`: 2,
		`(letrec ((even (lambda (n) (if (= n 0) 1 (odd (- n 1))))) (odd (lambda (n) (if (= n 0) 0 (even (- n 1)))))) (even 20))`: 1,
		`(let ((if (lambda (a b c) b))) (if #f 1 2))`:   1,
		`(let ((quote (lambda (x) x))) (quote 3))`:      3,
		`((lambda (lambda) (lambda 4)) (lambda (x) x))`: 4,
		`(let ((loop 9)) (let loop ((x loop)) x))`:      9,
	}
	for source, want := range tests {
		t.Run(source, func(t *testing.T) {
			if got := number(eval(t, engine(), source)); got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		})
	}
}
func TestRestArgumentsAndConstantData(t *testing.T) {
	for _, source := range []string{`((lambda args args) 1 2)`, `((lambda (x . args) args) 0 1 2)`} {
		v := eval(t, engine(), source).(*scheme.Pair)
		if number(v.Car()) != 1 || number(v.Cdr().(*scheme.Pair).Car()) != 2 {
			t.Fatal("rest list")
		}
		if err := v.SetCar(scheme.True); err != nil {
			t.Fatal("rest frozen", err)
		}
	}
	for _, source := range []string{`'(1 2)`, `#(1 2)`, `"ab"`, `#u8(1 2)`} {
		v := eval(t, engine(), source)
		var err error
		switch x := v.(type) {
		case *scheme.Pair:
			err = x.SetCar(scheme.True)
		case *scheme.Vector:
			err = x.Set(0, scheme.True)
		case *scheme.String:
			c, _ := scheme.NewCharacter('c')
			err = x.Set(0, c)
		case *scheme.Bytevector:
			err = x.Set(0, 3)
		}
		if !errors.Is(err, scheme.ErrImmutable) {
			t.Fatal("mutable constant", source, err)
		}
	}
	if eval(t, engine(), `(if #f 1)`) != scheme.Unspecified {
		t.Fatal("missing alternate")
	}
	if eval(t, engine(), ``) != scheme.Unspecified {
		t.Fatal("empty program")
	}
}
func TestCompiledTailCallsAndEvaluationOrder(t *testing.T) {
	e := engine()
	v := eval(t, e, `(let loop ((n 100000) (acc 0)) (if (= n 0) acc (begin 1 (loop (- n 1) (+ acc 1)))))`)
	if number(v) != 100000 || e.Stats().PeakFrames > 3 {
		t.Fatal("tail compilation", e.Stats())
	}
	var order []int64
	e.Bind("mark", vm.Native("mark", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		order = append(order, number(a[0]))
		return vm.Return(a[0]), nil
	}))
	eval(t, e, `((begin (mark 1) (lambda (x y) y)) (mark 2) (mark 3))`)
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatal("evaluation order", order)
	}
}
func TestCompiledContinuationAndMultipleResults(t *testing.T) {
	e := engine()
	var saved *scheme.Procedure
	e.Bind("capture", vm.Native("capture", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) {
		saved = c.Capture()
		return vm.Return(scheme.NewInteger(1)), nil
	}))
	if number(eval(t, e, `(define n 0) (define result (+ (capture) 2)) (set! n (+ n 1)) result`)) != 3 {
		t.Fatal("initial")
	}
	e.Bind("saved", saved)
	if number(eval(t, e, `(saved 10)`)) != 12 {
		t.Fatal("resumed program return")
	}
	if schemeInt(t, e, `n`) != 2 {
		t.Fatal("top-level continuation omitted subsequent expressions")
	}
	if number(eval(t, e, `(saved 20)`)) != 22 || schemeInt(t, e, `n`) != 3 {
		t.Fatal("multi-shot top-level continuation")
	}
	if schemeInt(t, e, `result`) != 22 {
		t.Fatal("pending argument")
	}
	e.Bind("many", vm.Native("many", 0, 0, func(_ *vm.Context, _ []scheme.Value) (vm.Action, error) {
		return vm.Return(scheme.True, scheme.False), nil
	}))
	r, err := e.Eval(context.Background(), "values.scm", `((lambda () (if #t (many) 1)))`)
	if err != nil || r.Len() != 2 {
		t.Fatal("tail values", r, err)
	}
	_, err = e.Eval(context.Background(), "values.scm", `(+ (many) 1)`)
	if !errors.Is(err, scheme.ErrValueCount) {
		t.Fatal("value count", err)
	}
}
func schemeInt(t *testing.T, e *core.Engine, s string) int64 {
	t.Helper()
	return number(eval(t, e, s))
}
func TestMalformedSyntaxAndUninitializedBindings(t *testing.T) {
	for _, source := range []string{`()`, `(quote)`, `(quote 1 2)`, `(if 1)`, `(if 1 2 3 4)`, `(lambda (x x) x)`, `(lambda (x . x) x)`, `(lambda (1) 2)`, `(lambda ())`, `(set! 1 2)`, `(define)`, `(define x 1 2)`, `(let ((x 1) (x 2)) x)`, `(let ((x)) x)`, `(letrec ((x 1) (x 2)) x)`, `((lambda () 1 (define x 2) x))`, `(if #t (define x 1) 2)`, `(if #t (begin) 1)`, `((lambda () (define define 1) 2))`, `((lambda () (begin (define begin 1)) 2))`, `(fn (x) x)`} {
		t.Run(source, func(t *testing.T) {
			_, err := engine().Eval(context.Background(), "bad.scm", source)
			if err == nil {
				t.Fatal("accepted malformed source")
			}
		})
	}
	for _, source := range []string{`missing`, `(set! missing 1)`, `(letrec ((x 1) (y x)) y)`, `(letrec* ((x y) (y 1)) x)`, `(letrec ((x (set! x 1))) x)`} {
		_, err := engine().Eval(context.Background(), "bad.scm", source)
		if !errors.Is(err, scheme.ErrUninitialized) {
			t.Fatal("uninitialized", source, err)
		}
	}
	_, err := engine().Eval(context.Background(), "arity.scm", `((lambda (x) x))`)
	if !errors.Is(err, vm.ErrArity) {
		t.Fatal("arity", err)
	}
	_, err = engine().Eval(context.Background(), "where.scm", "\n(if #t 1 2 3)")
	var diagnostic *core.Error
	if !errors.As(err, &diagnostic) || diagnostic.Source.Line != 2 {
		t.Fatal("source", err)
	}
	// Bare names from old Guri remain ordinary unbound identifiers.
	for _, source := range []string{`nil`, `true`, `false`, `def`, `defn`, `fn`} {
		_, err = engine().Eval(context.Background(), "legacy.scm", source)
		if err == nil {
			t.Fatal("legacy special form", source)
		}
	}
}

func TestInternalDefinitionsHaveTheirOwnRegion(t *testing.T) {
	for source, want := range map[string]int64{
		`((lambda (x) (define x 2) x) 1)`:                               2,
		`(let ((x 1)) (define x 2) x)`:                                  2,
		`((lambda () (define begin (lambda (x) (+ x 1))) (begin 4)))`:   5,
		`((lambda () (define if (lambda (a b c) b)) (if #f 1 2)))`:      1,
		`((lambda () (begin (define x 1) (define y (+ x 1))) (+ x y)))`: 3,
	} {
		t.Run(source, func(t *testing.T) {
			if got := number(eval(t, engine(), source)); got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		})
	}
}

func TestReferenceExamplesWithoutInjectedProcedures(t *testing.T) {
	for source, want := range map[string]int64{
		`((lambda (x) x) 42)`:       42,
		`(if '() 1 2)`:              1,
		`(define x 1) (set! x 2) x`: 2,
		`(define (id x) x) (id 42)`: 42,
		`(begin 1 2)`:               2,
		`(let ((x 1) (y 2)) y)`:     2,
		`(let* ((x 1) (y x)) y)`:    1,
		`(letrec ((first (lambda () (second))) (second (lambda () 42))) (first))`: 42,
		`(letrec* ((x 1) (y x)) y)`:                          1,
		`(let loop ((again #t)) (if again (loop #f) 42))`:    42,
		`(define f (lambda () later)) (define later 42) (f)`: 42,
	} {
		t.Run(source, func(t *testing.T) {
			if got := number(eval(t, core.NewCore(), source)); got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		})
	}
	if eval(t, core.NewCore(), `'symbol`).(*scheme.Symbol).Name() != "symbol" {
		t.Fatal("quoted symbol")
	}
}
