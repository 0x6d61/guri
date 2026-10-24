package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// The hidden record owns its handle; no session-wide table retains otherwise
// unreachable environments. The handle is never exposed as a Scheme procedure.
type evaluationEnvironment struct {
	owner   *Runtime
	engine  *Engine
	mutable bool
	copies  map[*binding]*scheme.Cell
}

func (*evaluationEnvironment) ProcedureCode() {}

var environmentType = func() *scheme.RecordType {
	s := scheme.NewSymbolTable()
	t, err := scheme.NewRecordType(s.Intern("environment"), []scheme.Field{{Name: s.Intern("handle")}})
	if err != nil {
		panic(err)
	}
	return t
}()

func environmentValue(e *evaluationEnvironment) *scheme.Record {
	v, err := scheme.NewRecord(environmentType, scheme.NewProcedure(e))
	if err != nil {
		panic(err)
	}
	return v
}
func (r *Runtime) environmentFrom(v scheme.Value) (*evaluationEnvironment, error) {
	rec, ok := v.(*scheme.Record)
	if !ok || rec.RecordType() != environmentType {
		return nil, scheme.ErrType
	}
	h, err := rec.Ref(0)
	if err != nil {
		return nil, err
	}
	e, ok := h.(*scheme.Procedure).Code().(*evaluationEnvironment)
	if !ok || e.owner != r {
		return nil, scheme.ErrType
	}
	return e, nil
}

// datumNode mirrors the graph rather than printing and rereading it. Quoted
// objects retain identity, sharing and cycles; code cycles are compiler errors.
func datumNode(ctx context.Context, value scheme.Value, pos scheme.Source) (*reader.Node, error) {
	seen := map[scheme.Value]*reader.Node{}
	pending := []scheme.Value{}
	node := func(v scheme.Value) *reader.Node {
		if n := seen[v]; n != nil {
			return n
		}
		n := atom(v, pos)
		seen[v] = n
		pending = append(pending, v)
		return n
	}
	root := node(value)
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		v := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		n := seen[v]
		switch x := v.(type) {
		case *scheme.Pair:
			n.Car = node(x.Car())
			n.Cdr = node(x.Cdr())
		case *scheme.Vector:
			for _, v := range x.Values() {
				n.Elements = append(n.Elements, node(v))
			}
		}
	}
	return root, nil
}
func dependencyOrder(dependencies []*library) []*library {
	var order []*library
	visited := map[*library]bool{}
	var visit func(*library)
	visit = func(l *library) {
		if visited[l] {
			return
		}
		visited[l] = true
		for _, d := range l.dependencies {
			visit(d)
		}
		order = append(order, l)
	}
	for _, l := range dependencies {
		visit(l)
	}
	return order
}
func (r *Runtime) installEvaluation() {
	e := r.bootstrap
	e.Bind("environment", vm.Native("environment", 0, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		engine := r.newEngine()
		engine.immutable = true
		sets := []*reader.Node{atom(e.symbols.Intern("import"), c.Source())}
		for _, v := range a {
			n, err := datumNode(c.Context(), v, c.Source())
			if err != nil {
				return vm.Action{}, err
			}
			sets = append(sets, n)
		}
		deps, err := r.applyImports(c.Context(), engine, list(sets, atom(scheme.Null, c.Source()), c.Source()))
		if err != nil {
			return pairFailure(c, "environment", a, err)
		}
		env := environmentValue(&evaluationEnvironment{owner: r, engine: engine})
		done := vm.Native("environment-result", 0, 0, func(_ *vm.Context, _ []scheme.Value) (vm.Action, error) { return vm.Return(env), nil })
		return initializeLibraries(c, dependencyOrder(deps), 0, done)
	}))
	e.Bind("eval", vm.Native("eval", 2, 2, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		env, err := r.environmentFrom(a[1])
		if err != nil {
			return pairFailure(c, "eval", a, err)
		}
		n, err := datumNode(c.Context(), a[0], c.Source())
		if err != nil {
			return vm.Action{}, err
		}
		code, err := env.engine.compileEvaluation(c.Context(), []*reader.Node{n})
		if err != nil {
			return pairFailure(c, "eval", a, err)
		}
		return vm.Invoke(vm.Closure(code), nil, nil), nil
	}))
	for _, name := range []string{"scheme-report-environment", "null-environment"} {
		e.Bind(name, vm.Native(name, 1, 1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			version, ok := a[0].(*scheme.Integer)
			if !ok || !version.BigInt().IsInt64() || version.BigInt().Int64() != 5 {
				return pairFailure(c, name, a, fmt.Errorf("unsupported report version; expected exact integer 5"))
			}
			engine := r.newEngine()
			engine.immutable = true
			for _, name := range strings.Fields(standardLibraryNames["(scheme r5rs)"]) {
				b := e.find(nil, e.primitives, scheme.NewIdentifier(e.symbols.Intern(name)))
				if b == nil {
					return pairFailure(c, name, a, scheme.ErrUninitialized)
				}
				engine.globals = append(engine.globals, b)
			}
			if name == "null-environment" {
				var syntax []*binding
				for _, b := range engine.globals {
					if b.form != variable {
						syntax = append(syntax, b)
					}
				}
				engine.globals = syntax
			}
			return vm.Return(environmentValue(&evaluationEnvironment{owner: r, engine: engine})), nil
		}))
	}
	// Interaction and load are installed here before the standard export snapshot.
	r.installInteraction()
	e.primitives = append([]*binding(nil), e.globals...)
}

// Compilation can introduce placeholders/transformers before discovering an
// error. Restore those changes so an unsuccessful REPL/eval does not poison
// subsequent input; Scheme effects from executed code are never rolled back.
func (e *Engine) compileEvaluation(ctx context.Context, ns []*reader.Node) (code *vm.Code, err error) {
	globals := append([]*binding(nil), e.globals...)
	originals := make([]binding, len(globals))
	for i, b := range globals {
		originals[i] = *b
	}
	defer func() {
		if err != nil {
			for i, b := range globals {
				*b = originals[i]
			}
			e.globals = globals
		}
	}()
	return e.compileNodes(ctx, "<eval>", ns)
}
