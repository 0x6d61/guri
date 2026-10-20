package guri

import (
	"errors"
	"io"
	"os"
	"testing"
)

func TestCond(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   Value
	}{
		{`(cond)`, nil}, {`(cond false 1 nil 2)`, nil},
		{`(cond false missing true 42 missing missing)`, int64(42)},
		{`(cond false 1 true 2)`, int64(2)}, {`(cond true nil true 3)`, nil},
		{`(cond 0 1 true 2)`, int64(1)}, {`(cond "" 1 true 2)`, int64(1)},
		{`(cond [] 1 true 2)`, int64(1)}, {`(cond {} 1 true 2)`, int64(1)},
		{`(cond '() 1 true 2)`, int64(1)}, {`(cond b"" 1 true 2)`, int64(1)},
		{`(let [n 0] (cond (do (set! n 1) false) missing (do (set! n (inc n)) true) n true missing))`, int64(2)},
		{`(let [n 0] (cond true (do (set! n 3) (inc n))) n)`, int64(3)},
		{`(cond true (def answer 42)) answer`, int64(42)},
		{`(defn classify [x] (cond (< x 0) :negative (= x 0) :zero true :positive)) (classify 2)`, KeywordValue("positive")},
		{`(cond false 1 true (cond false 2 true 3))`, int64(3)},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := evaluate(t, tt.source); !Equal(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCondErrors(t *testing.T) {
	for _, source := range []string{`(cond true)`, `(cond false 1 true)`} {
		expectError(t, source, ArityError)
	}
	expectError(t, `(cond missing 1)`, NameError)
	expectError(t, `(cond true (/ 1 0))`, ArithmeticError)
	i := NewInterpreter()
	if _, err := i.Eval("cond.guri", `(def n 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Eval("cond.guri", `(cond (do (set! n 1) true) 42 false)`); err == nil {
		t.Fatal("expected unmatched condition error")
	}
	if value, err := i.Eval("cond.guri", `n`); err != nil || value != int64(0) {
		t.Fatalf("invalid cond evaluated a condition: %v, %v", value, err)
	}
	_, err := i.Eval("cond.guri", "(cond\n  true (/ 1 0))")
	var e *RuntimeError
	if !errors.As(err, &e) || e.Kind != ArithmeticError || e.Position != (Position{"cond.guri", 2, 8}) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBracketSearchExample(t *testing.T) {
	source, err := os.ReadFile("examples/search-brackets.guri")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		code string
		ok   bool
	}{
		{"", true}, {"abc", true}, {"[]", true}, {"[[]]", true}, {"[][[]]", true}, {"日[本]語", true},
		{"[", false}, {"]", false}, {"][", false}, {"[[[]]", false},
	} {
		t.Run(tt.code, func(t *testing.T) {
			i := NewInterpreter()
			i.Output = io.Discard
			if _, err := i.Eval("examples/search-brackets.guri", string(source)); err != nil {
				t.Fatal(err)
			}
			value, err := i.Eval("bracket-test.guri", "(search-brackets "+Format(tt.code)+")")
			if err != nil {
				t.Fatal(err)
			}
			m := value.(*MapValue)
			if m.entries[KeywordValue("ok")] != tt.ok {
				t.Fatalf("unexpected result: %s", Format(m))
			}
			if tt.code == "[[]]" {
				pairs := m.entries[KeywordValue("brackets")].(*MapValue)
				for left, right := range map[int64]int64{0: 3, 3: 0, 1: 2, 2: 1} {
					if pairs.entries[left] != right {
						t.Fatalf("missing bracket %d -> %d", left, right)
					}
				}
			}
			if tt.code == "日[本]語" {
				pairs := m.entries[KeywordValue("brackets")].(*MapValue)
				if pairs.entries[int64(1)] != int64(3) || pairs.entries[int64(3)] != int64(1) {
					t.Fatal("bracket positions must use code points")
				}
			}
		})
	}
}
