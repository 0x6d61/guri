package guri

import (
	"errors"
	"runtime"
	"testing"
)

func TestApply(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{`(apply + [1 2 3])`, int64(6)}, {`(apply + '(1 2 3))`, int64(6)},
		{`(apply + 10 [1 2 3])`, int64(16)}, {`(apply + 10 20 '(1 2))`, int64(33)},
		{`(apply + [])`, int64(0)}, {`(apply + '())`, int64(0)},
		{`(apply (fn [] 7) [])`, int64(7)},
		{`(apply (fn [a b] (- a b)) [9 2])`, int64(7)},
		{`(apply (fn [first & rest] rest) 1 '(2 3))`, VectorValue{int64(2), int64(3)}},
		{`(apply (fn [x] x) '(unknown))`, SymbolValue("unknown")},
		{`(apply :x [{:x 9}])`, int64(9)}, {`(apply :x [{} 7])`, int64(7)},
		{`(apply :x {} '(7))`, int64(7)},
		{`(def spread apply) (set! apply +) (spread + [1 2])`, int64(3)},
		{`((fn [call] (call + [1 2])) apply)`, int64(3)},
		{`(apply apply [+ [1 2]])`, int64(3)},
		{`(apply apply [apply [+ [1 2]]])`, int64(3)},
		{`(def a [1]) (apply (fn [v] (assoc! v 0 9)) [a]) a`, VectorValue{int64(9)}},
		{`(def argv [1 2]) (def rest (apply (fn [& rest] rest) argv)) (assoc! rest 0 9) argv`, VectorValue{int64(1), int64(2)}},
		{`(def n 0) (apply (do (set! n 1) (fn [a b] [a b n])) (do (set! n 2) n) (do (set! n 3) [n]))`, VectorValue{int64(2), int64(3), int64(3)}},
		{`(reduce apply + [[1 2]])`, int64(3)},
		{`(:x (update {:x +} :x apply [1 2]))`, int64(3)},
		{`(defn loop [n] (if (= n 0) 7 (apply loop [(- n 1)]))) (def x (apply loop [1000])) (set! x (+ x 1)) x`, int64(8)},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := evaluate(t, tt.source); !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestApplyErrors(t *testing.T) {
	for _, source := range []string{`(apply)`, `(apply +)`, `(apply inc [])`, `(apply (fn [x] x) [1 2])`, `(apply :x [])`} {
		expectError(t, source, ArityError)
	}
	for _, source := range []string{`(apply 1 [])`, `(apply '+ [1 2])`, `(apply + nil)`, `(apply + {})`, `(apply + "12")`, `(apply + b"12")`, `(apply + [true])`, `(apply :x [[]])`} {
		expectError(t, source, TypeError)
	}
	expectError(t, `(apply 1 missing)`, NameError)
	expectError(t, `(apply error ["stop"])`, UserError)
	i := NewInterpreter()
	_, err := i.Eval("apply.guri", "\n  (apply inc [])")
	var e *RuntimeError
	if !errors.As(err, &e) || e.Kind != ArityError || e.Position != (Position{"apply.guri", 2, 3}) {
		t.Fatalf("missing forwarded call location: %v", err)
	}
	if _, err := i.Eval("definition.guri", "(defn fail []\n  (error \"stop\"))"); err != nil {
		t.Fatal(err)
	}
	_, err = i.Eval("apply.guri", `(apply fail [])`)
	if !errors.As(err, &e) || e.Kind != UserError || e.Cause != "stop" || e.Position != (Position{"definition.guri", 2, 3}) {
		t.Fatalf("lost function location: %v", err)
	}
}

func TestApplyTailCallsKeepHostStackBounded(t *testing.T) {
	for _, tt := range []struct{ name, definitions, call string }{
		{"direct", `(defn loop [n] (if (= n 0) (stack-depth) (apply loop [(- n 1)])))`, `loop`},
		{"mutual", `(defn even [n] (if (= n 0) (stack-depth) (apply odd (- n 1) '()))) (defn odd [n] (if (= n 0) (stack-depth) (apply even [(- n 1)])))`, `even`},
		{"alias", `(def spread apply) (defn loop [n] (if (= n 0) (stack-depth) (spread loop (- n 1) '())))`, `loop`},
		{"nested-apply", `(defn loop [n] (if (= n 0) (stack-depth) (apply apply [loop [(- n 1)]])))`, `loop`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			i := NewInterpreter()
			i.native("stack-depth", 0, 0, func([]Value) (Value, error) { pcs := make([]uintptr, 4096); return int64(runtime.Callers(0, pcs)), nil })
			if _, err := i.Eval("apply-tail.guri", tt.definitions); err != nil {
				t.Fatal(err)
			}
			shallow, err := i.Eval("apply-tail.guri", "("+tt.call+" 8)")
			if err != nil {
				t.Fatal(err)
			}
			deep, err := i.Eval("apply-tail.guri", "("+tt.call+" 256)")
			if err != nil {
				t.Fatal(err)
			}
			if deep.(int64) > shallow.(int64)+32 {
				t.Fatalf("apply grows host stack: shallow=%v deep=%v", shallow, deep)
			}
		})
	}
}

func TestDeepTailRecursionThroughApply(t *testing.T) {
	if got := evaluate(t, `(defn sum [n acc] (if (= n 0) acc (apply sum (- n 1) [(+ acc n)]))) (sum 100000 0)`); got != int64(5000050000) {
		t.Fatal(got)
	}
}
