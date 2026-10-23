package core_test

import (
	"context"
	"fmt"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func ExampleEngine_Eval() {
	e := core.NewCore()
	results, err := e.Eval(context.Background(), "example.scm", `(define (choose x) (if x 42 0)) (choose '())`)
	if err != nil {
		panic(err)
	}
	value, err := results.Single()
	if err != nil {
		panic(err)
	}
	fmt.Println(value.(*scheme.Integer).String())
	// Output: 42
}
