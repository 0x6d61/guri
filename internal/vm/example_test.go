package vm_test

import (
	"context"
	"fmt"

	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func ExampleMachine_Run() {
	c, err := vm.NewCode(vm.Spec{Constants: []scheme.Value{scheme.NewInteger(42)}, Instructions: []vm.Instruction{{Op: vm.Const}, {Op: vm.ReturnOp}}})
	if err != nil {
		panic(err)
	}
	results, err := vm.New().Run(context.Background(), vm.Closure(c))
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
