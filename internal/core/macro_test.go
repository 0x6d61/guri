package core_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestMacroHygieneAndDefinitionBindings(t *testing.T) {
	tests := map[string]int64{
		`(define-syntax swap! (syntax-rules () ((_ a b) (let ((temp a)) (set! a b) (set! b temp))))) (let ((temp 1) (x 2)) (swap! temp x) (- temp x))`: 1,
		`(let ((x 10)) (let-syntax ((get-x (syntax-rules () ((_) x)))) (let ((x 20)) (get-x))))`:                                                       10,
		`(define-syntax choose (syntax-rules () ((_ x a b) (if x a b)))) (let ((if (lambda (x a b) b))) (choose #t 1 2))`:                              1,
		`(let ((f (lambda () 7))) (let-syntax ((m (syntax-rules () ((_) (f))))) (let ((f (lambda () 9))) (m))))`:                                       7,
		`(define-syntax get (syntax-rules () ((_ x) x))) (let ((get (lambda (x) (+ x 1)))) (get 3))`:                                                   4,
		`(define-syntax identity (syntax-rules () ((_ x) x))) (identity (let ((x 42)) x))`:                                                             42,
		`(let-syntax ((make (syntax-rules () ((_ name) (define (name) 42))))) (make f) (f))`:                                                           42,
		`((lambda () (define-syntax d (syntax-rules () ((_ name value) (define name value)))) (d x 42) x))`:                                            42,
		`((lambda () (define-syntax a (syntax-rules () ((_) (b)))) (define-syntax b (syntax-rules () ((_) 42))) (a)))`:                                 42,
		`(define-syntax a (syntax-rules () ((_) 1))) (let-syntax ((a (syntax-rules () ((_) 2))) (b (syntax-rules () ((_) (a))))) (b))`:                 1,
		`(define-syntax a (syntax-rules () ((_) 1))) (letrec-syntax ((a (syntax-rules () ((_) 2))) (b (syntax-rules () ((_) (a))))) (b))`:              2,
		`(letrec-syntax ((a (syntax-rules () ((_ x) (b x)))) (b (syntax-rules () ((_ x) x)))) (a 42))`:                                                 42,
	}
	for source, want := range tests {
		t.Run(source, func(t *testing.T) {
			if got := number(eval(t, engine(), source)); got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		})
	}
}
func TestLiteralIdentifiersUseBindingIdentity(t *testing.T) {
	tests := map[string]int64{
		`(define-syntax m (syntax-rules (else) ((_ else) 1) ((_ x) 2))) (m else)`:                               1,
		`(define-syntax m (syntax-rules (else) ((_ else) 1) ((_ x) 2))) (let ((else 0)) (m else))`:              2,
		`(let ((tag 0)) (let-syntax ((m (syntax-rules (tag) ((_ tag) 1) ((_ _) 2)))) (m tag)))`:                 1,
		`(let ((tag 0)) (let-syntax ((m (syntax-rules (tag) ((_ tag) 1) ((_ _) 2)))) (let ((tag 0)) (m tag))))`: 2,
		`(define-syntax m (syntax-rules (_) ((_ _) 1) ((_ x) 2))) (m anything)`:                                 2,
		`(define-syntax m (syntax-rules (...) ((_ ...) 1) ((_ x) 2))) (m ...)`:                                  1,
		`(define-syntax m (syntax-rules () ((_ _ _) 42))) (m 1 2)`:                                              42,
	}
	for source, want := range tests {
		t.Run(source, func(t *testing.T) {
			if got := number(eval(t, engine(), source)); got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		})
	}
}
func TestEllipsesVectorsAndDottedPatterns(t *testing.T) {
	tests := map[string]int64{
		`(define-syntax sum (syntax-rules () ((_) 0) ((_ x xs ...) (+ x (sum xs ...))))) (sum 1 2 3 4)`:                                                          10,
		`(define-syntax seq (syntax-rules () ((_ x ...) (begin x ... 42)))) (seq)`:                                                                               42,
		`(define-syntax last (syntax-rules () ((_ x ... y) y))) (last 1 2 42)`:                                                                                   42,
		`(define-syntax m (syntax-rules () ((_ #(a ... z)) z))) (m #(1 2 42))`:                                                                                   42,
		`(define-syntax m (syntax-rules () ((_ (a ... . tail)) tail))) (m (1 2 . 42))`:                                                                           42,
		`(define-syntax m (syntax-rules () ((_ (a . tail)) (quote tail)))) (m (1 . 42))`:                                                                         42,
		`(define-syntax seq (syntax-rules ::: () ((_ x :::) (begin x :::)))) (seq 1 2 42)`:                                                                       42,
		`(define-syntax flatten (syntax-rules () ((_ ((x ...) ...)) (begin x ... ... 42)))) (flatten (() (1 2) () (3)))`:                                         42,
		`(define-syntax flatten (syntax-rules () ((_ ((x ...) ...)) (begin x ... ...)))) (flatten ((1 2) (3 42)))`:                                               42,
		`(define-syntax m (syntax-rules () ((_ #u8(1 2)) 42) ((_ _) 0))) (m #u8(1 2))`:                                                                           42,
		`(define-syntax m (syntax-rules () ((_ "ok") 42) ((_ _) 0))) (m "ok")`:                                                                                   42,
		`(define-syntax m (syntax-rules () ((_ 1) 1) ((_ _) 2))) (m 1.0)`:                                                                                        2,
		`(define-syntax make (syntax-rules () ((_ name) (define-syntax name (syntax-rules () ((_ x (... ...)) (begin x (... ...)))))))) (make seq) (seq 1 2 42)`: 42,
	}
	for source, want := range tests {
		t.Run(source, func(t *testing.T) {
			if got := number(eval(t, engine(), source)); got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		})
	}
	v := eval(t, engine(), `(define-syntax matrix (syntax-rules () ((_ ((x ...) ...)) (quote #(#(x ...) ...))))) (matrix (() (1 2) (3)))`).(*scheme.Vector)
	first, _ := v.Ref(0)
	last, _ := v.Ref(2)
	if first.(*scheme.Vector).Len() != 0 || last.(*scheme.Vector).Len() != 1 {
		t.Fatal("ragged nested repetitions")
	}
}
func TestMacroErrorsSourcesAndCancellation(t *testing.T) {
	for _, source := range []string{
		`(define-syntax m (syntax-rules () ((_ x x) x)))`,
		`(define-syntax m (syntax-rules () ((_ x ...) x)))`,
		`(define-syntax m (syntax-rules () ((_ x) (x ...))))`,
		`(define-syntax m (syntax-rules () ((_ x) x))) (m)`,
		`(define-syntax m (syntax-rules (1) ((_) 1)))`,
		`(define-syntax m (syntax-rules () (1 2)))`,
		`(define-syntax m (syntax-rules () ((_) ...)))`,
		`(define-syntax m (syntax-rules () ((_ (x ...) (y ...)) (quote ((x y) ...))))) (m (1) (2 3))`,
		`(let-syntax ((m (syntax-rules () ((_) 1))) (m (syntax-rules () ((_) 2)))) (m))`,
		`(syntax-rules () ((_) 1))`, `(define-syntax m 1)`, `(syntax-error 1)`,
		`((lambda () 1 (define-syntax m (syntax-rules () ((_) 2))) (m)))`,
	} {
		t.Run(source, func(t *testing.T) {
			_, err := engine().Eval(context.Background(), "bad.scm", source)
			if !errors.Is(err, core.ErrSyntax) {
				t.Fatal("macro error", err)
			}
		})
	}
	_, err := engine().Eval(context.Background(), "use.scm", "(define-syntax m (syntax-rules () ((_ x) x)))\n(m)")
	var diagnostic *core.Error
	if !errors.As(err, &diagnostic) || diagnostic.Source.Filename != "use.scm" || diagnostic.Source.Line != 2 {
		t.Fatal("use location", err)
	}
	_, err = engine().Eval(context.Background(), "message.scm", `(define-syntax m (syntax-rules () ((_ x) (syntax-error "expected a name" x)))) (m (missing))`)
	if !errors.Is(err, core.ErrSyntax) || !strings.Contains(err.Error(), "expected a name") {
		t.Fatal("syntax-error message", err)
	}
	e := engine()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = e.Eval(ctx, "loop.scm", `(define-syntax loop (syntax-rules () ((_) (loop)))) (loop)`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("expansion cancellation", err)
	}
}

func TestMacroCompositionAndUnboundDefinitionReferences(t *testing.T) {
	tests := map[string]int64{
		`(define-syntax m (syntax-rules (tag tag) ((_ tag) 42))) (m tag)`:                                                                                                    42,
		`(define-syntax make (syntax-rules () ((_ name) (let ((x 42)) (define-syntax name (syntax-rules () ((_) x))) (let ((x 0)) (name)))))) (make getter)`:                 42,
		`(define-syntax first (syntax-rules () ((_ x xs ...) x))) (define-syntax wrap (syntax-rules () ((_ x) (let ((temp x)) (first temp 0))))) (let ((temp 9)) (wrap 42))`: 42,
		`(define-syntax select (syntax-rules () ((_ x) (if x 42 0)))) (define if (lambda (x a b) b)) (select #t)`:                                                            42,
		`(define-syntax same (syntax-rules () ((_ x) x))) (let ((m (lambda (x) 42))) ((same m) 1))`:                                                                          42,
	}
	for source, want := range tests {
		t.Run(source, func(t *testing.T) {
			if got := number(eval(t, engine(), source)); got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		})
	}
	_, err := engine().Eval(context.Background(), "free.scm", `(define-syntax m (syntax-rules () ((_) unbound-at-definition))) (let ((unbound-at-definition 42)) (m))`)
	if !errors.Is(err, scheme.ErrUninitialized) {
		t.Fatal("use-site binding captured a definition-site free identifier", err)
	}
}
func TestSyntaxErrorRetainsUnevaluatedIrritants(t *testing.T) {
	_, err := engine().Eval(context.Background(), "diagnostic.scm", `(define-syntax m (syntax-rules () ((_ x) (syntax-error "bad argument" x)))) (m (never-evaluate))`)
	var failure *core.SyntaxFailure
	if !errors.As(err, &failure) || len(failure.Irritants) != 1 || failure.Definition.Filename != "diagnostic.scm" {
		t.Fatal("missing macro diagnostic metadata", err)
	}
	if failure.Irritants[0].(*scheme.Pair).Car().(*scheme.Symbol).Name() != "never-evaluate" {
		t.Fatal("irritant was not retained as datum")
	}
}

func TestGeneratedLiteralBindingsAndQuotedIdentifierNames(t *testing.T) {
	e := engine()
	source := `(define tag 1)
 (define-syntax make-match (syntax-rules () ((_ name) (define-syntax name (syntax-rules (tag) ((_ tag) 42) ((_ x) 0))))))
 (make-match match-tag) (match-tag tag)`
	if number(eval(t, e, source)) != 42 {
		t.Fatal("generated literal lost its binding")
	}
	v := eval(t, engine(), `(define-syntax symbol (syntax-rules () ((_) (quote introduced-name)))) (symbol)`)
	if v.(*scheme.Symbol).Name() != "introduced-name" {
		t.Fatal("hygiene changed quoted Symbol spelling")
	}
	v = eval(t, engine(), `(let-syntax ((given-that (syntax-rules () ((_ test stmt ...) (if test (begin stmt ...)))))) (let ((if #t)) (given-that if (set! if 'now)) if))`)
	if v.(*scheme.Symbol).Name() != "now" {
		t.Fatal("definition keyword and use variable were confused")
	}
}

func TestMacroReferenceExamples(t *testing.T) {
	for source, want := range map[string]int64{
		`(define-syntax answer (syntax-rules () ((_) 42))) (answer)`:          42,
		`(let-syntax ((keep (syntax-rules () ((_ value) value)))) (keep 42))`: 42,
		`(letrec-syntax ((my-or (syntax-rules () ((_) #f) ((_ first rest ...) (let ((saved first)) (if saved saved (my-or rest ...))))))) (let ((saved 9)) (my-or #f 42)))`: 42,
		`(define-syntax seq (syntax-rules ::: () ((_ first rest :::) (begin first rest :::)))) (seq 1 2 42)`:                                                                42,
		`(define-syntax define-constant (syntax-rules () ((_ name value) (define name value)))) (define-constant answer 42) answer`:                                         42,
	} {
		t.Run(source, func(t *testing.T) {
			if got := number(eval(t, core.NewCore(), source)); got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		})
	}
	_, err := core.NewCore().Eval(context.Background(), "example.scm", `(define-syntax expect-one (syntax-rules () ((_ x) x) ((_ rest ...) (syntax-error "expected one value" rest ...)))) (expect-one)`)
	if !errors.Is(err, core.ErrSyntax) || !strings.Contains(err.Error(), "expected one value") {
		t.Fatal("syntax-error example", err)
	}
}
