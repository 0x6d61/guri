package core_test

import (
	"context"
	"fmt"
	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func ExampleEngine_Eval_dynamicWind() {
	e := core.NewCore()
	r, err := e.Eval(context.Background(), "wind.scm", `(define stage 'initial) (define answer (dynamic-wind (lambda () (set! stage 'before)) (lambda () stage) (lambda () (set! stage 'after)))) (values answer stage)`)
	if err != nil {
		panic(err)
	}
	v := r.Values()
	fmt.Println(v[0].(*scheme.Symbol).Name(), v[1].(*scheme.Symbol).Name())
	// Output: before after
}
