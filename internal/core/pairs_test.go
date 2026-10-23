package core_test

import (
	"context"
	"encoding/csv"
	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
	"os"
	"strings"
	"testing"
)

func TestStandardPairsListsAndSymbols(t *testing.T) {
	for source, want := range map[string]string{
		`(cons 1 2)`: `(1 . 2)`, `(list 1 2 3)`: `(1 2 3)`, `(make-list 3 'a)`: `(a a a)`,
		`(cadddr '(0 1 2 3))`: `3`, `(caaadr '((0) ((42))))`: `42`,
		`(length '(a b c))`: `3`, `(reverse '(1 2 3))`: `(3 2 1)`, `(append '(1 2) '(3 . 4))`: `(1 2 3 . 4)`,
		`(list-copy '(1 2 . 3))`: `(1 2 . 3)`, `(list-copy 42)`: `42`, `(list-tail '(1 2 . 3) 2)`: `3`,
		`(define x (list 1 2)) (list-set! x 1 9) x`:                                                    `(1 9)`,
		`(define tail (list 3)) (define x (append '(1 2) tail)) (set-car! tail 9) x`:                   `(1 2 9)`,
		`(define x (list 1 2)) (define y (list-copy x)) (set-car! y 9) x`:                              `(1 2)`,
		`(list (pair? '(1 . 2)) (null? '()) (list? '(1 . 2)) (list? '()) (not '()) (boolean=? #t #t))`: `(#t #t #f #t #f #t)`,
		`(list (eq? 'a 'a) (eqv? 1 1.0) (equal? '(1) '(1)) (equal? '#(1 2) '#(1 2)))`:                  `(#t #f #t #t)`,
		`(list (symbol? 'a) (symbol=? 'a (string->symbol "a")) (symbol->string '|雪 山|))`:               `(#t #t "雪 山")`,
		`(memv 2 '(1 2 3))`: `(2 3)`, `(member '(2) '((1) (2) (3)))`: `((2) (3))`,
		`(assoc 2.0 '((1 a) (2 b)) =)`: `(2 b)`, `(member 2.0 '(1 2 3) =)`: `(2 3)`,
		`(assq 'b '((a . 1) (b . 2)))`: `(b . 2)`, `(assv 2 '((1 a) (2 b)))`: `(2 b)`,
		`(map + '(1 2 3) '(4 5))`: `(5 7)`, `(define n 0) (for-each (lambda (x) (set! n (+ n x))) '(1 2 3)) n`: `6`,
		`(guard (e (else #t)) (set-car! '(1) 2) #f)`: `#t`, `(guard (e (else #t)) (length '(1 . 2)) #f)`: `#t`,
		`(guard (e (else #t)) (list-ref '(1) 2) #f)`: `#t`,
	} {
		t.Run(source, func(t *testing.T) {
			if got := repr(eval(t, engine(), source)); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
}

func TestCircularEqualityAndLongLists(t *testing.T) {
	e := core.NewCore()
	for _, src := range []string{
		`(define a (list 1 2)) (set-cdr! (cdr a) a) (define b (list 1 2 1 2)) (set-cdr! (cdddr b) b) (equal? a b)`,
		`(if (list? a) #f #t)`, `(= (list-ref a 10001) 2)`,
		`(guard (e (else #t)) (list-copy a) #f)`,
		`(guard (e (else #t)) (length a) #f)`,
		`(set-car! b 9) (if (equal? a b) #f #t)`,
		`(let loop ((n 100000) (xs '())) (if (= n 0) (= (length (reverse xs)) 100000) (loop (- n 1) (cons n xs))))`,
	} {
		if v := eval(t, e, src); v != scheme.True {
			t.Fatal(src, v)
		}
	}
	// Cycles can also pass through vectors, not only cdr chains.
	x, y := scheme.NewVector(scheme.False), scheme.NewVector(scheme.False)
	if err := x.Set(0, x); err != nil {
		t.Fatal(err)
	}
	if err := y.Set(0, y); err != nil {
		t.Fatal(err)
	}
	e.Bind("vx", x)
	e.Bind("vy", y)
	if v := eval(t, e, `(equal? vx vy)`); v != scheme.True {
		t.Fatal(v)
	}
	if _, err := e.Eval(context.Background(), "cycle.scm", `(append a '())`); err == nil {
		t.Fatal("circular prefix accepted")
	}
}

func TestListCallbackContinuations(t *testing.T) {
	e := core.NewCore()
	src := `(define saved #f) (define result (map (lambda (x) (if (= x 2) (call/cc (lambda (k) (set! saved k) x)) x)) '(1 2 3))) result`
	if got := repr(eval(t, e, src)); got != "(1 2 3)" {
		t.Fatal(got)
	}
	for _, tc := range []struct{ src, want string }{{`(saved 20)`, `(1 20 3)`}, {`(saved 30)`, `(1 30 3)`}} {
		if got := repr(eval(t, e, tc.src)); got != tc.want {
			t.Fatal(got)
		}
	}
	e = core.NewCore()
	src = `(define saved #f) (define result (member 2 '(1 2 3) (lambda (a b) (if (= b 2) (call/cc (lambda (k) (set! saved k) #t)) #f)))) result`
	if got := repr(eval(t, e, src)); got != "(2 3)" {
		t.Fatal(got)
	}
	if got := repr(eval(t, e, `(saved #f)`)); got != "#f" {
		t.Fatal(got)
	}
	if got := repr(eval(t, e, `(saved #t)`)); got != "(2 3)" {
		t.Fatal(got)
	}
}

func TestListCallbackResultsRemainIndependent(t *testing.T) {
	e := core.NewCore()
	first := eval(t, e, `(define saved #f) (define output (map (lambda (x) (if (= x 2) (call/cc (lambda (k) (set! saved k) x)) x)) '(1 2 3))) output`)
	if err := first.(*scheme.Pair).SetCar(scheme.NewInteger(9)); err != nil {
		t.Fatal(err)
	}
	if got := repr(eval(t, e, `(saved 20)`)); got != "(1 20 3)" {
		t.Fatal(got)
	}
	if got := repr(first); got != "(9 2 3)" {
		t.Fatal("earlier result mutated", got)
	}
	e = core.NewCore()
	src := `(define saved #f) (define n 0) (for-each (lambda (x) (set! n (+ n (if (= x 2) (call/cc (lambda (k) (set! saved k) x)) x)))) '(1 2 3)) n`
	// The pending + captured n=1 before call/cc. Re-entering the continuation
	// repeats that pending assignment and the final callback.
	if v := number(eval(t, e, src)); v != 6 {
		t.Fatal(v)
	}
	if v := number(eval(t, e, `(saved 20)`)); v != 24 {
		t.Fatal(v)
	}
	if v := number(eval(t, e, `(saved 30)`)); v != 34 {
		t.Fatal(v)
	}
	for _, src := range []string{
		`(guard (e (else #t)) (map (lambda (x) (values 1 2)) '(1)) #f)`,
		`(begin (for-each (lambda (x) (values 1 2)) '(1 2)) #t)`,
		`(define cycle (list 10)) (set-cdr! cycle cycle) (equal? (map + cycle '(1 2 3)) '(11 12 13))`,
		`(= (list-ref cycle 1000000000000000000000000000000000000) 10)`,
	} {
		if v := eval(t, e, src); v != scheme.True {
			t.Fatal(src, v)
		}
	}
}

func TestSymbolsAndEquivalenceChoices(t *testing.T) {
	e := core.NewCore()
	s := eval(t, e, `(symbol->string 'hello)`).(*scheme.String)
	lower, err := scheme.NewCharacter('x')
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set(0, lower); err != scheme.ErrImmutable {
		t.Fatal("mutable symbol name", err)
	}
	mutable, err := scheme.NewString("before")
	if err != nil {
		t.Fatal(err)
	}
	e.Bind("name", mutable)
	symbol := eval(t, e, `(string->symbol name)`).(*scheme.Symbol)
	upper, err := scheme.NewCharacter('X')
	if err != nil {
		t.Fatal(err)
	}
	if err := mutable.Set(0, upper); err != nil {
		t.Fatal(err)
	}
	if symbol.Name() != "before" {
		t.Fatal("symbol aliases mutable string")
	}
	for _, src := range []string{
		`(let ((x (cons 1 2))) (and (eq? x x) (eqv? x x)))`,
		`(and (not (eqv? 0.0 -0.0)) (not (equal? 1 1.0)) (not (eqv? (cons 1 2) (cons 1 2))))`,
		`(let ((f (lambda () 1))) (eq? f f))`,
		`(guard (e (else #t)) (boolean=? #t 1) #f)`,
		`(guard (e (else #t)) (symbol=? 1 'a) #f)`,
		`(guard (e (else #t)) (list-ref '(1) 1.0) #f)`,
	} {
		if v := eval(t, e, src); v != scheme.True {
			t.Fatal(src, v)
		}
	}
}

func TestListTraversalBoundariesAndScale(t *testing.T) {
	for _, src := range []string{
		`(define x (list 1)) (set-cdr! x x) (guard (e (else #t)) (map + x) #f)`,
		`(define x (list 1 2)) (set-cdr! (cdr x) x) (guard (e (else #t)) (member 3 x) #f)`,
		`(eq? (append) '())`, `(eq? (list-copy #f) #f)`,
		`(let ((tail (list 1))) (eq? tail (append tail)))`,
		`(= (length (map (lambda (x) x) (make-list 20000 1))) 20000)`,
		`(not (member 2 (make-list 20000 1)))`,
		`(not (member 2 (make-list 20000 1) =))`,
	} {
		e := core.NewCore()
		if v := eval(t, e, src); v != scheme.True {
			t.Fatal(src, v)
		}
		if e.Stats().PeakFrames > 8 {
			t.Fatal("list operation retained callback frames", e.Stats())
		}
	}
}

func TestListWikiExamples(t *testing.T) {
	data, err := os.ReadFile("../../docs/wiki/Pairs-Lists-Symbols.md")
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
			e = core.NewCore()
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
		got := repr(eval(t, e, source))
		if got != strings.TrimSpace(want) {
			t.Fatalf("%s: got %s want %s", source, got, want)
		}
		count++
		source = ""
	}
	if count != 18 {
		t.Fatalf("checked %d examples", count)
	}
}

func TestSearchShortCircuitAndSharedResults(t *testing.T) {
	for _, source := range []string{
		`(define xs (list 1 2 3)) (eq? (memv 2 xs) (cdr xs))`,
		`(define entry (cons 'a 1)) (eq? (assq 'a (list entry)) entry)`,
		`(define n 0) (define xs '(1 2 3)) (define result (member 2 xs (lambda (a b) (set! n (+ n 1)) (if (= a b) 0 #f)))) (and (= n 2) (eq? result (cdr xs)))`,
	} {
		if v := eval(t, engine(), source); v != scheme.True {
			t.Fatal(source, v)
		}
	}
	e := engine()
	source := `(define saved #f) (define result (assoc 2 '((1 a) (2 b) (3 c)) (lambda (a b) (if (= b 2) (call/cc (lambda (k) (set! saved k) #t)) #f)))) result`
	if got := repr(eval(t, e, source)); got != "(2 b)" {
		t.Fatal(got)
	}
	if got := repr(eval(t, e, `(saved #f)`)); got != "#f" {
		t.Fatal(got)
	}
	if got := repr(eval(t, e, `(saved #t)`)); got != "(2 b)" {
		t.Fatal(got)
	}
}

func TestAllListLibraryBindingsInstalled(t *testing.T) {
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
	e := core.NewCore()
	for _, row := range rows[1:] {
		if row[3] != "55" || seen[row[1]] {
			continue
		}
		seen[row[1]] = true
		if _, ok := eval(t, e, row[1]).(*scheme.Procedure); !ok {
			t.Fatal("not installed", row[1])
		}
	}
	if len(seen) != 63 {
		t.Fatal("update inventory expectation", len(seen))
	}
}
