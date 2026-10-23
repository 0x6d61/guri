package guri

import (
	"errors"
	"runtime"
	"testing"
)

func TestTailCallsKeepHostStackBounded(t *testing.T) {
	for _, tt := range []struct{ name, definitions, call string }{
		{"self", `(defn loop [n] (if (= n 0) (stack-depth) (loop (- n 1))))`, `loop`},
		{"mutual", `(defn even [n] (if (= n 0) (stack-depth) (odd (- n 1)))) (defn odd [n] (if (= n 0) (stack-depth) (even (- n 1))))`, `even`},
		{"do", `(defn loop [n] (if (= n 0) (stack-depth) (do 1 (loop (- n 1)))))`, `loop`},
		{"let", `(defn loop [n] (if (= n 0) (stack-depth) (let [next (- n 1)] (loop next))))`, `loop`},
		{"cond", `(defn loop [n] (cond (= n 0) (stack-depth) true (loop (- n 1))))`, `loop`},
		{"and", `(defn loop [n] (if (= n 0) (stack-depth) (and true (loop (- n 1)))))`, `loop`},
		{"or", `(defn loop [n] (if (= n 0) (stack-depth) (or false (loop (- n 1)))))`, `loop`},
		{"closure", `(def next nil) (set! next (let [step 1] (fn [n] (if (= n 0) (stack-depth) (next (- n step))))))`, `next`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			i := NewInterpreter()
			i.native("stack-depth", 0, 0, func([]Value) (Value, error) { pcs := make([]uintptr, 4096); return int64(runtime.Callers(0, pcs)), nil })
			if _, err := i.Eval("tail.guri", tt.definitions); err != nil {
				t.Fatal(err)
			}
			shallow, err := i.Eval("tail.guri", "("+tt.call+" 8)")
			if err != nil {
				t.Fatal(err)
			}
			deep, err := i.Eval("tail.guri", "("+tt.call+" 256)")
			if err != nil {
				t.Fatal(err)
			}
			if deep.(int64) > shallow.(int64)+32 {
				t.Fatalf("host stack grows with tail calls: shallow=%v deep=%v", shallow, deep)
			}
		})
	}
}

func TestDeepTailRecursion(t *testing.T) {
	if got := evaluate(t, `(defn sum [n acc] (if (= n 0) acc (sum (- n 1) (+ acc n)))) (sum 100000 0)`); got != int64(5000050000) {
		t.Fatal(got)
	}
}

func TestTailCallsPreserveEvaluation(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{`(defn count [n] (if (= n 0) 0 (count (- n 1)))) (+ 1 (count 1000))`, int64(1)},
		{`(defn f [n] (if (= n 0) 2 (f (- n 1)))) (do (f 1000) 42)`, int64(42)},
		{`(defn f [n] (if (= n 0) false (f (- n 1)))) (and (f 1000) missing)`, false},
		{`(defn f [n] (if (= n 0) true (f (- n 1)))) (or (f 1000) missing)`, true},
		{`(defn f [n] (if (= n 0) 7 (f (- n 1)))) (let [x (f 1000)] [x {:x x}])`, VectorValue{int64(7), func() *MapValue { m := NewMap(); _ = m.Set(KeywordValue("x"), int64(7)); return m }()}},
		{`(defn f [n] (if (= n 0) 7 (f (- n 1)))) (def x (f 1000)) (set! x (f 1000)) x`, int64(7)},
		{`(defn f [n] (if (= n 0) 2 (f (- n 1)))) (let [n 0] (while (< n 3) (f 100) (set! n (inc n))) n)`, int64(3)},
		{`(defn f [n] (if (= n 0) 1 (f (- n 1)))) (reduce (fn [a x] (+ a (f 100))) 0 [1 2 3])`, int64(3)},
		{`(defn f [n] (if (= n 0) 7 (f (- n 1)))) (def m {:x 0}) (update! m :x (fn [old] (f 1000))) (:x m)`, int64(7)},
		{`(defn collect [n & rest] (if (= n 0) rest (collect (- n 1) 1 2))) (collect 1000)`, VectorValue{int64(1), int64(2)}},
		{`(defn build [n callbacks] (if (= n 0) callbacks (build (- n 1) (append callbacks (fn [] n))))) (def callbacks (build 3 [])) [((nth callbacks 0)) ((nth callbacks 1)) ((nth callbacks 2))]`, VectorValue{int64(3), int64(2), int64(1)}},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := evaluate(t, tt.source); !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTailCallErrors(t *testing.T) {
	i := NewInterpreter()
	if _, err := i.Eval("definition.guri", "(defn fail [n]\n  (if (= n 0) (error \"stop\") (fail (- n 1))))"); err != nil {
		t.Fatal(err)
	}
	_, err := i.Eval("call.guri", `(fail 1000)`)
	var e *RuntimeError
	if !errors.As(err, &e) || e.Kind != UserError || e.Cause != "stop" || e.Position != (Position{"definition.guri", 2, 15}) {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, source := range []string{`(defn f [] (inc)) (f)`, `(defn f [] ((fn [x] x))) (f)`} {
		expectError(t, source, ArityError)
	}
	expectError(t, `(defn f [] (1 2)) (f)`, TypeError)
}
