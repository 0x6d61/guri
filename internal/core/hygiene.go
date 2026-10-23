package core

import (
	"context"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// A definition context preserves global binding identities and the lexical
// region. Region contents are completed as internal definitions are discovered.
type definitionContext struct {
	source  scheme.Source
	engine  *Engine
	env     *scope
	globals []*binding
}

func (e *Engine) find(env *scope, globals []*binding, id scheme.Identifier) *binding {
	for s := env; s != nil; s = s.parent {
		for _, b := range s.bindings {
			if b.id.SameContext(id) {
				return b
			}
		}
	}
	for _, b := range globals {
		if b.id.SameContext(id) {
			return b
		}
	}
	scopes := id.Scopes()
	if len(scopes) > 0 {
		if c, ok := scopes[len(scopes)-1].Context().(*definitionContext); ok {
			original := scheme.NewIdentifier(id.Symbol())
			for _, s := range scopes[:len(scopes)-1] {
				original = original.WithScope(s)
			}
			return c.engine.find(c.env, c.globals, original)
		}
	}
	return nil
}
func (e *Engine) replaceGlobal(b *binding) {
	for i, old := range e.globals {
		if old.id.SameContext(b.id) {
			e.globals[i] = b
			return
		}
	}
	e.globals = append(e.globals, b)
}
func (e *Engine) variableBinding(id scheme.Identifier) *binding {
	b := e.global(id)
	if b.form != variable {
		b = &binding{id: id, cell: scheme.NewUninitializedCell()}
		e.replaceGlobal(b)
	}
	b.declared = true
	return b
}
func (s *scope) addTransformer(id scheme.Identifier) (*binding, error) {
	for _, b := range s.bindings {
		if b.id.SameContext(id) {
			return nil, ErrSyntax
		}
	}
	b := &binding{id: id, form: macroCall, declared: true, slot: -1}
	s.bindings = append(s.bindings, b)
	return b, nil
}
func (b *builder) defineTransformer(n *reader.Node, env *scope) error {
	xs, err := nodes(n)
	if err != nil {
		return err
	}
	if len(xs) != 3 {
		return syntax(n, "define-syntax requires identifier and transformer")
	}
	ident, err := id(xs[1])
	if err != nil {
		return err
	}
	var bind *binding
	if env != nil {
		bind, err = env.addTransformer(ident)
		if err != nil {
			return syntax(xs[1], "duplicate syntax definition")
		}
	} else {
		bind = b.engine.global(ident)
		if bind.declared {
			bind = &binding{id: ident}
			b.engine.replaceGlobal(bind)
		}
		bind.form = macroCall
		bind.declared = true
	}
	bind.transformer, err = b.parseTransformer(xs[2])
	return err
}
func (b *builder) localTransformers(n *reader.Node, args []*reader.Node, recursive, tail bool) error {
	if len(args) < 2 {
		return syntax(n, "syntax binding requires bindings and body")
	}
	inits, err := bindings(args[0])
	if err != nil {
		return err
	}
	env := &scope{parent: b.env}
	child := b.child(env, 0, false)
	var binds []*binding
	for _, init := range inits {
		ident, _ := id(init.name)
		bind, err := env.addTransformer(ident)
		if err != nil {
			return syntax(init.name, "duplicate syntax binding")
		}
		binds = append(binds, bind)
	}
	parser := b
	if recursive {
		parser = child
	}
	for i, init := range inits {
		binds[i].transformer, err = parser.parseTransformer(init.expr)
		if err != nil {
			return err
		}
	}
	if err := child.body(args[1:], n.Source()); err != nil {
		return err
	}
	if err := b.closure(child, n.Source()); err != nil {
		return err
	}
	b.emit(vm.Push, 0, 0, n.Source())
	op := vm.Call
	if tail {
		op = vm.TailCall
	}
	b.emit(op, 0, 0, n.Source())
	return nil
}
func (b *builder) expandHead(n *reader.Node) (*reader.Node, error) {
	for {
		if b.ctx == nil {
			b.ctx = context.Background()
		}
		if err := b.ctx.Err(); err != nil {
			return nil, err
		}
		kind := b.headForm(n)
		if kind == include || kind == includeCI || kind == condExpand {
			var err error
			n, err = b.sourceExpansion(n)
			if err != nil {
				return nil, err
			}
			continue
		}
		if kind != macroCall {
			return n, nil
		}
		ident, _ := id(n.Car)
		bind, _ := b.resolve(ident)
		if bind.transformer == nil {
			return nil, syntax(n, "syntax binding used before its transformer is initialized")
		}
		var err error
		n, err = bind.transformer.expand(b, n)
		if err != nil {
			return nil, err
		}
	}
}
func atom(v scheme.Value, pos scheme.Source) *reader.Node {
	return &reader.Node{Syntax: scheme.NewSyntax(v, pos)}
}
func pair(a, d *reader.Node, pos scheme.Source) *reader.Node {
	return &reader.Node{Syntax: scheme.NewSyntax(scheme.Cons(a.Datum(), d.Datum()), pos), Car: a, Cdr: d}
}
func list(xs []*reader.Node, tail *reader.Node, pos scheme.Source) *reader.Node {
	for i := len(xs) - 1; i >= 0; i-- {
		tail = pair(xs[i], tail, pos)
	}
	return tail
}

// Split keeps every original tail node so dotted pattern variables capture the
// source structure, including all remaining elements, without rebuilding it.
func split(n *reader.Node) (xs, tails []*reader.Node) {
	tails = append(tails, n)
	for n.Datum().Kind() == scheme.KindPair && n.Car != nil && n.Cdr != nil {
		xs = append(xs, n.Car)
		n = n.Cdr
		tails = append(tails, n)
	}
	return
}
