package core

import (
	numeric "github.com/0x6d61/guri/internal/number"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func (e *Engine) installNumbers() {
	for name, arity := range numeric.Catalog() {
		e.Bind(name, vm.Native(name, arity.Min, arity.Max, func(c *vm.Context, args []scheme.Value) (vm.Action, error) {
			results, err := numeric.Call(name, args...)
			if err != nil {
				object, objectErr := NewErrorObject(ErrorOrdinary, err.Error(), args...)
				if objectErr != nil {
					return vm.Action{}, objectErr
				}
				return Raise(c, object, false)
			}
			return vm.Return(results.Values()...), nil
		}))
	}
}
