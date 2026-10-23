package core

import (
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func iterateVectors(c *vm.Context, name string, a []scheme.Value) (vm.Action, error) {
	if _, ok := a[0].(*scheme.Procedure); !ok {
		return pairFailure(c, name, a, scheme.ErrType)
	}
	inputs := make([]*scheme.Vector, len(a)-1)
	length := -1
	for i, v := range a[1:] {
		x, err := vectorValue(v)
		if err != nil {
			return pairFailure(c, name, a, err)
		}
		inputs[i] = x
		if length < 0 || x.Len() < length {
			length = x.Len()
		}
	}
	return vectorStep(c, name, a[0], inputs, 0, length, nil)
}
func vectorStep(c *vm.Context, name string, procedure scheme.Value, inputs []*scheme.Vector, index, length int, prefix *mappedPrefix) (vm.Action, error) {
	if index == length {
		if name == "vector-for-each" {
			return vm.Return(scheme.Unspecified), nil
		}
		values := make([]scheme.Value, length)
		i := length
		for node := prefix; node != nil; node = node.previous {
			i--
			values[i] = node.value
		}
		return vm.Return(scheme.NewVector(values...)), nil
	}
	args := make([]scheme.Value, len(inputs))
	for i, v := range inputs {
		x, err := v.Ref(index)
		if err != nil {
			return pairFailure(c, name, nil, err)
		}
		args[i] = x
	}
	return vm.Invoke(procedure, args, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
		next := prefix
		if name == "vector-map" {
			v, err := r.Single()
			if err != nil {
				return pairFailure(c, name, args, err)
			}
			next = &mappedPrefix{v, prefix}
		}
		return vectorStep(c, name, procedure, inputs, index+1, length, next)
	}), nil
}
