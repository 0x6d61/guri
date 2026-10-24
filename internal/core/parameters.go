package core

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

type parameter struct {
	initial   scheme.Value
	converter *scheme.Procedure
}

func (*parameter) ProcedureCode() {}

type parameterBindings map[*parameter]scheme.Value

func (p *parameter) Call(c *vm.Context, args []scheme.Value) (vm.Action, error) {
	if len(args) != 0 {
		return vm.Action{}, vm.ErrArity
	}
	return vm.Return(p.value(c)), nil
}
func (p *parameter) value(c *vm.Context) scheme.Value {
	for d := c.Dynamic(); d != nil; d = d.Parent() {
		if values, ok := d.Payload().(parameterBindings); ok {
			if v, found := values[p]; found {
				return v
			}
		}
	}
	return p.initial
}
func addParameter(old parameterBindings, p *parameter, value scheme.Value) parameterBindings {
	values := parameterBindings{}
	for key, v := range old {
		values[key] = v
	}
	values[p] = value
	return values
}
func parameterAction(c *vm.Context, params []*parameter, values []scheme.Value, index int, bound parameterBindings, old *vm.Dynamic, thunk scheme.Value) (vm.Action, error) {
	for index < len(params) {
		p, value := params[index], values[index]
		if p.converter == nil {
			bound = addParameter(bound, p, value)
			index++
			continue
		}
		next := index + 1
		previous := bound
		return vm.Invoke(p.converter, []scheme.Value{value}, func(ctx *vm.Context, r scheme.Results) (vm.Action, error) {
			converted, err := r.Single()
			if err != nil {
				return vm.Action{}, err
			}
			return parameterAction(ctx, params, values, next, addParameter(previous, p, converted), old, thunk)
		}), nil
	}
	return vm.Invoke(thunk, nil, func(_ *vm.Context, r scheme.Results) (vm.Action, error) {
		return vm.Return(r.Values()...).WithDynamic(old), nil
	}).WithDynamic(vm.NewDynamic(old, bound)), nil
}
func (b *builder) lowerParameterize(n *reader.Node, args []*reader.Node) (*reader.Node, error) {
	if len(args) < 2 {
		return nil, syntax(n, "parameterize requires bindings and body")
	}
	inits, err := nodes(args[0])
	if err != nil {
		return nil, err
	}
	l := b.lowerer(n.Source())
	var operands []*reader.Node
	for _, init := range inits {
		xs, err := nodes(init)
		if err != nil {
			return nil, err
		}
		if len(xs) != 2 {
			return nil, syntax(init, "parameter binding requires parameter and value")
		}
		operands = append(operands, xs...)
	}
	operands = append(operands, l.form("lambda", append([]*reader.Node{atom(scheme.Null, l.pos)}, args[1:]...)...))
	count := len(operands)
	run := vm.Native("parameterize", count, count, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		var params []*parameter
		var values []scheme.Value
		for i := 0; i < len(a)-1; i += 2 {
			proc, ok := a[i].(*scheme.Procedure)
			if !ok {
				return vm.Action{}, scheme.ErrType
			}
			param, ok := proc.Code().(*parameter)
			if !ok {
				return vm.Action{}, scheme.ErrType
			}
			params = append(params, param)
			values = append(values, a[i+1])
		}
		return parameterAction(c, params, values, 0, parameterBindings{}, c.Dynamic(), a[len(a)-1])
	})
	return l.call(atom(run, l.pos), operands...), nil
}
