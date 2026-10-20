package guri

import (
	"bytes"
	"errors"
	"testing"
)

func TestErrorFunction(t *testing.T) {
	for _, message := range []string{"対応する [ がありません。", "", "100% %s %d"} {
		i := NewInterpreter()
		var out bytes.Buffer
		i.Output = &out
		value, err := i.Eval("user-error.guri", "\n  (error "+Format(message)+")")
		var e *RuntimeError
		if !errors.As(err, &e) || e.Kind != UserError || e.Cause != message || e.Position != (Position{"user-error.guri", 2, 3}) {
			t.Fatalf("message=%q: got %v", message, err)
		}
		if value != nil || out.Len() != 0 {
			t.Fatalf("error returned value or printed output: %#v %q", value, out.String())
		}
	}
}

func TestErrorArityAndType(t *testing.T) {
	for _, source := range []string{`(error)`, `(error "a" "b")`} {
		expectError(t, source, ArityError)
	}
	for _, source := range []string{`(error nil)`, `(error 1)`, `(error false)`, `(error :message)`, `(error b"message")`, `(error [])`} {
		expectError(t, source, TypeError)
	}
}

func TestErrorPropagation(t *testing.T) {
	for _, source := range []string{
		`(error "stop")`,
		`(def fail error) (fail "stop")`,
		`((fn [] (error "stop")))`,
		`(let [] (error "stop"))`,
		`(while true (error "stop"))`,
		`(while (error "stop") nil)`,
		`(cond true (error "stop"))`,
		`(cond (error "stop") 1)`,
		`(reduce (fn [acc x] (error "stop")) 0 [1])`,
		`(update {:x "stop"} :x error)`,
		`[1 (error "stop") 3]`,
		`{:x (error "stop")}`,
	} {
		expectError(t, source, UserError)
	}
	for _, source := range []string{`(if false (error "skip") 1)`, `(and false (error "skip"))`, `(or true (error "skip"))`, `(cond false (error "skip") true 1)`, `(while false (error "skip"))`} {
		if _, err := NewInterpreter().Eval("short-circuit.guri", source); err != nil {
			t.Fatal(err)
		}
	}
}

func TestErrorAbortsEvaluationAndInterpreterSurvives(t *testing.T) {
	i := NewInterpreter()
	_, err := i.Eval("before.guri", `(def n 0) (set! n 1) (error "stop") (set! n 2)`)
	var e *RuntimeError
	if !errors.As(err, &e) || e.Kind != UserError {
		t.Fatalf("got %v", err)
	}
	if got, err := i.Eval("after.guri", `n`); err != nil || got != int64(1) {
		t.Fatalf("side effects or evaluation boundary wrong: %v %v", got, err)
	}
	if got, err := i.Eval("after.guri", `(+ n 2)`); err != nil || got != int64(3) {
		t.Fatalf("interpreter not reusable: %v %v", got, err)
	}
}

func TestErrorPreservesFunctionBodyPosition(t *testing.T) {
	i := NewInterpreter()
	if _, err := i.Eval("definition.guri", "(defn fail []\n  (error \"stop\"))"); err != nil {
		t.Fatal(err)
	}
	_, err := i.Eval("call.guri", `(fail)`)
	var e *RuntimeError
	if !errors.As(err, &e) || e.Kind != UserError || e.Position != (Position{"definition.guri", 2, 3}) {
		t.Fatalf("unexpected error: %v", err)
	}
}
