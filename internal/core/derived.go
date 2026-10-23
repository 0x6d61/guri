package core

import (
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// Lowering uses protected initial syntax bindings and fresh temporary names.
type lowerer struct {
	b    *builder
	pos  scheme.Source
	mark *scheme.Scope
}

func (b *builder) lowerer(pos scheme.Source) *lowerer {
	return &lowerer{b, pos, scheme.NewScopeWithContext(&definitionContext{engine: b.engine, globals: b.engine.primitives, source: pos})}
}
func (l *lowerer) name(name string) *reader.Node {
	return &reader.Node{Syntax: scheme.NewSyntax(l.b.engine.symbols.Intern(name), l.pos, l.mark)}
}
func (l *lowerer) call(fn *reader.Node, args ...*reader.Node) *reader.Node {
	return list(append([]*reader.Node{fn}, args...), atom(scheme.Null, l.pos), l.pos)
}
func (l *lowerer) form(name string, args ...*reader.Node) *reader.Node {
	return l.call(l.name(name), args...)
}
func (l *lowerer) seq(xs []*reader.Node) *reader.Node {
	if len(xs) == 0 {
		return atom(scheme.Unspecified, l.pos)
	}
	return l.form("begin", xs...)
}
func (l *lowerer) quoted(n *reader.Node) *reader.Node { return l.form("quote", n) }
func (l *lowerer) bind(name, init, body *reader.Node) *reader.Node {
	bs := list([]*reader.Node{list([]*reader.Node{name, init}, atom(scheme.Null, l.pos), l.pos)}, atom(scheme.Null, l.pos), l.pos)
	return l.form("let", bs, body)
}
func (b *builder) isAux(n *reader.Node, name string) bool {
	ident, ok := n.Syntax.Identifier()
	if !ok {
		return false
	}
	bind, _ := b.resolve(ident)
	return bind.form == auxiliary && bind.id.Symbol().Name() == name
}
func (b *builder) lowerDerived(n *reader.Node, args []*reader.Node) (*reader.Node, error) {
	ident, _ := id(n.Car)
	bind, _ := b.resolve(ident)
	name := bind.id.Symbol().Name()
	l := b.lowerer(n.Source())
	switch name {
	case "and", "or":
		if len(args) == 0 {
			return atom(scheme.Boolean(name == "and"), l.pos), nil
		}
		if len(args) == 1 {
			return args[0], nil
		}
		rest := l.form(name, args[1:]...)
		if name == "and" {
			return l.form("if", args[0], rest, atom(scheme.False, l.pos)), nil
		}
		tmp := l.name("or-value")
		return l.bind(tmp, args[0], l.form("if", tmp, tmp, rest)), nil
	case "when", "unless":
		if len(args) < 2 {
			return nil, syntax(n, "when/unless require test and body")
		}
		yes, no := l.seq(args[1:]), atom(scheme.Unspecified, l.pos)
		if name == "unless" {
			yes, no = no, yes
		}
		return l.form("if", args[0], yes, no), nil
	case "cond":
		return b.condClauses(l, args)
	case "case":
		if len(args) == 0 {
			return nil, syntax(n, "case requires key")
		}
		key := l.name("case-key")
		body, err := b.caseClauses(l, key, args[1:], map[scheme.Value]bool{})
		if err != nil {
			return nil, err
		}
		return l.bind(key, args[0], body), nil
	case "do":
		return b.lowerDo(l, n, args)
	case "case-lambda":
		return b.lowerCaseLambda(l, n, args)
	}
	return nil, syntax(n, "unknown derived syntax")
}
func (b *builder) condClauses(l *lowerer, clauses []*reader.Node) (*reader.Node, error) {
	return b.condClausesDefault(l, clauses, atom(scheme.Unspecified, l.pos))
}
func (b *builder) condClausesDefault(l *lowerer, clauses []*reader.Node, fallback *reader.Node) (*reader.Node, error) {
	if len(clauses) == 0 {
		return fallback, nil
	}
	c, err := nodes(clauses[0])
	if err != nil {
		return nil, err
	}
	if len(c) == 0 {
		return nil, syntax(clauses[0], "empty cond clause")
	}
	if b.isAux(c[0], "else") {
		if len(clauses) != 1 || len(c) < 2 {
			return nil, syntax(clauses[0], "else must be last and have a body")
		}
		return l.seq(c[1:]), nil
	}
	rest, err := b.condClausesDefault(l, clauses[1:], fallback)
	if err != nil {
		return nil, err
	}
	if len(c) >= 2 && b.isAux(c[1], "=>") {
		if len(c) != 3 {
			return nil, syntax(clauses[0], "cond => requires one recipient")
		}
		tmp := l.name("cond-value")
		return l.bind(tmp, c[0], l.form("if", tmp, l.call(c[2], tmp), rest)), nil
	}
	if len(c) == 1 {
		tmp := l.name("cond-value")
		return l.bind(tmp, c[0], l.form("if", tmp, tmp, rest)), nil
	}
	return l.form("if", c[0], l.seq(c[1:]), rest), nil
}
func eqvDatum(a, b scheme.Value) bool { return equality.Eqv(a, b) }

func (b *builder) caseClauses(l *lowerer, key *reader.Node, clauses []*reader.Node, seen map[scheme.Value]bool) (*reader.Node, error) {
	if len(clauses) == 0 {
		return atom(scheme.Unspecified, l.pos), nil
	}
	c, err := nodes(clauses[0])
	if err != nil {
		return nil, err
	}
	if len(c) < 2 {
		return nil, syntax(clauses[0], "case clause requires body")
	}
	isElse := b.isAux(c[0], "else")
	var data []*reader.Node
	if isElse {
		if len(clauses) != 1 {
			return nil, syntax(clauses[0], "else must be last")
		}
	} else {
		data, err = nodes(c[0])
		if err != nil {
			return nil, err
		}
		for _, node := range data {
			for old := range seen {
				if eqvDatum(old, node.Datum()) {
					return nil, syntax(node, "duplicate case datum")
				}
			}
			seen[node.Datum()] = true
		}
	}
	var yes *reader.Node
	if b.isAux(c[1], "=>") {
		if len(c) != 3 {
			return nil, syntax(clauses[0], "case => requires one recipient")
		}
		yes = l.call(c[2], key)
	} else {
		yes = l.seq(c[1:])
	}
	if isElse {
		return yes, nil
	}
	rest, err := b.caseClauses(l, key, clauses[1:], seen)
	if err != nil {
		return nil, err
	}
	values := make([]scheme.Value, len(data))
	for i, v := range data {
		values[i] = v.Datum()
	}
	member := vm.Native("case-match", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		for _, v := range values {
			if eqvDatum(a[0], v) {
				return vm.Return(scheme.True), nil
			}
		}
		return vm.Return(scheme.False), nil
	})
	return l.form("if", l.call(atom(member, l.pos), key), yes, rest), nil
}
func (b *builder) lowerDo(l *lowerer, n *reader.Node, args []*reader.Node) (*reader.Node, error) {
	if len(args) < 2 {
		return nil, syntax(n, "do requires bindings and termination clause")
	}
	bs, err := nodes(args[0])
	if err != nil {
		return nil, err
	}
	end, err := nodes(args[1])
	if err != nil {
		return nil, err
	}
	if len(end) == 0 {
		return nil, syntax(args[1], "do requires termination test")
	}
	var inits, steps []*reader.Node
	for _, x := range bs {
		parts, err := nodes(x)
		if err != nil {
			return nil, err
		}
		if len(parts) < 2 || len(parts) > 3 {
			return nil, syntax(x, "do binding requires name, init and optional step")
		}
		if _, err := id(parts[0]); err != nil {
			return nil, err
		}
		inits = append(inits, list(parts[:2], atom(scheme.Null, l.pos), l.pos))
		step := parts[0]
		if len(parts) == 3 {
			step = parts[2]
		}
		steps = append(steps, step)
	}
	loop := l.name("do-loop")
	commands := append(append([]*reader.Node(nil), args[2:]...), l.call(loop, steps...))
	body := l.form("if", end[0], l.seq(end[1:]), l.seq(commands))
	return l.form("let", loop, list(inits, atom(scheme.Null, l.pos), l.pos), body), nil
}

type clauseArity struct {
	required int
	rest     bool
}

func (b *builder) lowerCaseLambda(l *lowerer, n *reader.Node, clauses []*reader.Node) (*reader.Node, error) {
	var functions []*reader.Node
	var arities []clauseArity
	for _, clause := range clauses {
		xs, err := nodes(clause)
		if err != nil {
			return nil, err
		}
		if len(xs) < 2 {
			return nil, syntax(clause, "case-lambda requires formals and body")
		}
		a := clauseArity{}
		f := xs[0]
		for f.Datum().Kind() == scheme.KindPair {
			a.required++
			f = f.Cdr
		}
		a.rest = f.Datum() != scheme.Null
		functions = append(functions, l.form("lambda", xs...))
		arities = append(arities, a)
	}
	factory := vm.Native("make-case-lambda", len(functions), len(functions), func(_ *vm.Context, values []scheme.Value) (vm.Action, error) {
		targets := append([]scheme.Value(nil), values...)
		dispatch := vm.Native("case-lambda", 0, -1, func(_ *vm.Context, args []scheme.Value) (vm.Action, error) {
			for i, a := range arities {
				if len(args) == a.required || a.rest && len(args) >= a.required {
					return vm.Invoke(targets[i], args, nil), nil
				}
			}
			return vm.Action{}, vm.ErrArity
		})
		return vm.Return(dispatch), nil
	})
	return l.call(atom(factory, l.pos), functions...), nil
}
