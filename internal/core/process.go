package core

import (
	"fmt"
	"math/big"

	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// ExitRequest is the internal terminal outcome consumed by the future CLI.
// It is deliberately not a Scheme condition or a public embedding API.
type ExitRequest struct {
	Code      int
	Emergency bool
}

func (e *ExitRequest) Error() string {
	name := "exit"
	if e.Emergency {
		name = "emergency-exit"
	}
	return fmt.Sprintf("%s requested status %d", name, e.Code)
}
func exitCode(args []scheme.Value) int {
	if len(args) == 0 || args[0] == scheme.True {
		return 0
	}
	if n, ok := args[0].(*scheme.Integer); ok {
		return int(new(big.Int).Mod(n.BigInt(), big.NewInt(256)).Int64())
	}
	return 1
}
func (e *Engine) installProcessExit() {
	for _, name := range []string{"exit", "emergency-exit"} {
		e.Bind(name, vm.Native(name, 0, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
			emergency := name == "emergency-exit"
			action := vm.Fail(&ExitRequest{Code: exitCode(a), Emergency: emergency})
			if !emergency {
				action = action.WithDynamic(nil)
			}
			return action, nil
		}))
	}
}
