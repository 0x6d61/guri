package core

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func (e *Engine) installControl() {
	e.controls = map[string]*scheme.Procedure{}
	cc := vm.Native("call-with-current-continuation", 1, 1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		return vm.Invoke(a[0], []scheme.Value{c.Capture()}, nil), nil
	})
	e.controls["call/cc"] = cc
	e.controls["call-with-current-continuation"] = cc
	e.controls["values"] = vm.Native("values", 0, -1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) { return vm.Return(a...), nil })
	e.controls["call-with-values"] = vm.Native("call-with-values", 2, 2, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		consumer := a[1]
		return vm.Invoke(a[0], nil, func(_ *vm.Context, r scheme.Results) (vm.Action, error) {
			return vm.Invoke(consumer, r.Values(), nil), nil
		}), nil
	})
	e.controls["apply"] = vm.Native("apply", 2, -1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		last, err := properValues(a[len(a)-1])
		if err != nil {
			return vm.Action{}, err
		}
		args := append(append([]scheme.Value(nil), a[1:len(a)-1]...), last...)
		return vm.Invoke(a[0], args, nil), nil
	})
	e.controls["dynamic-wind"] = vm.Native("dynamic-wind", 3, 3, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		for _, v := range a {
			if _, ok := v.(*scheme.Procedure); !ok {
				return vm.Action{}, scheme.ErrType
			}
		}
		old := c.Dynamic()
		extent := vm.NewWind(old, a[0].(*scheme.Procedure), a[2].(*scheme.Procedure))
		return vm.Invoke(a[1], nil, func(_ *vm.Context, r scheme.Results) (vm.Action, error) {
			return vm.Return(r.Values()...).WithDynamic(old), nil
		}).WithDynamic(extent), nil
	})
	for name, p := range e.controls {
		e.Bind(name, p)
	}
}
func (l *lowerer) control(name string, args ...*reader.Node) *reader.Node {
	return l.call(atom(l.b.engine.controls[name], l.pos), args...)
}

type valueFormals struct {
	names    []*reader.Node
	required int
	rest     bool
}

func parseValueFormals(n *reader.Node) (valueFormals, error) {
	out := valueFormals{}
	var seen []scheme.Identifier
	for n.Datum() != scheme.Null {
		var name *reader.Node
		if _, ok := n.Datum().(*scheme.Symbol); ok {
			name = n
			out.rest = true
		} else {
			if n.Datum().Kind() != scheme.KindPair {
				return out, syntax(n, "invalid value formals")
			}
			name = n.Car
			out.required++
		}
		ident, err := id(name)
		if err != nil {
			return out, err
		}
		for _, old := range seen {
			if old.SameContext(ident) {
				return out, syntax(name, "duplicate value formal")
			}
		}
		seen = append(seen, ident)
		out.names = append(out.names, name)
		if out.rest {
			break
		}
		n = n.Cdr
	}
	return out, nil
}
func (b *builder) lowerValueDefinition(n *reader.Node) (*reader.Node, error) {
	xs, err := nodes(n)
	if err != nil {
		return nil, err
	}
	if len(xs) != 3 {
		return nil, syntax(n, "define-values requires formals and initializer")
	}
	f, err := parseValueFormals(xs[1])
	if err != nil {
		return nil, err
	}
	l := b.lowerer(n.Source())
	temporary := l.name("defined-values")
	max := f.required
	if f.rest {
		max = -1
	}
	collect := vm.Native("collect-defined-values", f.required, max, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) { return vm.Return(scheme.List(a...)), nil })
	producer := l.form("lambda", atom(scheme.Null, l.pos), xs[2])
	init := l.control("call-with-values", producer, atom(collect, l.pos))
	definitions := []*reader.Node{l.form("define", temporary, init)}
	for i, name := range f.names {
		index := i
		rest := f.rest && i == f.required
		getter := vm.Native("defined-value", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
			tail := a[0]
			for j := 0; j < index; j++ {
				p, ok := tail.(*scheme.Pair)
				if !ok {
					return vm.Action{}, scheme.ErrValueCount
				}
				tail = p.Cdr()
			}
			if rest {
				return vm.Return(tail), nil
			}
			p, ok := tail.(*scheme.Pair)
			if !ok {
				return vm.Action{}, scheme.ErrValueCount
			}
			return vm.Return(p.Car()), nil
		})
		definitions = append(definitions, l.form("define", name, l.call(atom(getter, l.pos), temporary)))
	}
	return l.form("begin", definitions...), nil
}

type valueGroup struct {
	formals, init *reader.Node
	parsed        valueFormals
	temporary     *reader.Node
}

func (b *builder) lowerValueBindings(n *reader.Node, args []*reader.Node, sequential bool) (*reader.Node, error) {
	if len(args) < 2 {
		return nil, syntax(n, "value bindings require bindings and body")
	}
	raw, err := nodes(args[0])
	if err != nil {
		return nil, err
	}
	l := b.lowerer(n.Source())
	var groups []valueGroup
	var names []scheme.Identifier
	for _, node := range raw {
		xs, err := nodes(node)
		if err != nil {
			return nil, err
		}
		if len(xs) != 2 {
			return nil, syntax(node, "value binding requires formals and initializer")
		}
		f, err := parseValueFormals(xs[0])
		if err != nil {
			return nil, err
		}
		if !sequential {
			for _, name := range f.names {
				ident, _ := id(name)
				for _, old := range names {
					if old.SameContext(ident) {
						return nil, syntax(name, "duplicate let-values variable")
					}
				}
				names = append(names, ident)
			}
		}
		// Each group gets a distinct introduction scope; Symbol spelling is immaterial.
		temporary := b.lowerer(node.Source()).name("value-group")
		groups = append(groups, valueGroup{xs[0], xs[1], f, temporary})
	}
	if sequential {
		body := l.form("let", append([]*reader.Node{atom(scheme.Null, l.pos)}, args[1:]...)...)
		for i := len(groups) - 1; i >= 0; i-- {
			g := groups[i]
			producer := l.form("lambda", atom(scheme.Null, l.pos), g.init)
			consumer := l.form("lambda", g.formals, body)
			body = l.control("call-with-values", producer, consumer)
		}
		return body, nil
	}
	body := l.form("let", append([]*reader.Node{atom(scheme.Null, l.pos)}, args[1:]...)...)
	for i := len(groups) - 1; i >= 0; i-- {
		g := groups[i]
		consumer := l.form("lambda", g.formals, body)
		body = l.control("apply", consumer, g.temporary)
	}
	for i := len(groups) - 1; i >= 0; i-- {
		g := groups[i]
		producer := l.form("lambda", atom(scheme.Null, l.pos), g.init)
		collector := l.form("lambda", g.temporary, body)
		body = l.control("call-with-values", producer, collector)
	}
	return body, nil
}
