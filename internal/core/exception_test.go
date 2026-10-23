package core_test

import (
	"context"
	"errors"
	"github.com/0x6d61/guri/internal/vm"
	"reflect"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestExceptionHandlersAndGuard(t *testing.T) {
	for s, want := range map[string]string{
		`(with-exception-handler (lambda (x) (+ x 1)) (lambda () (+ 1 (raise-continuable 40))))`:          `42`,
		`(call/cc (lambda (exit) (with-exception-handler (lambda (x) (exit x)) (lambda () (raise 42)))))`: `42`,
		`(guard (e (else e)) (raise 42))`:                                 `42`,
		`(guard (e ((= e 1) 0) ((= e 42) => (lambda (x) e))) (raise 42))`: `42`,
		`(call/cc (lambda (exit) (with-exception-handler (lambda (secondary) (exit 42)) (lambda () (with-exception-handler (lambda (x) 1) (lambda () (raise 'original)))))))`: `42`,
		`(with-exception-handler (lambda (x) 42) (lambda () (guard (e (#f 0)) (raise-continuable 1))))`:                                                                       `42`,
		`(guard (e ((error-object? e) (error-object-message e))) (error "bad" 1 2))`:                                                                                          `"bad"`,
		`(guard (e ((error-object? e) (error-object-irritants e))) (error "bad" 1 2))`:                                                                                        `(1 2)`,
		`(guard (e (else 0)) 42)`: `42`,
	} {
		t.Run(s, func(t *testing.T) {
			if got := repr(eval(t, engine(), s)); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
	e := engine()
	r, err := e.Eval(context.Background(), "multi.scm", `(with-exception-handler (lambda (x) (values 20 22)) (lambda () (raise-continuable 1)))`)
	if err != nil || r.Len() != 2 {
		t.Fatal("continuable results", r, err)
	}
	_, err = e.Eval(context.Background(), "raise.scm", `(raise 'unhandled)`)
	var raised *scheme.Raised
	if !errors.As(err, &raised) || raised.Object().(*scheme.Symbol).Name() != "unhandled" {
		t.Fatal("unhandled payload", err)
	}
}
func TestExceptionsPreserveWindAndDynamicParameters(t *testing.T) {
	e, log := windEngine()
	s := `(with-exception-handler (lambda (x) (record 'handler) 42) (lambda () (dynamic-wind (lambda () (record 'in)) (lambda () (raise-continuable 1)) (lambda () (record 'out)))))`
	if number(eval(t, e, s)) != 42 || !reflect.DeepEqual(*log, []string{"in", "handler", "out"}) {
		t.Fatal("handler ran outside raise extent", *log)
	}
	*log = nil
	s = `(guard (e (else (record 'guard) e)) (dynamic-wind (lambda () (record 'in)) (lambda () (raise 42)) (lambda () (record 'out))))`
	if number(eval(t, e, s)) != 42 || !reflect.DeepEqual(*log, []string{"in", "out", "guard"}) {
		t.Fatal("guard environment", *log)
	}
	*log = nil
	_, err := e.Eval(context.Background(), "unhandled.scm", `(dynamic-wind (lambda () (record 'in)) (lambda () (raise 'x)) (lambda () (record 'out)))`)
	var raised *scheme.Raised
	if !errors.As(err, &raised) || !reflect.DeepEqual(*log, []string{"in", "out"}) {
		t.Fatal("unhandled unwind", err, *log)
	}
}
func TestParametersConversionRegionAndReentry(t *testing.T) {
	for s, want := range map[string]string{
		`(define p (make-parameter 42)) (p)`:                                                                                                `42`,
		`(define p (make-parameter 1)) (parameterize ((p 42)) (p))`:                                                                         `42`,
		`(define p (make-parameter 1)) (parameterize ((p 42)) (p)) (p)`:                                                                     `1`,
		`(define p (make-parameter 1)) (parameterize ((p 20)) (parameterize ((p 42)) (p)))`:                                                 `42`,
		`(define p (make-parameter 20 (lambda (x) (+ x 1)))) (parameterize ((p 41)) (p))`:                                                   `42`,
		`(define p (make-parameter 1)) (define q (make-parameter 2)) (parameterize ((p 10) (q (p))) (q))`:                                   `1`,
		`(define p (make-parameter 1)) (with-exception-handler (lambda (x) (p)) (lambda () (parameterize ((p 42)) (raise-continuable 0))))`: `42`,
		`(define p (make-parameter 1)) (guard (e (else (p))) (parameterize ((p 42)) (raise 0)))`:                                            `1`,
	} {
		t.Run(s, func(t *testing.T) {
			if got := repr(eval(t, engine(), s)); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
	e := engine()
	s := `(define conversions 0) (define k #f) (define p (make-parameter 1 (lambda (x) (set! conversions (+ conversions 1)) x))) (parameterize ((p 42)) (call/cc (lambda (c) (set! k c) 0)) (p))`
	if number(eval(t, e, s)) != 42 || number(eval(t, e, `(k 1)`)) != 42 || number(eval(t, e, `conversions`)) != 2 || number(eval(t, e, `(p)`)) != 1 {
		t.Fatal("parameter snapshot or converter rerun")
	}
	for _, s := range []string{`(parameterize ((1 2)) 0)`, `(make-parameter 1 2)`, `(parameterize ((p 1 2)) 0)`} {
		_, err := engine().Eval(context.Background(), "bad.scm", s)
		if err == nil {
			t.Fatal("invalid parameter operation", s)
		}
	}
}

func TestGuardLexicalAuxiliariesAndReraiseEnvironment(t *testing.T) {
	if number(eval(t, engine(), `(guard (else (else 42) (#t 7)) (raise #f))`)) != 7 {
		t.Fatal("guard variable did not shadow else")
	}

	e, log := windEngine()
	s := `(with-exception-handler (lambda (x) (record 'parent) 42) (lambda () (guard (e (#f 0)) (dynamic-wind (lambda () (record 'in)) (lambda () (raise-continuable 1)) (lambda () (record 'out))))))`
	if number(eval(t, e, s)) != 42 || !reflect.DeepEqual(*log, []string{"in", "out", "in", "parent", "out"}) {
		t.Fatal("guard reraising context", *log)
	}
	r, err := e.Eval(context.Background(), "guard-values.scm", `(guard (e (else (values 1 2))) (values 20 22))`)
	if err != nil || r.Len() != 2 {
		t.Fatal("guard normal values", r, err)
	}
}

func TestErrorCategoriesAndConverterContinuation(t *testing.T) {
	e := engine()
	read, err := core.NewErrorObject(core.ErrorRead, "bad datum", scheme.NewInteger(42))
	if err != nil {
		t.Fatal(err)
	}
	file, err := core.NewErrorObject(core.ErrorFile, "missing file")
	if err != nil {
		t.Fatal(err)
	}
	e.Bind("read-condition", read)
	e.Bind("file-condition", file)
	for s, want := range map[string]string{`(read-error? read-condition)`: `#t`, `(file-error? read-condition)`: `#f`, `(file-error? file-condition)`: `#t`, `(error-object-irritants read-condition)`: `(42)`, `(error-object? 1)`: `#f`} {
		if got := repr(eval(t, e, s)); got != want {
			t.Fatal(s, got, want)
		}
	}
	s := `(define converter-k #f) (define p (make-parameter 1 (lambda (x) (call/cc (lambda (k) (set! converter-k k) x))))) (parameterize ((p 42)) (p))`
	if number(eval(t, e, s)) != 42 || number(eval(t, e, `(converter-k 43)`)) != 43 || number(eval(t, e, `(converter-k 44)`)) != 44 || number(eval(t, e, `(p)`)) != 1 {
		t.Fatal("converter continuation mutated a saved binding map")
	}
}

func TestUnhandledErrorPreservesMessageAndSourceAfterWind(t *testing.T) {
	_, err := engine().Eval(context.Background(), "diagnostic.scm", "(dynamic-wind\n (lambda () 0)\n (lambda () (error \"diagnostic message\"))\n (lambda () 99))")
	var raised *scheme.Raised
	var diagnostic *vm.Error
	if !errors.As(err, &raised) || !errors.As(err, &diagnostic) || raised.Source().Line != 3 || diagnostic.Source.Line != 3 || !strings.Contains(err.Error(), "diagnostic message") {
		t.Fatal("unhandled diagnostic lost source/message", err)
	}
}

func TestExceptionReferenceExamples(t *testing.T) {
	for s, want := range map[string]string{`(with-exception-handler (lambda (condition) 42) (lambda () (raise-continuable 'problem)))`: `42`, `(guard (condition (else condition)) (raise 42))`: `42`, `(guard (condition ((error-object? condition) (error-object-message condition))) (error "bad input" 1 2))`: `"bad input"`, `(define p (make-parameter 1)) (parameterize ((p 42)) (p)) (p)`: `1`} {
		if got := repr(eval(t, core.NewCore(), s)); got != want {
			t.Fatal(s, got, want)
		}
	}
}
