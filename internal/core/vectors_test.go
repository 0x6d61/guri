package core_test

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestStandardVectorsAndBytevectors(t *testing.T) {
	for _, src := range []string{
		`(and (vector? (vector)) (not (vector? '())) (bytevector? (bytevector)) (not (bytevector? (vector))))`,
		`(equal? (vector 1 'a #t) '#(1 a #t))`, `(equal? (make-vector 3 1) '#(1 1 1))`,
		`(= (vector-length (make-vector 3)) 3)`, `(= (vector-ref '#(1 2 3) 1) 2)`,
		`(define v (vector 1 2 3)) (define alias v) (vector-set! alias 1 9) (equal? v '#(1 9 3))`,
		`(define p (list 1)) (define a (vector p 2)) (define b (vector-copy a)) (vector-set! b 1 9) (set-car! p 7) (and (= (vector-ref a 1) 2) (eq? (vector-ref a 0) (vector-ref b 0)))`,
		`(equal? (vector-copy '#(1 2 3) 1 3) '#(2 3))`, `(equal? (vector-append '#(1) '#(2 3)) '#(1 2 3))`, `(equal? (vector-append) '#())`,
		`(equal? (vector->list '#(1 2 3) 1 3) '(2 3))`, `(equal? (list->vector '(1 2)) '#(1 2))`,
		`(string=? (vector->string (string->vector "Aλ😀" 1 3)) "λ😀")`,
		`(string=? (vector->string (vector 1 #\λ #\😀) 1 2) "λ")`,
		`(define v (vector 0 1 2 3 4)) (vector-copy! v 1 v 0 4) (equal? v '#(0 0 1 2 3))`,
		`(define v (vector 0 1 2 3 4)) (vector-copy! v 0 v 1 5) (equal? v '#(1 2 3 4 4))`,
		`(define v (vector 1 2 3)) (vector-fill! v 'x 1 3) (equal? v '#(1 x x))`,
		`(equal? (bytevector 0 127 255) #u8(0 127 255))`, `(equal? (make-bytevector 3 255) #u8(255 255 255))`,
		`(= (bytevector-length (make-bytevector 2)) 2)`, `(= (bytevector-u8-ref #u8(0 255) 1) 255)`,
		`(define b (bytevector 1 2)) (define alias b) (bytevector-u8-set! alias 0 255) (equal? b #u8(255 2))`,
		`(define a (bytevector 1 2)) (define b (bytevector-copy a)) (bytevector-u8-set! b 0 9) (equal? a #u8(1 2))`,
		`(equal? (bytevector-copy #u8(1 2 3) 1 3) #u8(2 3))`, `(equal? (bytevector-append #u8(1 2) #u8(3)) #u8(1 2 3))`,
		`(define b (bytevector 0 1 2 3 4)) (bytevector-copy! b 1 b 0 4) (equal? b #u8(0 0 1 2 3))`,
		`(define b (bytevector 0 1 2 3 4)) (bytevector-copy! b 0 b 1 5) (equal? b #u8(1 2 3 4 4))`,
		`(equal? (string->utf8 "Aλ😀" 1 2) #u8(206 187))`, `(string=? (utf8->string #u8(65 206 187 66) 1 3) "λ")`,
		`(string=? (utf8->string (string->utf8 "\x0;Aλ😀")) "\x0;Aλ😀")`,
		`(equal? (vector-map + '#(1 2 3) '#(4 5)) '#(5 7))`,
		`(define total 0) (vector-for-each (lambda (a b) (set! total (+ total a b))) '#(1 2 3) '#(4 5)) (= total 12)`,
	} {
		t.Run(src, func(t *testing.T) {
			if v := eval(t, engine(), src); v != scheme.True {
				t.Fatal(v)
			}
		})
	}
}
func TestVectorErrorsAndAtomicMutation(t *testing.T) {
	for _, src := range []string{
		`(vector-set! '#(1) 0 2)`, `(vector-fill! '#() 1)`, `(vector-copy! '#() 0 '#() 0 0)`,
		`(bytevector-u8-set! #u8(1) 0 2)`, `(bytevector-copy! #u8() 0 #u8())`,
		`(vector-ref '#(1) 1)`, `(vector-ref '#(1) 0.0)`, `(vector-copy '#(1) 1 0)`, `(vector-copy '#(1) -1)`,
		`(vector->string '#(1))`, `(list->vector '(1 . 2))`, `(bytevector 256)`, `(bytevector -1)`, `(bytevector 1.0)`,
		`(make-bytevector 0 256)`, `(bytevector-u8-set! (bytevector 1) 0 256)`,
		`(utf8->string #u8(206))`, `(utf8->string #u8(192 128))`, `(utf8->string #u8(237 160 128))`, `(utf8->string #u8(244 144 128 128))`,
		`(vector-map (lambda (x) (values 1 2)) '#(1))`,
	} {
		t.Run(src, func(t *testing.T) {
			if v := eval(t, engine(), `(guard (e (else #t)) `+src+` #f)`); v != scheme.True {
				t.Fatal("accepted invalid input", v)
			}
		})
	}
	for _, src := range []string{
		`(define v (vector 1 2)) (guard (e (else #t)) (vector-copy! v 1 '#(8 9))) (equal? v '#(1 2))`,
		`(define b (bytevector 1 2)) (guard (e (else #t)) (bytevector-copy! b 1 #u8(8 9))) (equal? b #u8(1 2))`,
	} {
		if v := eval(t, engine(), src); v != scheme.True {
			t.Fatal("partial invalid mutation", v)
		}
	}
}

func TestVectorCallbackContinuationsAndSharing(t *testing.T) {
	e := engine()
	first := eval(t, e, `(define saved #f) (define result (vector-map (lambda (x) (if (= x 2) (call/cc (lambda (k) (set! saved k) x)) x)) '#(1 2 3))) result`).(*scheme.Vector)
	if err := first.Set(0, scheme.NewInteger(9)); err != nil {
		t.Fatal(err)
	}
	if got := repr(eval(t, e, `(saved 20)`)); got != "#(1 20 3)" {
		t.Fatal(got)
	}
	if got := repr(eval(t, e, `(saved 30)`)); got != "#(1 30 3)" {
		t.Fatal(got)
	}
	if got := repr(first); got != "#(9 2 3)" {
		t.Fatal("earlier result changed", got)
	}
	e = engine()
	if v := number(eval(t, e, `(define saved #f) (define n 0) (vector-for-each (lambda (x) (if (= x 2) (call/cc (lambda (k) (set! saved k) #f)) #f) (set! n (+ n 1))) '#(1 2 3)) n`)); v != 3 {
		t.Fatal(v)
	}
	if v := number(eval(t, e, `(saved #f)`)); v != 5 {
		t.Fatal(v)
	}
	if v := number(eval(t, e, `(saved #f)`)); v != 7 {
		t.Fatal(v)
	}
	for _, src := range []string{
		`(define p (make-parameter 1)) (equal? (parameterize ((p 9)) (vector-map (lambda (x) (p)) '#(1 2))) '#(9 9))`,
		`(begin (vector-for-each (lambda (x) (values 1 2)) '#(1 2)) #t)`,
		`(= (vector-length (vector-map (lambda (x) x) (make-vector 20000 1))) 20000)`,
		`(equal? (vector-map + '#(1 2) '#()) '#())`,
		`(define p (list 1)) (define v (vector-map (lambda (x) p) '#(0 0))) (and (eq? (vector-ref v 0) p) (eq? (vector-ref v 0) (vector-ref v 1)))`,
		`(string=? (utf8->string #u8(255 65 255) 1 2) "A")`,
		`(string=? (utf8->string #u8()) "")`, `(equal? (string->utf8 "") #u8())`,
		`(define bytes (string->utf8 "A")) (bytevector-u8-set! bytes 0 66) (string=? (utf8->string bytes) "B")`,
	} {
		e := engine()
		if v := eval(t, e, src); v != scheme.True {
			t.Fatal(src, v)
		}
		if e.Stats().PeakFrames > 8 {
			t.Fatal("vector iteration retained callback frames", e.Stats())
		}
	}
}

func TestVectorWikiExamplesAndBindings(t *testing.T) {
	data, err := os.ReadFile("../../docs/wiki/Vectors-Bytevectors.md")
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
			t.Fatalf("%s: got %v want %s", source, got, want)
		}
		count++
		source = ""
	}
	if count != strings.Count(string(data), "; =>") {
		t.Fatal("example not executed", count)
	}
	t.Logf("verified %d examples", count)
	file, err := os.Open("../../docs/r7rs/exports.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	e = engine()
	for _, row := range rows[1:] {
		if row[3] != "57" || seen[row[1]] {
			continue
		}
		seen[row[1]] = true
		if _, ok := eval(t, e, row[1]).(*scheme.Procedure); !ok {
			t.Fatal("not installed", row[1])
		}
	}
	if len(seen) != 27 {
		t.Fatal("update function inventory", len(seen))
	}
}
