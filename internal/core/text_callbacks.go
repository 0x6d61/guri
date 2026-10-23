package core

import (
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func iterateStrings(c *vm.Context, name string, a []scheme.Value) (vm.Action, error) {
	if _, ok := a[0].(*scheme.Procedure); !ok {
		return pairFailure(c, name, a, scheme.ErrType)
	}
	strings := make([]*scheme.String, len(a)-1)
	length := -1
	for i, v := range a[1:] {
		s, err := stringValue(v)
		if err != nil {
			return pairFailure(c, name, a, err)
		}
		strings[i] = s
		if length < 0 || s.Len() < length {
			length = s.Len()
		}
	}
	return stringStep(c, name, a[0], strings, 0, length, nil)
}
func stringStep(c *vm.Context, name string, procedure scheme.Value, inputs []*scheme.String, index, length int, prefix *mappedPrefix) (vm.Action, error) {
	if index == length {
		if name == "string-for-each" {
			return vm.Return(scheme.Unspecified), nil
		}
		chars := make([]rune, length)
		i := length
		for node := prefix; node != nil; node = node.previous {
			i--
			chars[i] = node.value.(scheme.Character).Rune()
		}
		result, err := scheme.NewString(string(chars))
		if err != nil {
			return vm.Action{}, err
		}
		return vm.Return(result), nil
	}
	args := make([]scheme.Value, len(inputs))
	for i, s := range inputs {
		value, err := s.Ref(index)
		if err != nil {
			return pairFailure(c, name, nil, err)
		}
		args[i] = value
	}
	return vm.Invoke(procedure, args, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
		next := prefix
		if name == "string-map" {
			value, err := r.Single()
			if err != nil {
				return pairFailure(c, name, args, err)
			}
			if _, ok := value.(scheme.Character); !ok {
				return pairFailure(c, name, []scheme.Value{value}, scheme.ErrType)
			}
			next = &mappedPrefix{value, prefix}
		}
		return stringStep(c, name, procedure, inputs, index+1, length, next)
	}), nil
}
