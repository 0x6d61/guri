package core

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

type bindingInit struct{ name, expr *reader.Node }

func bindings(n *reader.Node) ([]bindingInit, error) {
	xs, err := nodes(n)
	if err != nil {
		return nil, err
	}
	var out []bindingInit
	for _, x := range xs {
		pair, err := nodes(x)
		if err != nil {
			return nil, err
		}
		if len(pair) != 2 {
			return nil, syntax(x, "binding requires name and initializer")
		}
		if _, err := id(pair[0]); err != nil {
			return nil, err
		}
		out = append(out, bindingInit{pair[0], pair[1]})
	}
	return out, nil
}
func (b *builder) bindingForm(n *reader.Node, args []*reader.Node, kind form, tail bool) error {
	if len(args) < 2 {
		return syntax(n, "binding form requires bindings and body")
	}
	pos := n.Source()
	var name *reader.Node
	if kind == let {
		if _, ok := args[0].Datum().(*scheme.Symbol); ok {
			name = args[0]
			args = args[1:]
			if len(args) < 2 {
				return syntax(n, "named let requires body")
			}
		}
	}
	inits, err := bindings(args[0])
	if err != nil {
		return err
	}
	body := args[1:]
	if kind == letStar {
		return b.sequentialBindings(inits, body, tail, pos)
	}
	if name != nil {
		return b.namedLet(name, inits, body, tail, pos)
	}
	env := &scope{parent: b.env}
	var binds []*binding
	for _, init := range inits {
		ident, _ := id(init.name)
		bind, err := env.add(ident)
		if err != nil {
			return syntax(init.name, "duplicate binding")
		}
		binds = append(binds, bind)
	}
	required := 0
	if kind == let {
		required = len(inits)
	}
	child := b.child(env, required, false)
	if kind == letRec || kind == letRecStar {
		for i, init := range inits {
			if err := child.expr(init.expr, false, false); err != nil {
				return err
			}
			if kind == letRec {
				child.emit(vm.Push, 0, 0, init.name.Source())
			} else {
				child.emit(vm.InitLocal, 0, binds[i].slot, init.name.Source())
			}
		}
		if kind == letRec {
			for i := len(inits) - 1; i >= 0; i-- {
				child.emit(vm.Pop, 0, 0, pos)
				child.emit(vm.InitLocal, 0, binds[i].slot, inits[i].name.Source())
			}
		}
	}
	if err := child.body(body, pos); err != nil {
		return err
	}
	if err := b.closure(child, pos); err != nil {
		return err
	}
	b.emit(vm.Push, 0, 0, pos)
	if kind == let {
		for _, init := range inits {
			if err := b.expr(init.expr, false, false); err != nil {
				return err
			}
			b.emit(vm.Push, 0, 0, init.name.Source())
		}
	}
	op := vm.Call
	if tail {
		op = vm.TailCall
	}
	b.emit(op, required, 0, pos)
	return nil
}
func (b *builder) sequentialBindings(inits []bindingInit, body []*reader.Node, tail bool, pos scheme.Source) error {
	env := &scope{parent: b.env}
	required := 0
	if len(inits) > 0 {
		ident, _ := id(inits[0].name)
		if _, err := env.add(ident); err != nil {
			return err
		}
		required = 1
	}
	child := b.child(env, required, false)
	if len(inits) <= 1 {
		if err := child.body(body, pos); err != nil {
			return err
		}
	} else {
		if err := child.sequentialBindings(inits[1:], body, true, pos); err != nil {
			return err
		}
	}
	if err := b.closure(child, pos); err != nil {
		return err
	}
	b.emit(vm.Push, 0, 0, pos)
	if required == 1 {
		if err := b.expr(inits[0].expr, false, false); err != nil {
			return err
		}
		b.emit(vm.Push, 0, 0, inits[0].name.Source())
	}
	op := vm.Call
	if tail {
		op = vm.TailCall
	}
	b.emit(op, required, 0, pos)
	return nil
}
func (b *builder) namedLet(name *reader.Node, inits []bindingInit, body []*reader.Node, tail bool, pos scheme.Source) error {
	// Evaluate the recursive procedure expression first; initializers still compile
	// in the outer region, so the loop name does not capture initializer references.
	env := &scope{parent: b.env}
	ident, _ := id(name)
	bind, _ := env.add(ident)
	wrapper := b.child(env, 0, false)
	params := &scope{parent: env}
	for _, init := range inits {
		ident, _ := id(init.name)
		if _, err := params.add(ident); err != nil {
			return syntax(init.name, "duplicate named let parameter")
		}
	}
	loop := wrapper.child(params, len(inits), false)
	if err := loop.body(body, pos); err != nil {
		return err
	}
	if err := wrapper.closure(loop, pos); err != nil {
		return err
	}
	wrapper.emit(vm.InitLocal, 0, bind.slot, pos)
	wrapper.emit(vm.Local, 0, bind.slot, pos)
	if err := b.closure(wrapper, pos); err != nil {
		return err
	}
	b.emit(vm.Push, 0, 0, pos)
	b.emit(vm.Call, 0, 0, pos)
	b.emit(vm.Push, 0, 0, pos)
	for _, init := range inits {
		if err := b.expr(init.expr, false, false); err != nil {
			return err
		}
		b.emit(vm.Push, 0, 0, init.name.Source())
	}
	op := vm.Call
	if tail {
		op = vm.TailCall
	}
	b.emit(op, len(inits), 0, pos)
	return nil
}
