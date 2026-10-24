package core

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

type library struct {
	definition                                     *reader.Node
	exports                                        libraryExports
	dependencies                                   []*library
	code                                           *vm.Code
	preparing, prepared, initializing, initialized bool
	err                                            error
}

// Runtime is an internal library session. Programs start with empty globals;
// library cells, primitive parameters and the continuation-owning VM are shared.
// The root Go API and command-line interpreter are migrated separately.
type Runtime struct {
	bootstrap *Engine
	modules   map[string]*library
	standard  map[string]bool
	paths     []string
	context   context.Context
	running   bool
}

func NewRuntime(paths ...string) *Runtime {
	if len(paths) == 0 {
		paths = []string{"lib"}
	}
	r := &Runtime{bootstrap: NewCore(), modules: map[string]*library{}, standard: map[string]bool{}, paths: append([]string(nil), paths...)}
	for name, names := range standardLibraryNames {
		ns, err := reader.ReadAll("<standard-libraries>", name, r.bootstrap.symbols)
		if err != nil {
			panic(err)
		}
		key, _, err := parseLibraryName(ns[0])
		if err != nil {
			panic(err)
		}
		r.standard[key] = true
		exports := libraryExports{}
		complete := true
		for _, name := range strings.Fields(names) {
			b := r.bootstrap.find(nil, r.bootstrap.primitives, scheme.NewIdentifier(r.bootstrap.symbols.Intern(name)))
			if b == nil || !b.declared {
				complete = false
				break
			}
			exports[name] = b
		}
		if complete {
			r.modules[key] = &library{exports: exports, prepared: true, initialized: true}
		}
	}
	return r
}
func (r *Runtime) newEngine() *Engine {
	e := &Engine{strict: true, symbols: r.bootstrap.symbols, machine: r.bootstrap.machine, controls: r.bootstrap.controls, primitives: r.bootstrap.primitives, features: r.bootstrap.features, loader: fileSourceLoader}
	e.libraries = r.available
	return e
}
func rawForm(n *reader.Node, name string) bool {
	return n.Car != nil && n.Datum().Kind() == scheme.KindPair && n.Car.Datum().Kind() == scheme.KindSymbol && n.Car.Datum().(*scheme.Symbol).Name() == name
}

// Register indexes definitions without executing library bodies.
func (r *Runtime) Register(filename, source string) error {
	if r.running {
		return vm.ErrReentrant
	}
	return r.register(filename, source)
}
func (r *Runtime) register(filename, source string) error {
	ns, err := reader.ReadAll(filename, source, r.bootstrap.symbols)
	if err != nil {
		return err
	}
	pending := map[string]*library{}
	for _, n := range ns {
		if !rawForm(n, "define-library") {
			return syntax(n, "library source requires define-library declarations")
		}
		xs, err := nodes(n)
		if err != nil {
			return err
		}
		if len(xs) < 2 {
			return syntax(n, "define-library requires a name")
		}
		key, _, err := parseLibraryName(xs[1])
		if err != nil {
			return err
		}
		if _, found := r.modules[key]; found || r.standard[key] {
			return syntax(n, "duplicate or reserved library name")
		}
		if _, found := pending[key]; found {
			return syntax(n, "duplicate library name")
		}
		pending[key] = &library{definition: n}
	}
	for key, l := range pending {
		r.modules[key] = l
	}
	return nil
}
func (r *Runtime) imported(ctx context.Context, n *reader.Node) (libraryExports, *library, error) {
	key, _, err := parseLibraryName(n)
	if err != nil {
		return nil, nil, err
	}
	l, err := r.locate(ctx, n)
	if err != nil {
		return nil, nil, err
	}
	if l.err != nil {
		return nil, nil, l.err
	}
	if l.preparing {
		return nil, nil, syntax(n, "cyclic library dependency: "+key)
	}
	if !l.prepared {
		l.preparing = true
		err = r.prepare(ctx, l)
		l.preparing = false
		if err != nil {
			l.err = err
			return nil, nil, err
		}
		l.prepared = true
	}
	return l.exports, l, nil
}
func (r *Runtime) applyImports(ctx context.Context, e *Engine, n *reader.Node) ([]*library, error) {
	xs, err := nodes(n)
	if err != nil {
		return nil, err
	}
	var dependencies []*library
	for _, set := range xs[1:] {
		exports, err := resolveImportSet(set, func(name *reader.Node) (libraryExports, error) {
			exports, l, err := r.imported(ctx, name)
			if err == nil {
				dependencies = append(dependencies, l)
			}
			return exports, err
		})
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(exports))
		for name := range exports {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			original := exports[name]
			id := scheme.NewIdentifier(e.symbols.Intern(name))
			if old := e.find(nil, e.globals, id); old != nil {
				if bindingIdentity(old) != bindingIdentity(original) {
					return nil, syntax(set, "conflicting imported identifier: "+name)
				}
				continue
			}
			alias := *original
			alias.id = id
			alias.origin = bindingIdentity(original)
			alias.imported = true
			e.globals = append(e.globals, &alias)
		}
	}
	return dependencies, nil
}
func (r *Runtime) prepare(ctx context.Context, l *library) error {
	xs, err := nodes(l.definition)
	if err != nil {
		return err
	}
	e := r.newEngine()
	declarations, err := r.declarations(ctx, e, xs[2:], map[string]bool{})
	if err != nil {
		return err
	}
	var body []*reader.Node
	var exports []*reader.Node
	for _, n := range declarations {
		parts, err := nodes(n)
		if err != nil {
			return err
		}
		switch parts[0].Datum().(*scheme.Symbol).Name() {
		case "import":
			deps, err := r.applyImports(ctx, e, n)
			if err != nil {
				return err
			}
			l.dependencies = append(l.dependencies, deps...)
		case "export":
			exports = append(exports, parts[1:]...)
		case "begin":
			body = append(body, parts[1:]...)
		case "include", "include-ci":
			included, err := r.included(ctx, n, parts[1:], rawForm(n, "include-ci"))
			if err != nil {
				return err
			}
			body = append(body, included...)
		}
	}
	l.code, err = e.compileNodes(ctx, l.definition.Source().Filename, body)
	if err != nil {
		return err
	}
	l.exports = libraryExports{}
	for _, spec := range exports {
		internal, external := spec, spec
		if spec.Datum().Kind() == scheme.KindPair {
			pair, err := nodes(spec)
			if err != nil {
				return err
			}
			if len(pair) != 3 || !symbolNamed(pair[0], "rename") {
				return syntax(spec, "export rename needs two identifiers")
			}
			internal, external = pair[1], pair[2]
		}
		from, err := importIdentifier(internal)
		if err != nil {
			return err
		}
		to, err := importIdentifier(external)
		if err != nil {
			return err
		}
		b := e.find(nil, e.globals, scheme.NewIdentifier(e.symbols.Intern(from)))
		if b == nil || !b.declared {
			return syntax(internal, "exported identifier is not defined: "+from)
		}
		if err := addExport(l.exports, to, b, spec); err != nil {
			return err
		}
	}
	return nil
}
func symbolNamed(n *reader.Node, name string) bool {
	s, ok := n.Datum().(*scheme.Symbol)
	return ok && s.Name() == name
}
func (r *Runtime) Run(ctx context.Context, filename, source string) (result scheme.Results, err error) {
	if r.running {
		return result, vm.ErrReentrant
	}
	r.running = true
	r.context = ctx
	defer func() {
		r.running = false
		r.context = nil
		for _, l := range r.modules {
			if l.initializing {
				l.initializing = false
				l.err = err
				if l.err == nil {
					l.err = fmt.Errorf("library initializer escaped before returning")
				}
			}
		}
	}()
	ns, err := reader.ReadAll(filename, source, r.bootstrap.symbols)
	if err != nil {
		return result, err
	}
	e := r.newEngine()
	var dependencies []*library
	count := 0
	for count < len(ns) && rawForm(ns[count], "import") {
		deps, err := r.applyImports(ctx, e, ns[count])
		if err != nil {
			return result, err
		}
		dependencies = append(dependencies, deps...)
		count++
	}
	if count == 0 {
		return result, &Error{Source: scheme.Source{Filename: filename, Line: 1, Column: 1}, Cause: fmt.Errorf("%w: program requires an initial import declaration", ErrSyntax)}
	}
	code, err := e.compileNodes(ctx, filename, ns[count:])
	if err != nil {
		return result, err
	}
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
	main := vm.Closure(code)
	entry := vm.Native("initialize-libraries", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) {
		return initializeLibraries(c, order, 0, main)
	})
	return r.bootstrap.machine.Run(ctx, entry)
}
func initializeLibraries(c *vm.Context, order []*library, index int, main *scheme.Procedure) (vm.Action, error) {
	for index < len(order) {
		l := order[index]
		if l.err != nil {
			return vm.Action{}, l.err
		}
		if l.initialized {
			index++
			continue
		}
		l.initializing = true
		next := index + 1
		return vm.Invoke(vm.Closure(l.code), nil, func(ctx *vm.Context, _ scheme.Results) (vm.Action, error) {
			l.initializing = false
			l.initialized = true
			l.err = nil
			return initializeLibraries(ctx, order, next, main)
		}), nil
	}
	return vm.Invoke(main, nil, nil), nil
}
