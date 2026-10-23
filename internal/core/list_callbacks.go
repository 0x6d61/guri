package core

import (
	"strings"

	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// The prefix is persistent Go state, not a mutable Scheme result list. A saved
// callback continuation can create a fresh result every time it is resumed.
type mappedPrefix struct {
	value    scheme.Value
	previous *mappedPrefix
}

func iterateLists(c *vm.Context, name string, args []scheme.Value) (vm.Action, error) {
	if _, ok := args[0].(*scheme.Procedure); !ok {
		return pairFailure(c, name, args, scheme.ErrType)
	}
	finite := false
	for _, tail := range args[1:] {
		ends, err := finiteList(tail)
		if err != nil {
			return pairFailure(c, name, args, err)
		}
		finite = finite || ends
	}
	if !finite {
		return pairFailure(c, name, args, scheme.ErrType)
	}
	return mapStep(c, name, args[0], args[1:], nil)
}
func mapStep(c *vm.Context, name string, procedure scheme.Value, tails []scheme.Value, prefix *mappedPrefix) (vm.Action, error) {
	values := make([]scheme.Value, len(tails))
	next := make([]scheme.Value, len(tails))
	empty := false
	for i, tail := range tails {
		if tail == scheme.Null {
			empty = true
			continue
		}
		p, ok := tail.(*scheme.Pair)
		if !ok {
			return pairFailure(c, name, tails, scheme.ErrType)
		}
		values[i], next[i] = p.Car(), p.Cdr()
	}
	if empty {
		if name == "for-each" {
			return vm.Return(scheme.Unspecified), nil
		}
		var out scheme.Value = scheme.Null
		for node := prefix; node != nil; node = node.previous {
			out = scheme.Cons(node.value, out)
		}
		return vm.Return(out), nil
	}
	return vm.Invoke(procedure, values, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
		result := prefix
		if name == "map" {
			value, err := r.Single()
			if err != nil {
				return pairFailure(c, name, values, err)
			}
			result = &mappedPrefix{value, prefix}
		}
		return mapStep(c, name, procedure, next, result)
	}), nil
}

// Brent's cycle detector is a small value copied into each resume closure.
// It needs neither a mutable visited map nor a quadratic persistent chain.
type listCycle struct {
	anchor        *scheme.Pair
	power, period uint64
}

func (state listCycle) advance(p *scheme.Pair) (listCycle, bool) {
	if state.anchor == nil {
		return listCycle{anchor: p, power: 1}, false
	}
	state.period++
	if p == state.anchor {
		return state, true
	}
	if state.period == state.power {
		state.anchor = p
		state.power *= 2
		state.period = 0
	}
	return state, false
}
func searchList(c *vm.Context, name string, args []scheme.Value) (vm.Action, error) {
	var compare scheme.Value
	if len(args) == 3 {
		if _, ok := args[2].(*scheme.Procedure); !ok {
			return pairFailure(c, name, args, scheme.ErrType)
		}
		compare = args[2]
	}
	return searchStep(c, name, args[0], args[1], compare, listCycle{})
}
func searchStep(c *vm.Context, name string, key, tail, compare scheme.Value, seen listCycle) (vm.Action, error) {
	// The built-in comparison takes this loop without accumulating VM frames.
	for tail != scheme.Null {
		p, ok := tail.(*scheme.Pair)
		if !ok {
			return pairFailure(c, name, []scheme.Value{key, tail}, scheme.ErrType)
		}
		visited, cycle := seen.advance(p)
		if cycle {
			return pairFailure(c, name, []scheme.Value{key, tail}, scheme.ErrType)
		}
		candidate := p.Car()
		result := scheme.Value(p)
		if strings.HasPrefix(name, "ass") {
			entry, ok := candidate.(*scheme.Pair)
			if !ok {
				return pairFailure(c, name, []scheme.Value{key, tail}, scheme.ErrType)
			}
			candidate, result = entry.Car(), entry
		}
		if compare != nil {
			return vm.Invoke(compare, []scheme.Value{key, candidate}, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
				v, err := r.Single()
				if err != nil {
					return pairFailure(c, name, []scheme.Value{key, candidate}, err)
				}
				if scheme.Truthy(v) {
					return vm.Return(result), nil
				}
				return searchStep(c, name, key, p.Cdr(), compare, visited)
			}), nil
		}
		match := false
		switch name {
		case "memq", "assq":
			match = equality.Eq(key, candidate)
		case "memv", "assv":
			match = equality.Eqv(key, candidate)
		default:
			match = equality.Equal(key, candidate)
		}
		if match {
			return vm.Return(result), nil
		}
		seen, tail = visited, p.Cdr()
	}
	return vm.Return(scheme.False), nil
}

// Input list mutation during map/for-each is an error in the standard. Validate
// the initial spines without snapshotting their elements or excluding mixed
// circular/finite inputs.
func finiteList(tail scheme.Value) (bool, error) {
	state := listCycle{}
	for tail != scheme.Null {
		p, ok := tail.(*scheme.Pair)
		if !ok {
			return false, scheme.ErrType
		}
		next, cycle := state.advance(p)
		if cycle {
			return false, nil
		}
		state = next
		tail = p.Cdr()
	}
	return true, nil
}
