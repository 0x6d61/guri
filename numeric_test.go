package guri

import (
	"errors"
	"testing"
)

func expectError(t *testing.T, source string, kind ErrorKind) {
	t.Helper()
	_, err := NewInterpreter().Eval("test.guri", source)
	var e *RuntimeError
	if !errors.As(err, &e) || e.Kind != kind || e.Position.Line == 0 {
		t.Fatalf("%s: got %v, want %s with source position", source, err, kind)
	}
}

func TestNumericBuiltins(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{"(+)", int64(0)}, {"(*)", int64(1)}, {"(+ 1 2 3)", int64(6)},
		{"(- 9 2 3)", int64(4)}, {"(- 3)", int64(-3)}, {"(* -2 3)", int64(-6)},
		{"(+ 1 2.5)", 3.5}, {"(/ 12 2 3)", float64(2)}, {"(/ 1 2)", 0.5},
		{"(inc 1)", int64(2)}, {"(inc 1.5)", 2.5},
		{"(= 1 1.0)", true}, {"(!= 1 1.0)", false}, {"(!= 1 2)", true},
		{"(= 1 1.5)", false}, {"(= 9007199254740993 9007199254740992.0)", false},
		{"(< 9007199254740992.0 9007199254740993)", true},
		{"(< 9223372036854775807 9223372036854775808.0)", true},
		{"(>= -9223372036854775808 -9223372036854775808.0)", true},
		{"(< 1 2 3)", true}, {"(<= 1 1 2)", true}, {"(> 3 2 1)", true},
		{"(>= 3 3 2)", true}, {"(< 1 3 2)", false}, {"(not nil)", true},
		{"(not 0)", false}, {"(= [1] [1.0])", true}, {"(= 1 1 1.0)", true},
		{"(and false (/ 1 0))", false}, {"(or \"ok\" (/ 1 0))", "ok"},
		{`(def make-counter (fn [] (let [n 0] (fn [] (set! n (+ n 1)))))) (def next (make-counter)) (next) (next)`, int64(2)},
		{`(def fact (fn [n] (if (= n 0) 1 (* n (fact (- n 1)))))) (fact 5)`, int64(120)},
	} {
		t.Run(tt.source, func(t *testing.T) {
			got := evaluate(t, tt.source)
			if !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
			if _, wantFloat := tt.want.(float64); wantFloat {
				if _, ok := got.(float64); !ok {
					t.Fatalf("expected Float, got %T", got)
				}
			}
		})
	}
}

func TestNumericErrors(t *testing.T) {
	for _, source := range []string{
		"(+ 9223372036854775807 1)", "(- -9223372036854775808 1)",
		"(- -9223372036854775808)", "(* 9223372036854775807 2)",
		"(* -9223372036854775808 -1)", "(inc 9223372036854775807)",
		"(/ 1 0)", "(/ 1 -0.0)", "(+ 1e308 1e308)", "(* 1e308 2.0)", "(/ 1e308 0.1)",
	} {
		expectError(t, source, ArithmeticError)
	}
	for _, source := range []string{"(+ true 1)", "(- \"1\")", "(* [] 1)", "(/ 1 nil)", "(< 1 :x)", "(inc false)"} {
		expectError(t, source, TypeError)
	}
	for _, source := range []string{"(-)", "(/ 1)", "(!= 1)", "(!= 1 2 3)", "(not)", "(inc 1 2)", "(=)", "(< 1)"} {
		expectError(t, source, ArityError)
	}
}
