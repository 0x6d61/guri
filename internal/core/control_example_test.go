package core_test

import (
	"context"
	"fmt"
	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
	"testing"
)

func ExampleEngine_Eval_values() {
	e := core.NewCore()
	r, err := e.Eval(context.Background(), "values.scm", `(call-with-values (lambda () (values 20 22)) (lambda (a b) b))`)
	if err != nil {
		panic(err)
	}
	v, err := r.Single()
	if err != nil {
		panic(err)
	}
	fmt.Println(v.(*scheme.Integer).String())
	// Output: 22
}
func TestControlReferenceExamples(t *testing.T) {
	for s, want := range map[string]int64{`(call/cc (lambda (exit) (exit 42)))`: 42, `(apply (lambda (x) x) '(42))`: 42, `(define-values (a b) (values 20 22)) b`: 22, `(let-values (((a b) (values 20 22))) b)`: 22, `(let*-values (((x) (values 42)) ((y) (values x))) y)`: 42} {
		if got := number(eval(t, core.NewCore(), s)); got != want {
			t.Fatalf("%s: got %d want %d", s, got, want)
		}
	}
}
