package core_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestRecordDefinitionsAndFields(t *testing.T) {
	for _, src := range []string{
		`(define-record-type pare (kons x y) pare? (x kar set-kar!) (y kdr)) (define p (kons 1 2)) (set-kar! p 3) (and (pare? p) (= (kar p) 3) (= (kdr p) 2) (not (pare? (cons 1 2))))`,
		`(define-record-type item (make y x) item? (x item-x) (y item-y)) (define p (make 2 1)) (and (= (item-x p) 1) (= (item-y p) 2))`,
		`(define-record-type item (make x) item? (x item-x) (y item-y set-y!)) (define p (make 1)) (set-y! p 2) (= (item-y p) 2)`,
		`(define-record-type empty (make-empty) empty?) (and (empty? (make-empty)) (not (eqv? (make-empty) (make-empty))))`,
		`((lambda () (define-record-type local (make x) local? (x get)) (= (get (make 7)) 7)))`,
		`(define (factory) (define-record-type item (make) item?) (list make item?)) (define a (factory)) (define b (factory)) (and ((cadr a) ((car a))) (not ((cadr a) ((car b)))))`,
		`(define-record-type item (make x) item? (x get)) (define p (list 1)) (eq? (get (make p)) p)`,
		`(define-record-type item (make x) item? (x get)) (guard (e (else #t)) (get 1) #f)`,
		`(define-record-type item (make x) item? (x get set-x!)) (guard (e (else #t)) (set-x! (cons 1 2) 3) #f)`,
		`(define-syntax make-type (syntax-rules () ((_ name ctor pred field accessor) (define-record-type name (ctor field) pred (field accessor))))) (make-type item make item? x get) (= (get (make 9)) 9)`,
	} {
		t.Run(src, func(t *testing.T) {
			if v := eval(t, engine(), src); v != scheme.True {
				t.Fatal(v)
			}
		})
	}
}
func TestRecordSyntaxErrors(t *testing.T) {
	for _, src := range []string{
		`(define-record-type item)`,
		`(if #t (define-record-type item (make) item?) #f)`,
		`((lambda () 1 (define-record-type item (make) item?)))`,
		`(define-record-type define-record-type (make) pred)`, `(define-record-type item make item?)`,
		`(define-record-type item (make missing) item? (x get))`,
		`(define-record-type item (make x x) item? (x get))`,
		`(define-record-type item (make) item? (x get) (x get2))`,
		`(define-record-type item (make) item? (x get set-x!) (y get))`,
		`(define-record-type item (make) item? (x get set-x!) (y get2 set-x!))`,
		`(define-record-type item (make) item? (x))`,
		`(define-record-type item (make) item? (x get set-x! extra))`,
		`(define-record-type item (make) item? (x get)) (set-x! (make) 1)`,
	} {
		_, err := core.NewCore().Eval(context.Background(), "record.scm", src)
		if err == nil {
			t.Fatal("accepted invalid record definition", src)
		}
		if src != `(define-record-type item (make) item? (x get)) (set-x! (make) 1)` && !errors.Is(err, core.ErrSyntax) {
			t.Fatal(src, err)
		}
	}
}

func TestRecordTypeIdentityAndHygiene(t *testing.T) {
	for _, src := range []string{
		`(define-record-type a (make-a x) a? (x a-x set-a-x!)) (define-record-type b (make-b x) b? (x b-x)) (guard (e (else #t)) (a-x (make-b 1)) #f)`,
		`(define-record-type a (make-a x) a? (x a-x set-a-x!)) (define-record-type b (make-b x) b? (x b-x)) (guard (e (else #t)) (set-a-x! (make-b 1) 2) #f)`,
		`(define-syntax mixed (syntax-rules () ((_ y ctor pred first second) (define-record-type hidden (ctor y x) pred (y first) (x second))))) (mixed x make item? first second) (define item (make 1 2)) (and (= (first item) 1) (= (second item) 2))`,
		`((lambda (define syntax-rules) (define-record-type item (make x) item? (x get)) (= (get (make 3)) 3)) 0 0)`,
		`(define-record-type item (make) item?) (define old? item?) (define old (make)) (define-record-type item (make) item?) (and (old? old) (not (item? old)))`,
	} {
		t.Run(src, func(t *testing.T) {
			if v := eval(t, engine(), src); v != scheme.True {
				t.Fatal(v)
			}
		})
	}
}

func TestRecordWikiExamples(t *testing.T) {
	data, err := os.ReadFile("../../docs/wiki/Records.md")
	if err != nil {
		t.Fatal(err)
	}
	var e *core.Engine
	active := false
	source := ""
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "```scheme" {
			active = true
			e = engine()
			source = ""
			continue
		}
		if line == "```" {
			active = false
			continue
		}
		if !active {
			continue
		}
		code, want, has := strings.Cut(line, "; =>")
		source += code + "\n"
		if !has {
			continue
		}
		got := eval(t, e, source)
		expected := eval(t, e, "(quote "+strings.TrimSpace(want)+")")
		if !equality.Equal(got, expected) {
			t.Fatal(source, got, want)
		}
		count++
		source = ""
	}
	if count != 10 {
		t.Fatal("update example inventory", count)
	}
}
