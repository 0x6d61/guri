package core

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// Private construction helpers do not publish incomplete standard procedures.
var qqCons = vm.Native("quasiquote-cons", 2, 2, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
	return vm.Return(scheme.Cons(a[0], a[1])), nil
})

func properValues(value scheme.Value) ([]scheme.Value, error) {
	var out []scheme.Value
	seen := map[*scheme.Pair]bool{}
	for value != scheme.Null {
		p, ok := value.(*scheme.Pair)
		if !ok || seen[p] {
			return nil, scheme.ErrType
		}
		seen[p] = true
		out = append(out, p.Car())
		value = p.Cdr()
	}
	return out, nil
}

var qqAppend = vm.Native("quasiquote-splice", 2, 2, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
	xs, err := properValues(a[0])
	if err != nil {
		return vm.Action{}, err
	}
	tail := a[1]
	for i := len(xs) - 1; i >= 0; i-- {
		tail = scheme.Cons(xs[i], tail)
	}
	return vm.Return(tail), nil
})
var qqVector = vm.Native("quasiquote-vector", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
	xs, err := properValues(a[0])
	if err != nil {
		return vm.Action{}, err
	}
	return vm.Return(scheme.NewVector(xs...)), nil
})

func (b *builder) lowerQuasiquote(n *reader.Node, depth int, element bool) (*reader.Node, bool, error) {
	l := b.lowerer(n.Source())
	kind := b.headForm(n)
	if kind == quasiquote || kind == unquote || kind == splice {
		xs, err := nodes(n)
		if err != nil {
			return nil, false, err
		}
		if len(xs) != 2 {
			return nil, false, syntax(n, "quasiquote/unquote requires one argument")
		}
		if kind != quasiquote && depth == 1 {
			if kind == splice && !element {
				return nil, false, syntax(n, "unquote-splicing must occur in an element position")
			}
			return xs[1], true, nil
		}
		next := depth - 1
		if kind == quasiquote {
			next = depth + 1
		}
		inner, dynamic, err := b.lowerQuasiquote(xs[1], next, false)
		if err != nil {
			return nil, false, err
		}
		if !dynamic {
			return l.quoted(n), false, nil
		}
		tail := l.call(atom(qqCons, l.pos), inner, l.quoted(atom(scheme.Null, l.pos)))
		return l.call(atom(qqCons, l.pos), l.quoted(xs[0]), tail), true, nil
	}
	switch n.Datum().Kind() {
	case scheme.KindPair:
		car, carDynamic, err := b.lowerQuasiquote(n.Car, depth, true)
		if err != nil {
			return nil, false, err
		}
		cdr, cdrDynamic, err := b.lowerQuasiquote(n.Cdr, depth, false)
		if err != nil {
			return nil, false, err
		}
		if !carDynamic && !cdrDynamic {
			return l.quoted(n), false, nil
		}
		fn := qqCons
		if depth == 1 && b.headForm(n.Car) == splice {
			fn = qqAppend
		}
		return l.call(atom(fn, l.pos), car, cdr), true, nil

	case scheme.KindVector:
		expr := l.quoted(atom(scheme.Null, l.pos))
		dynamic := false
		for i := len(n.Elements) - 1; i >= 0; i-- {
			item, changed, err := b.lowerQuasiquote(n.Elements[i], depth, true)
			if err != nil {
				return nil, false, err
			}
			dynamic = dynamic || changed
			fn := qqCons
			if depth == 1 && b.headForm(n.Elements[i]) == splice {
				fn = qqAppend
			}
			expr = l.call(atom(fn, l.pos), item, expr)
		}
		if !dynamic {
			return l.quoted(n), false, nil
		}
		return l.call(atom(qqVector, l.pos), expr), true, nil
	default:
		if ident, ok := n.Syntax.Identifier(); ok {
			bind, _ := b.resolve(ident)
			if bind.form == quasiquote || bind.form == unquote || bind.form == splice {
				return nil, false, syntax(n, "misplaced quasiquote keyword")
			}
		}
		return l.quoted(n), false, nil
	}
}
