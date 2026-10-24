package core

import (
	"errors"
	"io"
	"os"
	"sort"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func (r *Runtime) interactionEnvironment() *evaluationEnvironment {
	if r.interaction != nil {
		return r.interaction
	}
	e := r.newEngine()
	e.strict = false
	copies := map[*binding]*scheme.Cell{}
	all := libraryExports{}
	for _, l := range r.modules {
		if l.definition == nil {
			for name, b := range l.exports {
				all[name] = b
			}
		}
	}
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		b := *all[name]
		b.origin = bindingIdentity(all[name])
		b.imported = false
		if b.cell != nil {
			v, err := b.cell.Get()
			if err != nil {
				panic(err)
			}
			b.cell = scheme.NewCell(v)
			copies[b.origin] = b.cell
		}
		e.globals = append(e.globals, &b)
	}
	r.interaction = &evaluationEnvironment{owner: r, engine: e, mutable: true, copies: copies}
	r.interactionValue = environmentValue(r.interaction)
	return r.interaction
}
func (r *Runtime) installInteraction() {
	e := r.bootstrap
	e.Bind("interaction-environment", vm.Native("interaction-environment", 0, 0, func(_ *vm.Context, _ []scheme.Value) (vm.Action, error) {
		r.interactionEnvironment()
		return vm.Return(r.interactionValue), nil
	}))
	e.Bind("load", vm.Native("load", 1, 2, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		path, ok := a[0].(*scheme.String)
		if !ok {
			return pairFailure(c, "load", a, scheme.ErrType)
		}
		var env *evaluationEnvironment
		if len(a) == 2 {
			var err error
			env, err = r.environmentFrom(a[1])
			if err != nil {
				return pairFailure(c, "load", a, err)
			}
		} else {
			env = r.interactionEnvironment()
		}
		data, err := os.ReadFile(path.Text())
		if err != nil {
			return portFailure(c, "load", a, ErrorFile, err)
		}
		rd, err := reader.New(path.Text(), string(data), e.symbols)
		if err != nil {
			return portFailure(c, "load", a, ErrorRead, err)
		}
		var ns []*reader.Node
		for {
			n, readErr := rd.Read()
			if errors.Is(readErr, io.EOF) {
				break
			}
			if readErr != nil {
				err = readErr
				break
			}
			ns = append(ns, n)
		}
		// The source snapshot, next index and terminal reader error are immutable
		// continuation state. Captured continuations can repeat the remaining forms.
		return r.loadNext(c, env, ns, 0, err, a)
	}))
}
func (r *Runtime) loadNext(c *vm.Context, env *evaluationEnvironment, ns []*reader.Node, index int, readErr error, args []scheme.Value) (vm.Action, error) {
	if err := c.Context().Err(); err != nil {
		return vm.Action{}, err
	}
	if index == len(ns) {
		if readErr != nil {
			return portFailure(c, "load", args, ErrorRead, readErr)
		}
		return vm.Return(scheme.Unspecified), nil
	}
	n := ns[index]
	if rawForm(n, "import") {
		if !env.mutable {
			return pairFailure(c, "load", args, scheme.ErrImmutable)
		}
		deps, err := r.interactionImports(c, env.engine, n)
		if err != nil {
			return pairFailure(c, "load", args, err)
		}
		next := vm.Native("load-next", 0, 0, func(ctx *vm.Context, _ []scheme.Value) (vm.Action, error) {
			return r.loadNext(ctx, env, ns, index+1, readErr, args)
		})
		return initializeLibraries(c, dependencyOrder(deps), 0, next)
	}
	code, err := env.engine.compileEvaluation(c.Context(), []*reader.Node{n})
	if err != nil {
		return pairFailure(c, "load", args, err)
	}
	return vm.Invoke(vm.Closure(code), nil, func(ctx *vm.Context, _ scheme.Results) (vm.Action, error) {
		return r.loadNext(ctx, env, ns, index+1, readErr, args)
	}), nil
}
func (r *Runtime) interactionImports(c *vm.Context, e *Engine, n *reader.Node) ([]*library, error) {
	// Resolve the whole declaration before changing persistent bindings. The
	// REPL permits later imports to replace names, unlike programs/libraries.
	temporary := r.newEngine()
	deps, err := r.applyImports(c.Context(), temporary, n)
	if err != nil {
		return nil, err
	}
	for _, b := range temporary.globals {
		if old := e.find(nil, e.globals, b.id); old != nil && bindingIdentity(old) == bindingIdentity(b) {
			continue
		}
		alias := *b
		if b.cell != nil {
			original := bindingIdentity(b)
			for _, standard := range r.bootstrap.primitives {
				if original != standard {
					continue
				}
				cell := r.interaction.copies[original]
				if cell == nil {
					value, err := b.cell.Get()
					if err != nil {
						return nil, err
					}
					cell = scheme.NewCell(value)
					r.interaction.copies[original] = cell
				}
				alias.cell = cell
				break
			}
		}
		e.replaceGlobal(&alias)
	}
	return deps, nil
}
