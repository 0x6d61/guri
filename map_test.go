package guri

import "testing"

func TestMapBuiltins(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{`(def a {:age 25}) (def alias a) (assoc! alias :age 26) (:age a)`, int64(26)},
		{`(def a {:count 0}) (def b (assoc a :count 1)) [(:count a) (:count b)]`, VectorValue{int64(0), int64(1)}},
		{`(get {} :x)`, nil}, {`(get {} :x 9)`, int64(9)}, {`(:x {} 9)`, int64(9)},
		{`(get {:x nil} :x 9)`, nil}, {`(:x {:x nil} 9)`, nil},
		{`(def a {:x 1 :y 2}) (def b (dissoc a :x)) [(len a) (len b) (:x b)]`, VectorValue{int64(2), int64(1), nil}},
		{`(def a {:x 1 :y 2}) (dissoc! a :x :y :missing) (len a)`, int64(0)},
		{`(def a {:x 1}) (def b (update a :x inc)) [(:x a) (:x b)]`, VectorValue{int64(1), int64(2)}},
		{`(def a {:x 1}) (update! a :x + 2 3) (:x a)`, int64(6)},
		{`(:x (update {} :x (fn [x] (if x x 3))))`, int64(3)},
		{`(def a {:x 1}) (def b (clone a)) (assoc! b :x 2) (:x a)`, int64(1)},
		{`(def a {:x [1]}) (def b (clone a)) (assoc! (:x b) 0 9) (nth (:x a) 0)`, int64(9)},
		{`(def a {:x [1]}) (def b (assoc a :y 2)) (assoc! (:x b) 0 9) (nth (:x a) 0)`, int64(9)},
		{`(get {nil 7} nil)`, int64(7)}, {`(get {'name 3} 'name)`, int64(3)},
		{`(len {1 :int 1.0 :float})`, int64(2)},
		{`(= {:x 1 :y 2} {:y 2 :x 1.0})`, true},
		{`(def f (fn [] nil)) [(= f f) (= f (fn [] nil))]`, VectorValue{true, false}},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := evaluate(t, tt.source); !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMapErrors(t *testing.T) {
	for _, source := range []string{`(get {} [])`, `(assoc! {} [] 1)`, `(dissoc {} [])`, `(update {} [] inc)`, `(:x [])`, `(get [] :x)`, `(dissoc [] :x)`, `(update {} :x 1)`, `(clone nil)`} {
		expectError(t, source, TypeError)
	}
	for _, source := range []string{`(:x)`, `(:x {} 1 2)`, `(get {})`, `(update {} :x)`, `(dissoc {})`} {
		expectError(t, source, ArityError)
	}
	// A failed callback must not replace the key's previous value.
	i := NewInterpreter()
	if _, err := i.Eval("map.guri", `(def a {:x 1}) (update! a :x (fn [x] (/ x 0)))`); err == nil {
		t.Fatal("expected callback failure")
	}
	if value, err := i.Eval("map.guri", `(:x a)`); err != nil || value != int64(1) {
		t.Fatalf("%v %v", value, err)
	}
}
