package guri

import (
	"errors"
	"testing"
)

func TestDefn(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{`(defn add [x y] (+ x y)) (add 2 3)`, int64(5)},
		{`((defn identity [x] x) 42)`, int64(42)},
		{`(defn empty []) (empty)`, nil},
		{`(defn collect [x & rest] rest) (collect 1 2 3)`, VectorValue{int64(2), int64(3)}},
		{`(defn collect [& rest] rest) (collect)`, VectorValue{}},
		{`(defn fact [n] (if (= n 0) 1 (* n (fact (- n 1))))) (fact 5)`, int64(120)},
		{`(defn make-adder [n] (fn [x] (+ n x))) (def add10 (make-adder 10)) (let [n 99] (add10 5))`, int64(15)},
		{`(defn make-counter [] (let [n 0] (fn [] (set! n (inc n))))) (def next (make-counter)) (next) (next)`, int64(2)},
		{`(def count 0) (defn bump [] (set! count (inc count)) count) [(bump) (bump)]`, VectorValue{int64(1), int64(2)}},
		{`(defn deferred [] missing) 7`, int64(7)},
		{`(defn f [] 1) (= f f)`, true},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := evaluate(t, tt.source); !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDefnErrors(t *testing.T) {
	for _, source := range []string{`(defn)`, `(defn f)`, `(defn f [x] x) (f)`, `(defn f [] nil) (f 1)`} {
		expectError(t, source, ArityError)
	}
	for _, source := range []string{
		`(defn 1 [] nil)`, `(defn f x x)`, `(defn f [x x] x)`,
		`(defn f [x & x] x)`, `(defn f [&] nil)`, `(defn f [& rest x] nil)`,
		`(defn f [1] nil)`, `(defn f [] 1) (defn f [] 2)`,
		`(def f 1) (defn f [] 2)`, `(defn f [] 1) (def f 2)`,
		`(let [] (defn f [] 1))`, `((fn [] (defn f [] 1)))`,
	} {
		expectError(t, source, DefinitionError)
	}
}

func TestDefnFailedDefinitionDoesNotBindName(t *testing.T) {
	i := NewInterpreter()
	if _, err := i.Eval("bad.guri", `(defn f [x x] x)`); err == nil {
		t.Fatal("expected invalid parameters to fail")
	}
	if got, err := i.Eval("good.guri", `(defn f [] 7) (f)`); err != nil || got != int64(7) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDefnPreservesBodyPosition(t *testing.T) {
	i := NewInterpreter()
	if _, err := i.Eval("definition.guri", "(defn fail []\n  (/ 1 0))"); err != nil {
		t.Fatal(err)
	}
	_, err := i.Eval("call.guri", `(fail)`)
	var e *RuntimeError
	want := Position{Filename: "definition.guri", Line: 2, Column: 3}
	if !errors.As(err, &e) || e.Kind != ArithmeticError || e.Position != want {
		t.Fatalf("got %v, want ArithmeticError at %#v", err, want)
	}
}
