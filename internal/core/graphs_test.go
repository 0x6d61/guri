package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestCyclicExecutableDiagnostics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := core.NewCore().Eval(ctx, "cycle.scm", `#0=(begin #0#)`)
	if !errors.Is(err, core.ErrSyntax) {
		t.Fatalf("cyclic code needs SyntaxError, got %v", err)
	}
}
func TestCyclicQuotedLiterals(t *testing.T) {
	for _, src := range []string{
		`(define x '#0=(a . #0#)) (eq? x (cdr x))`,
		`(define x '#0=#(#0#)) (eq? x (vector-ref x 0))`,
		`(guard (e (else #t)) (set-car! '#0=(a . #0#) #t) #f)`,
	} {
		if v := eval(t, engine(), src); v != scheme.True {
			t.Fatal(src, v)
		}
	}
}

func TestMacroQuotedGraphSharing(t *testing.T) {
	src := `(define-syntax mk (syntax-rules () ((_) (quote (#0=(a) #0#))))) (define x (mk)) (eq? (car x) (cadr x))`
	if v := eval(t, engine(), src); v != scheme.True {
		t.Fatal("template expansion lost datum-label identity", v)
	}
}

func TestMacroCyclicLiteralSubstitution(t *testing.T) {
	for _, src := range []string{
		`(define-syntax mk (syntax-rules () ((_ x) (quote #0=(x . #0#))))) (define result (mk 7)) (and (= (car result) 7) (eq? result (cdr result)))`,
		`(define-syntax mk (syntax-rules () ((_) (quote #0=#(#0#))))) (define result (mk)) (eq? result (vector-ref result 0))`,
	} {
		if v := eval(t, engine(), src); v != scheme.True {
			t.Fatal(src, v)
		}
	}
}
func TestMacroEllipsisKeepsSharedLiteral(t *testing.T) {
	src := `(define-syntax mk (syntax-rules () ((_ x ...) (quote (x ... #0=(a) #0#))))) (define result (mk 1 2)) (eq? (list-ref result 2) (list-ref result 3))`
	if v := eval(t, engine(), src); v != scheme.True {
		t.Fatal("ellipsis expansion lost datum label sharing", v)
	}
}
func TestCyclicSyntaxAndQuasiquoteRejected(t *testing.T) {
	for _, src := range []string{`#0=(begin . #0#)`, `#0=(lambda () #0#)`, "`#0=(a . #0#)", "`#0=#(#0#)", `(let ((quote (lambda (x) x))) #0=(quote #0#))`} {
		_, err := core.NewCore().Eval(context.Background(), "cycle.scm", src)
		if !errors.Is(err, core.ErrSyntax) {
			t.Fatal(src, err)
		}
	}
}

func TestMacroGraphTailsAndCyclicEllipsis(t *testing.T) {
	for _, src := range []string{
		`(define-syntax mk (syntax-rules () ((_) (quote ((a . #0=(b c)) #0#))))) (define x (mk)) (eq? (cdr (car x)) (cadr x))`,
		`(define-syntax mk (syntax-rules () ((_ x ...) (quote #0=(x ... . #0#))))) (define x (mk 1 2)) (and (= (car x) 1) (= (cadr x) 2) (eq? x (cddr x)))`,
		`(define-syntax mk (syntax-rules () ((_ x ...) (quote #0=#(x ... #0#))))) (define x (mk 1 2)) (and (= (vector-length x) 3) (eq? x (vector-ref x 2)))`,
		`(define-syntax mk (syntax-rules () ((_ (x ...) ...) (quote ((#0=(x ...) #0#) ...))))) (define x (mk (1 2) (3))) (and (eq? (caar x) (cadar x)) (eq? (caadr x) (cadadr x)) (= (car (caadr x)) 3))`,
		`(define-syntax inspect (syntax-rules () ((_ (quote (a b . rest))) (quote rest)))) (define x (inspect '#0=(1 . #0#))) (eq? x (cdr x))`,
	} {
		t.Run(src, func(t *testing.T) {
			if v := eval(t, engine(), src); v != scheme.True {
				t.Fatal(v)
			}
		})
	}
}

func TestCyclicTemplatesEscapesAndEmptyRepeats(t *testing.T) {
	for _, src := range []string{
		`(define-syntax mk (syntax-rules () ((_) (quote (... #0=(a ... . #0#)))))) (define x (mk)) (eq? x (cddr x))`,
		`(define-syntax mk (syntax-rules () ((_ x ...) (quote #0=(x ... a . #0#))))) (define x (mk)) (and (eq? (car x) 'a) (eq? x (cdr x)))`,
	} {
		if v := eval(t, engine(), src); v != scheme.True {
			t.Fatal(src, v)
		}
	}
	_, err := core.NewCore().Eval(context.Background(), "empty-cycle.scm", `(define-syntax mk (syntax-rules () ((_ x ...) (quote #0=(x ... . #0#))))) (mk)`)
	if !errors.Is(err, core.ErrSyntax) {
		t.Fatal("no object after expansion", err)
	}
}
