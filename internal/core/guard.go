package core

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func (b *builder) lowerGuard(n *reader.Node, args []*reader.Node) (*reader.Node, error) {
	if len(args) < 2 {
		return nil, syntax(n, "guard requires specification and body")
	}
	spec, err := nodes(args[0])
	if err != nil {
		return nil, err
	}
	if len(spec) == 0 {
		return nil, syntax(args[0], "guard requires a variable")
	}
	if _, err := id(spec[0]); err != nil {
		return nil, err
	}
	l := b.lowerer(n.Source())
	guardK, handlerK, condition, results := l.name("guard-return"), l.name("handler-return"), l.name("guard-condition"), l.name("guard-values")
	empty := atom(scheme.Null, l.pos)
	formals := func(xs ...*reader.Node) *reader.Node { return list(xs, empty, l.pos) }
	thunk := func(body *reader.Node) *reader.Node { return l.form("lambda", empty, body) }
	reraise := l.control("raise-continuable", condition)
	fallback := l.call(handlerK, thunk(reraise))
	ident, _ := id(spec[0])
	resolver := *b
	resolver.env = &scope{parent: b.env, bindings: []*binding{{id: ident, form: variable, declared: true}}}
	clauses, err := resolver.condClausesDefault(l, spec[1:], fallback)
	if err != nil {
		return nil, err
	}
	clauseThunk := thunk(l.bind(spec[0], condition, clauses))
	handoff := l.control("call/cc", l.form("lambda", formals(handlerK), l.call(guardK, clauseThunk)))
	handler := l.form("lambda", formals(condition), l.call(handoff))
	producer := l.form("lambda", append([]*reader.Node{empty}, args[1:]...)...)
	consumer := l.form("lambda", results, l.call(guardK, thunk(l.control("apply", atom(b.engine.controls["values"], l.pos), results))))
	body := l.control("call-with-values", producer, consumer)
	guarded := l.control("with-exception-handler", handler, thunk(body))
	return l.call(l.control("call/cc", l.form("lambda", formals(guardK), guarded))), nil
}
