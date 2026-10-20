package guri

import (
	"errors"
	"testing"
)

func evaluate(t *testing.T, source string) Value {
	t.Helper()
	v, err := NewInterpreter().Eval("test.guri", source)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestEvaluate(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{"", nil}, {"1 2 3", int64(3)}, {"()", NewList()},
		{"(def a 2) [a {:x a}]", VectorValue{int64(2), func() *MapValue { m := NewMap(); _ = m.Set(KeywordValue("x"), int64(2)); return m }()}},
		{"'(a 1)", NewList(SymbolValue("a"), int64(1))},
		{"(quote [a])", VectorValue{SymbolValue("a")}},
		{"(let [a 1 b a] b)", int64(1)},
		{"(if false unknown 3)", int64(3)}, {"(if nil unknown)", nil},
		{"(if 0 1 unknown)", int64(1)}, {"(and)", true}, {"(or)", nil},
		{"(and 1 \"\" [])", VectorValue{}}, {"(and false unknown)", false},
		{"(or nil false \"ok\" unknown)", "ok"}, {"(or nil false)", false},
		{"(do)", nil}, {"(do 1 2)", int64(2)},
		{"((fn [a & rest] rest) 1 2 3)", VectorValue{int64(2), int64(3)}},
		{"((fn [& rest] rest))", VectorValue{}},
		{"((fn []))", nil},
		{"(def maker (fn [x] (fn [] x))) (def f (maker 42)) (let [x 9] (f))", int64(42)},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := evaluate(t, tt.source); !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestClosureCellsAndEvaluationOrder(t *testing.T) {
	i := NewInterpreter()
	// Access the functions through the returned value to test sharing before nth exists.
	v, err := i.Eval("cells.guri", "(let [n 0] [(fn [] (set! n 9)) (fn [] n)])")
	if err != nil {
		t.Fatal(err)
	}
	fns := v.(VectorValue)
	if _, err := i.apply(fns[0], nil); err != nil {
		t.Fatal(err)
	}
	if got, err := i.apply(fns[1], nil); err != nil || got != int64(9) {
		t.Fatalf("shared cell: %v %v", got, err)
	}
	source := `(def x 0) ((do (set! x 1) (fn [a b] [a b x])) (do (set! x 2) x) (do (set! x 3) x))`
	if got := evaluate(t, source); !Equal(got, VectorValue{int64(2), int64(3), int64(3)}) {
		t.Fatal(got)
	}
	source = `(def x 0) {(do (set! x 1) :a) x (do (set! x 2) :b) x}`
	m := evaluate(t, source).(*MapValue)
	if m.entries[KeywordValue("a")] != int64(1) || m.entries[KeywordValue("b")] != int64(2) {
		t.Fatal("map evaluation order")
	}
}

func TestEvalErrors(t *testing.T) {
	for _, tt := range []struct {
		source string
		kind   ErrorKind
	}{
		{"unknown", NameError}, {"(def x 1) (def x 2)", DefinitionError},
		{"(set! x 1)", DefinitionError}, {"(def 1 2)", DefinitionError},
		{"(let [] (def x 1))", DefinitionError}, {"(let [x 1 x 2] x)", DefinitionError},
		{"(let [x] x)", DefinitionError}, {"(fn [x x] x)", DefinitionError},
		{"(fn [a & b c] a)", DefinitionError}, {"(fn [&] nil)", DefinitionError},
		{"(fn [1] nil)", DefinitionError}, {"(fn x x)", DefinitionError},
		{"(if true)", ArityError}, {"(quote)", ArityError}, {"(def x)", ArityError},
		{"((fn [x] x))", ArityError}, {"((fn [] nil) 1)", ArityError},
		{"((fn [x & rest] x))", ArityError}, {"(1 2)", TypeError},
		{"{[] 1}", TypeError},
	} {
		t.Run(tt.source, func(t *testing.T) {
			_, err := NewInterpreter().Eval("error.guri", "\n"+tt.source)
			var e *RuntimeError
			if !errors.As(err, &e) || e.Kind != tt.kind || e.Position.Filename != "error.guri" || e.Position.Line != 2 {
				t.Fatalf("got %v, want %s at line 2", err, tt.kind)
			}
		})
	}
}

func TestInterpreterKeepsDefinitions(t *testing.T) {
	i := NewInterpreter()
	if _, err := i.Eval("one.guri", "(def x 1)"); err != nil {
		t.Fatal(err)
	}
	if got, err := i.Eval("two.guri", "(set! x 2) x"); err != nil || got != int64(2) {
		t.Fatalf("%v %v", got, err)
	}
}
