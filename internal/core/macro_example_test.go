package core_test

import (
	"context"
	"fmt"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func ExampleEngine_Eval_macros() {
	e := core.NewCore()
	result, err := e.Eval(context.Background(), "macro.scm", `
 (define-syntax my-if
   (syntax-rules () ((_ test yes no) (if test yes no))))
 (let ((if (lambda (a b c) c))) (my-if #t 42 0))`)
	if err != nil {
		panic(err)
	}
	value, err := result.Single()
	if err != nil {
		panic(err)
	}
	fmt.Println(value.(*scheme.Integer).String())
	// Output: 42
}
