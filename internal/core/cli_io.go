package core

import (
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// SetStandardPorts configures the internal CLI's default parameters before a
// program runs. It does not close resources or change their ownership.
func (r *Runtime) SetStandardPorts(input, output, diagnostic *ports.Resource) error {
	if r.running {
		return vm.ErrReentrant
	}
	resources := []*ports.Resource{input, output, diagnostic}
	names := []string{"current-input-port", "current-output-port", "current-error-port"}
	parameters := make([]*parameter, len(names))
	for i, name := range names {
		if resources[i] == nil {
			return scheme.ErrType
		}
		if err := checkTextPort(resources[i], i == 0); err != nil {
			return err
		}
		b := r.bootstrap.find(nil, r.bootstrap.primitives, scheme.NewIdentifier(r.bootstrap.symbols.Intern(name)))
		value, err := b.cell.Get()
		if err != nil {
			return err
		}
		parameters[i] = value.(*scheme.Procedure).Code().(*parameter)
	}
	for i, p := range parameters {
		p.initial = ports.Value(resources[i])
	}
	return nil
}
