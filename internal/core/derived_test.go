package core_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func repr(v scheme.Value) string {
	switch x := v.(type) {
	case *scheme.Integer:
		return x.String()
	case *scheme.Symbol:
		return x.Name()
	case *scheme.String:
		return fmt.Sprintf("%q", x.Text())
	case scheme.Boolean:
		if x {
			return "#t"
		}
		return "#f"
	case *scheme.Pair:
		var parts []string
		var d scheme.Value = x
		for d.Kind() == scheme.KindPair {
			p := d.(*scheme.Pair)
			parts = append(parts, repr(p.Car()))
			d = p.Cdr()
		}
		if d != scheme.Null {
			parts = append(parts, ".", repr(d))
		}
		return "(" + strings.Join(parts, " ") + ")"
	case *scheme.Vector:
		var parts []string
		for _, v := range x.Values() {
			parts = append(parts, repr(v))
		}
		return "#(" + strings.Join(parts, " ") + ")"
	default:
		if v == scheme.Null {
			return "()"
		}
		return string(v.Kind())
	}
}
func TestDerivedConditionalsAndEvaluationCounts(t *testing.T) {
	for source, want := range map[string]string{
		`(and)`: `#t`, `(or)`: `#f`, `(and 1 2)`: `2`, `(or #f 42 missing)`: `42`, `(and #f missing)`: `#f`,
		`(cond (#f missing) ((+ 1 2)) (else 9))`:                                   `3`,
		`(cond ((+ 1 2) => (lambda (x) (+ x 1))) (else 0))`:                        `4`,
		`(let ((=> 1)) (cond (#t => 42)))`:                                         `42`,
		`(let ((else #f)) (cond (else 1) (#t 42)))`:                                `42`,
		`(case (+ 1 1) ((0 1) 0) ((2 3) 42) (else 9))`:                             `42`,
		`(case 2 ((2) => (lambda (x) (+ x 40))) (else 0))`:                         `42`,
		`(case 42 ((1) 0) (else => (lambda (x) x)))`:                               `42`,
		`(define n 0) (cond ((begin (set! n (+ n 1)) n) => (lambda (x) (+ n x))))`: `2`,
		`(define n 0) (case (begin (set! n (+ n 1)) n) ((1) n))`:                   `1`,
		`(define x 0) (when #t (set! x 1)) (unless #f (set! x (+ x 1))) x`:         `2`,
		`(let ((if (lambda x 0)) (let 0) (begin 0)) (or #f (cond (#t 42))))`:       `42`,
	} {
		t.Run(source, func(t *testing.T) {
			if got := repr(eval(t, engine(), source)); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
}
func TestDoCaseLambdaAndTailPositions(t *testing.T) {
	for source, want := range map[string]string{
		`(do ((i 0 (+ i 1)) (sum 0 (+ sum i))) ((= i 5) sum))`:   `10`,
		`(do ((a 1 b) (b 2 a) (n 0 (+ n 1))) ((= n 1) (- a b)))`: `1`,
		`(do ((x 42) (done #f #t)) (done x))`:                    `42`,
		`((case-lambda (() 0) ((x) x) ((x y) (+ x y))) 42)`:      `42`,
		`((case-lambda ((x . rest) rest)) 0 1 2)`:                `(1 2)`,
		`((case-lambda (args args)) 1 2)`:                        `(1 2)`,
		`((case-lambda ((x . xs) 1) ((x) 2)) 42)`:                `1`,
	} {
		t.Run(source, func(t *testing.T) {
			if got := repr(eval(t, engine(), source)); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
	e := engine()
	v := eval(t, e, `(let loop ((n 100000)) (cond ((= n 0) 42) (else (and #t (or #f (loop (- n 1)))))))`)
	if number(v) != 42 || e.Stats().PeakFrames > 5 {
		t.Fatal("derived tail call", e.Stats())
	}
	e = engine()
	v = eval(t, e, `(do ((n 100000 (- n 1))) ((= n 0) 42))`)
	if number(v) != 42 || e.Stats().PeakFrames > 5 {
		t.Fatal("do tail call", e.Stats())
	}
	_, err := engine().Eval(context.Background(), "arity.scm", `((case-lambda ((x) x)))`)
	if !errors.Is(err, vm.ErrArity) {
		t.Fatal("case-lambda arity", err)
	}
}
func TestQuasiquoteTemplatesAndSharing(t *testing.T) {
	for source, want := range map[string]string{
		"`(a ,(+ 1 2) ,@'(4 5) . 6)":      `(a 3 4 5 . 6)`,
		"`#(a ,(+ 1 2) ,@'(4 5))":         `#(a 3 4 5)`,
		"(let ((x 42)) `(a `(b ,x ,,x)))": `(a (quasiquote (b (unquote x) (unquote 42))))`,
		"(let ((x 42)) `(a . ,x))":        `(a . 42)`,
		"`(,@'())":                        `()`,
	} {
		t.Run(source, func(t *testing.T) {
			if got := repr(eval(t, engine(), source)); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
	e := engine()
	v := eval(t, e, "(define x 42) `(a ,x (b c))").(*scheme.Pair)
	if err := v.SetCar(scheme.True); err != nil {
		t.Fatal("constructed pair not mutable", err)
	}
	static := v.Cdr().(*scheme.Pair).Cdr().(*scheme.Pair).Car().(*scheme.Pair)
	if err := static.SetCar(scheme.True); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal("static substructure not literal", err)
	}
	for _, s := range []string{"`(,@1)", "`,@'(1)", "`(1 . ,@'(2))", "(unquote 1)", "`(unquote 1 2)"} {
		_, err := engine().Eval(context.Background(), "bad.scm", s)
		if err == nil {
			t.Fatal("accepted bad quasiquote", s)
		}
	}
}
func TestDerivedSyntaxErrors(t *testing.T) {
	for _, s := range []string{`(cond 1)`, `(cond (else 1) (#t 2))`, `(cond (#t =>))`, `(case 1 ((1) 1) ((1) 2))`, `(case 1 (else 1) ((1) 2))`, `(when #t)`, `(do ((x 1) (x 2)) (#t))`, `(case-lambda ((x x) x))`, `(while #t 1)`} {
		_, err := engine().Eval(context.Background(), "bad.scm", s)
		if err == nil {
			t.Fatal("accepted malformed syntax", s)
		}
	}
}

func TestQuasiquoteVectorElementsAndInvalidKeywords(t *testing.T) {
	if got := repr(eval(t, engine(), "(let ((x 42)) `#(quote ,x))")); got != "#(quote 42)" {
		t.Fatal(got)
	}
	for _, source := range []string{"`#(quasiquote 1)", "`#(unquote 1)", "`unquote"} {
		_, err := engine().Eval(context.Background(), "bad.scm", source)
		if !errors.Is(err, core.ErrSyntax) {
			t.Fatal("bare quasiquote keyword", source, err)
		}
	}
}

func TestIncludeAndFeatureResolverContract(t *testing.T) {
	e := engine()
	var requests []string
	e.SetSourceLoader(func(_ context.Context, from, path string) (string, string, error) {
		requests = append(requests, from+":"+path)
		switch path {
		case "a":
			return "a.scm", `(define X 40)`, nil
		case "b":
			return "b.scm", `(+ X 2)`, nil
		case "lower":
			return "lower.scm", `(define VALUE 42)`, nil
		default:
			return "", "", fmt.Errorf("not found: %s", path)
		}
	})
	if number(eval(t, e, `(include "a" "b")`)) != 42 || len(requests) != 2 || requests[0] != "test.scm:a" {
		t.Fatal("include order", requests)
	}
	if number(eval(t, e, `(include-ci "lower") value`)) != 42 {
		t.Fatal("include-ci folding")
	}
	e.SetFeatures([]string{"present"}, func(name []scheme.Value) bool { return len(name) == 2 && name[0].(*scheme.Symbol).Name() == "scheme" })
	if number(eval(t, e, `(cond-expand ((and present (not absent) (library (scheme base))) 42) (else missing))`)) != 42 {
		t.Fatal("feature resolver")
	}
	if number(eval(t, e, `(cond-expand (absent (syntax-error "unselected")) (else 42))`)) != 42 {
		t.Fatal("unselected expansion")
	}
	if number(eval(t, e, `((lambda () (include "a") X))`)) != 40 {
		t.Fatal("included internal definition")
	}
	_, err := core.NewCore().Eval(context.Background(), "none.scm", `(include "a")`)
	if !errors.Is(err, core.ErrSourceLoader) {
		t.Fatal("missing loader", err)
	}
}
