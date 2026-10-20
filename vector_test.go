package guri

import "testing"

func TestVectorBuiltins(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{"(def a [1 2 3]) (def b a) (assoc! b 0 9) a", VectorValue{int64(9), int64(2), int64(3)}},
		{"(def a [1 2 3]) (def b a) (set! b (append b 4)) [(len a) (len b) (nth a 0)]", VectorValue{int64(3), int64(4), int64(1)}},
		{"(def a [1 2 3]) (def b (append a 4)) (assoc! b 0 9) a", VectorValue{int64(1), int64(2), int64(3)}},
		{"(def a [1 2 3]) (def b (slice a 0 1)) (def c (append b 9)) [(len b) (cap b) (nth a 1) (len c)]", VectorValue{int64(1), int64(3), int64(9), int64(2)}},
		{"(def a [1 2 3]) (def b (slice a 0 1)) (slice b 2 3)", VectorValue{int64(3)}},
		{"(def a [1 2 3]) (assoc! (slice a 1 2) 0 8) a", VectorValue{int64(1), int64(8), int64(3)}},
		{"(def a [1 2]) (def b (assoc a 0 7)) [a b]", VectorValue{VectorValue{int64(1), int64(2)}, VectorValue{int64(7), int64(2)}}},
		{"(def a [1 2]) (def b (clone a)) (assoc! b 0 7) a", VectorValue{int64(1), int64(2)}},
		{"(def a [[1]]) (def b (clone a)) (assoc! (nth b 0) 0 9) (nth (nth a 0) 0)", int64(9)},
		{"(cap [1 2 3])", int64(3)}, {"(cap [])", int64(0)},
		{"(len \"日本語\")", int64(9)}, {"(len '(1 2))", int64(2)},
		{"(conj [1] 2 3)", VectorValue{int64(1), int64(2), int64(3)}},
		{"(= append conj)", true}, {"(reduce + 10 [1 2 3])", int64(16)},
		{"(set! conj (fn [x] x)) (append [1] 2)", VectorValue{int64(1), int64(2)}},
		{"(reduce + 10 [])", int64(10)},
		{"(def a [1]) ((fn [v] (assoc! v 0 4)) a) a", VectorValue{int64(4)}},
		{"(def a [1]) ((fn [v] (set! v (append v 2))) a) (len a)", int64(1)},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := evaluate(t, tt.source); !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestVectorErrors(t *testing.T) {
	for _, source := range []string{"(nth [] 0)", "(nth [1] -1)", "(assoc! [1] 1 2)", "(slice [1] 0 2)", "(slice [1] 1 0)", "(slice [1] -1 0)", "(def a [1 2]) (nth (slice a 0 1) 1)"} {
		expectError(t, source, IndexError)
	}
	for _, source := range []string{"(nth [1] 0.0)", "(nth '(1) 0)", "(append 1 2)", "(cap {})", "(clone 1)", "(len 1)", "(assoc [1] :x 1)", "(reduce + 0 '(1))", "(reduce 1 0 [1])", "(slice [1] 0.0 1)"} {
		expectError(t, source, TypeError)
	}
	for _, source := range []string{"(append)", "(nth [1])", "(slice [1] 0)", "(reduce + [1])", "(assoc [1] 0)"} {
		expectError(t, source, ArityError)
	}
}
