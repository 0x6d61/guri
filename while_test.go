package guri

import (
	"errors"
	"testing"
)

func TestWhile(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{`(while false missing)`, nil}, {`(while nil missing)`, nil},
		{`(let [n 0] (while (< n 3) (set! n (inc n))) n)`, int64(3)},
		{`(let [n 0] (while (< n 1) (set! n 1) 42))`, nil},
		{`(let [n 0] (while (do (set! n (inc n)) (< n 3))) n)`, int64(3)},
		{`(let [n 0] (while (if (= n 0) 0 false) (set! n 1)) n)`, int64(1)},
		{`(let [n 0 total 0] (while (< n 4) (set! n (inc n)) (set! total (+ total n))) total)`, int64(10)},
		{`(let [x 0 n 0] (while (< n 3) (set! n (inc n)) (let [x 9] (set! x 10))) x)`, int64(0)},
		{`(def n 0) (while (< n 1) (def answer 42) (set! n 1)) answer`, int64(42)},
		{`(let [n 0 callbacks []] (while (< n 3) (set! callbacks (append callbacks (fn [] n))) (set! n (inc n))) ((nth callbacks 0)))`, int64(3)},
		{`(let [a 0] (while (< a 3) (let [b 0] (while (< b 2) (set! b (inc b)))) (set! a (inc a))) a)`, int64(3)},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := evaluate(t, tt.source); !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestWhileErrors(t *testing.T) {
	expectError(t, `(while)`, ArityError)
	expectError(t, `(while missing 1)`, NameError)
	expectError(t, `(while (/ 1 0) 1)`, ArithmeticError)
	expectError(t, `(while true (nth [] 0))`, IndexError)
	i := NewInterpreter()
	_, err := i.Eval("while.guri", "(while true\n  (/ 1 0))")
	var e *RuntimeError
	if !errors.As(err, &e) || e.Kind != ArithmeticError || e.Position != (Position{"while.guri", 2, 3}) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWhileManyIterations(t *testing.T) {
	if got := evaluate(t, `(let [n 0] (while (< n 100000) (set! n (inc n))) n)`); got != int64(100000) {
		t.Fatal(got)
	}
}
