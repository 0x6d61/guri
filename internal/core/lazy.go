package core

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// A promise is a private record with a forwarding link, readiness flag and
// payload and evaluation mode. Forwarding shares one mutable memoization root across all aliases.
// No new public value type, implicit forcing, or AST evaluation is introduced.
var promiseType = func() *scheme.RecordType {
	symbols := scheme.NewSymbolTable()
	t, err := scheme.NewRecordType(symbols.Intern("promise"), []scheme.Field{
		{Name: symbols.Intern("parent"), Mutable: true},
		{Name: symbols.Intern("ready"), Mutable: true},
		{Name: symbols.Intern("payload"), Mutable: true},
		{Name: symbols.Intern("tail"), Mutable: true},
	})
	if err != nil {
		panic(err)
	}
	return t
}()

func asPromise(value scheme.Value) (*scheme.Record, bool) {
	p, ok := value.(*scheme.Record)
	return p, ok && p.RecordType() == promiseType
}
func newPromise(ready bool, payload scheme.Value, tail bool) (*scheme.Record, error) {
	return scheme.NewRecord(promiseType, scheme.Null, scheme.Boolean(ready), payload, scheme.Boolean(tail))
}
func promiseRoot(p *scheme.Record) (*scheme.Record, error) {
	// Path halving uses constant workspace. Merging below clears obsolete
	// thunk payloads; only the shared root owns the current computation.
	for {
		parent, err := p.Ref(0)
		if err != nil {
			return nil, err
		}
		if parent == scheme.Null {
			return p, nil
		}
		next, ok := asPromise(parent)
		if !ok {
			return nil, scheme.ErrType
		}
		grandparent, err := next.Ref(0)
		if err != nil {
			return nil, err
		}
		if grandparent != scheme.Null {
			if err := p.Set(0, grandparent); err != nil {
				return nil, err
			}
		}
		p = next
	}
}
func promiseContent(p *scheme.Record) (bool, scheme.Value, error) {
	ready, err := p.Ref(1)
	if err != nil {
		return false, nil, err
	}
	payload, err := p.Ref(2)
	return ready == scheme.True, payload, err
}
func forcePromise(c *vm.Context, p *scheme.Record) (vm.Action, error) {
	root, err := promiseRoot(p)
	if err != nil {
		return pairFailure(c, "force", []scheme.Value{p}, err)
	}
	ready, payload, err := promiseContent(root)
	if err != nil {
		return pairFailure(c, "force", []scheme.Value{p}, err)
	}
	if ready {
		return vm.Return(payload), nil
	}
	mode, err := root.Ref(3)
	if err != nil {
		return pairFailure(c, "force", []scheme.Value{p}, err)
	}
	tail := mode == scheme.True
	return vm.Invoke(payload, nil, func(ctx *vm.Context, results scheme.Results) (vm.Action, error) {
		// Evaluating the thunk may reenter force and complete or redirect p.
		// A completed inner force wins over any later outer result.
		root, err := promiseRoot(p)
		if err != nil {
			return pairFailure(ctx, "force", []scheme.Value{p}, err)
		}
		ready, value, err := promiseContent(root)
		if err != nil {
			return pairFailure(ctx, "force", []scheme.Value{p}, err)
		}
		if ready {
			return vm.Return(value), nil
		}
		value, err = results.Single()
		if err != nil {
			return pairFailure(ctx, "force", []scheme.Value{p}, err)
		}
		if !tail {
			if err := root.Set(1, scheme.True); err != nil {
				return vm.Action{}, err
			}
			if err := root.Set(2, value); err != nil {
				return vm.Action{}, err
			}
			return vm.Return(value), nil
		}
		target, ok := asPromise(value)
		if !ok {
			return pairFailure(ctx, "force", []scheme.Value{p}, scheme.ErrType)
		}
		target, err = promiseRoot(target)
		if err != nil {
			return pairFailure(ctx, "force", []scheme.Value{p}, err)
		}
		if root != target {
			done, content, err := promiseContent(target)
			if err != nil {
				return pairFailure(ctx, "force", []scheme.Value{p}, err)
			}
			if err := root.Set(1, scheme.Boolean(done)); err != nil {
				return vm.Action{}, err
			}
			if err := root.Set(2, content); err != nil {
				return vm.Action{}, err
			}
			mode, err := target.Ref(3)
			if err != nil {
				return vm.Action{}, err
			}
			if err := root.Set(3, mode); err != nil {
				return vm.Action{}, err
			}
			// Redirect the old root rather than just the returned promise. Aliases
			// created before an exception/escape must still share the cached result.
			if err := target.Set(0, root); err != nil {
				return vm.Action{}, err
			}
			if err := target.Set(1, scheme.False); err != nil {
				return vm.Action{}, err
			}
			if err := target.Set(2, scheme.Unspecified); err != nil {
				return vm.Action{}, err
			}
		}
		// Return another VM action: Go/VM frames do not accumulate per tail link.
		return forcePromise(ctx, p)
	}), nil
}
func (e *Engine) installLazy() {
	for _, tail := range []bool{false, true} {
		name := "make-value-promise"
		if tail {
			name = "make-lazy-promise"
		}
		e.controls[name] = vm.Native(name, 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
			p, err := newPromise(false, a[0], tail)
			if err != nil {
				return vm.Action{}, err
			}
			return vm.Return(p), nil
		})
	}
	e.Bind("force", vm.Native("force", 1, 1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		p, ok := asPromise(a[0])
		if !ok {
			return pairFailure(c, "force", a, scheme.ErrType)
		}
		return forcePromise(c, p)
	}))
	e.Bind("promise?", vm.Native("promise?", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		_, ok := asPromise(a[0])
		return vm.Return(scheme.Boolean(ok)), nil
	}))
	e.Bind("make-promise", vm.Native("make-promise", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		if _, ok := asPromise(a[0]); ok {
			return vm.Return(a[0]), nil
		}
		p, err := newPromise(true, a[0], false)
		if err != nil {
			return vm.Action{}, err
		}
		return vm.Return(p), nil
	}))
}
func (b *builder) lowerDelay(n *reader.Node, args []*reader.Node, tail bool) (*reader.Node, error) {
	if len(args) != 1 {
		return nil, syntax(n, "delay and delay-force require one expression")
	}
	l := b.lowerer(n.Source())
	thunk := l.form("lambda", atom(scheme.Null, l.pos), args[0])
	name := "make-value-promise"
	if tail {
		name = "make-lazy-promise"
	}
	return l.call(atom(b.engine.controls[name], l.pos), thunk), nil
}
