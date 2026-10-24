package core

import (
	"os"
	"sort"

	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func (e *Engine) installLibraryPrimitives() {
	e.features = map[string]bool{"guri": true, "exact-closed": true, "exact-complex": true, "ieee-float": true, "full-unicode": true, "ratios": true}
	e.Bind("features", vm.Native("features", 0, 0, func(_ *vm.Context, _ []scheme.Value) (vm.Action, error) {
		var names []string
		for name, enabled := range e.features {
			if enabled {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		var values []scheme.Value
		for _, name := range names {
			values = append(values, e.symbols.Intern(name))
		}
		return vm.Return(scheme.List(values...)), nil
	}))
	e.Bind("procedure?", vm.Native("procedure?", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		_, ok := a[0].(*scheme.Procedure)
		return vm.Return(scheme.Boolean(ok)), nil
	}))
	for _, name := range []string{"file-exists?", "delete-file"} {
		e.Bind(name, vm.Native(name, 1, 1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			s, ok := a[0].(*scheme.String)
			if !ok {
				return portFailure(c, name, a, ErrorOrdinary, scheme.ErrType)
			}
			if name == "delete-file" {
				if err := os.Remove(s.Text()); err != nil {
					return portFailure(c, name, a, ErrorFile, err)
				}
				return vm.Return(scheme.Unspecified), nil
			}
			_, err := os.Stat(s.Text())
			if os.IsNotExist(err) {
				return vm.Return(scheme.False), nil
			}
			if err != nil {
				return portFailure(c, name, a, ErrorFile, err)
			}
			return vm.Return(scheme.True), nil
		}))
	}
}
