package core

import (
	"errors"
	"io"
	"math"

	"github.com/0x6d61/guri/internal/number"
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

type jsonLibrary struct {
	symbols           *scheme.SymbolTable
	depth, characters *parameter
	input, output     *parameter
}

func jsonFailure(c *vm.Context, err error) (vm.Action, error) {
	object, e := NewErrorObject(ErrorJSON, "json: "+err.Error())
	if e != nil {
		return vm.Action{}, e
	}
	return Raise(c, object, false)
}
func isJSONError(v scheme.Value) bool {
	if !isErrorObject(v) {
		return false
	}
	kind, err := v.(*scheme.Record).Ref(2)
	return err == nil && kind.(*scheme.Integer).BigInt().Uint64()&uint64(ErrorJSON) != 0
}
func (r *Runtime) installJSON() {
	j := &jsonLibrary{symbols: r.bootstrap.symbols, depth: &parameter{initial: scheme.NewFloat(math.Inf(1))}, characters: &parameter{initial: scheme.NewFloat(math.Inf(1))}}
	for name, target := range map[string]**parameter{"current-input-port": &j.input, "current-output-port": &j.output} {
		b := r.bootstrap.find(nil, r.bootstrap.primitives, scheme.NewIdentifier(j.symbols.Intern(name)))
		v, err := b.cell.Get()
		if err != nil {
			panic(err)
		}
		*target = v.(*scheme.Procedure).Code().(*parameter)
	}
	exports := libraryExports{}
	bind := func(name string, value scheme.Value) {
		exports[name] = &binding{id: scheme.NewIdentifier(j.symbols.Intern(name)), form: variable, declared: true, cell: scheme.NewCell(value)}
	}
	for name, p := range map[string]*parameter{"json-nesting-depth-limit": j.depth, "json-number-of-character-limit": j.characters} {
		p.converter = vm.Native(name, 0, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if len(a) != 1 {
				return jsonFailure(c, vm.ErrArity)
			}
			if _, err := jsonLimit(a[0]); err != nil {
				return jsonFailure(c, err)
			}
			return vm.Return(a[0]), nil
		})
		bind(name, scheme.NewProcedure(p))
	}
	for name, arity := range map[string]pairArity{
		"json-error?": {1, 1}, "json-error-reason": {1, 1}, "json-null?": {1, 1},
		"json-generator": {0, 1}, "json-fold": {6, 7}, "json-read": {0, 1},
		"json-lines-read": {0, 1}, "json-sequence-read": {0, 1},
		"json-accumulator": {1, 1}, "json-write": {1, 2},
	} {
		bind(name, vm.Native(name, 0, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if len(a) < arity.min || len(a) > arity.max {
				return jsonFailure(c, vm.ErrArity)
			}
			switch name {
			case "json-error?":
				return vm.Return(scheme.Boolean(isJSONError(a[0]))), nil
			case "json-error-reason":
				if !isJSONError(a[0]) {
					return jsonFailure(c, scheme.ErrType)
				}
				v, err := a[0].(*scheme.Record).Ref(0)
				return vm.Return(v), err
			case "json-null?":
				s, ok := a[0].(*scheme.Symbol)
				return vm.Return(scheme.Boolean(ok && s.Name() == "null")), nil
			case "json-generator", "json-read", "json-fold":
				offset := 0
				if name == "json-fold" {
					offset = 6
					for _, v := range a[:5] {
						if _, ok := v.(*scheme.Procedure); !ok {
							return jsonFailure(c, scheme.ErrType)
						}
					}
				}
				input := j.input.value(c)
				if len(a) > offset {
					input = a[offset]
				}
				g, err := j.generator(c, input)
				if err != nil {
					return jsonFailure(c, err)
				}
				if name == "json-generator" {
					return vm.Return(g), nil
				}
				if name == "json-read" {
					return j.read(c, g, nil)
				}
				return j.fold(c, g, a[:5], a[5], nil)
			case "json-lines-read", "json-sequence-read":
				return j.sequence(c, name, a)
			case "json-accumulator", "json-write":
				return j.write(c, name, a)
			}
			panic("unhandled JSON procedure")
		}))
	}
	ns, err := reader.ReadAll("<srfi-libraries>", "(srfi 180)", j.symbols)
	if err != nil {
		panic(err)
	}
	key, _, err := parseLibraryName(ns[0])
	if err != nil {
		panic(err)
	}
	r.modules[key] = &library{exports: exports, prepared: true, initialized: true}
}
func jsonLimit(v scheme.Value) (scheme.Real, error) {
	real, ok := v.(scheme.Real)
	if !ok {
		return nil, scheme.ErrType
	}
	cmp, ordered := number.Compare(real, scheme.NewInteger(0))
	if !ordered || cmp < 0 {
		return nil, errors.New("JSON limit must be a nonnegative real number")
	}
	return real, nil
}

// jsonInput borrows the source. Neither EOF nor parsing errors close it.
type jsonInput struct {
	port  *ports.Resource
	chars *scheme.Procedure
}

func newJSONInput(value scheme.Value) (*jsonInput, error) {
	if p, ok := value.(*scheme.Procedure); ok {
		return &jsonInput{chars: p}, nil
	}
	p, err := ports.From(value)
	if err != nil {
		return nil, err
	}
	if err := checkTextPort(p, true); err != nil {
		return nil, err
	}
	return &jsonInput{port: p}, nil
}
func (j *jsonLibrary) generator(c *vm.Context, input scheme.Value) (*scheme.Procedure, error) {
	source, err := newJSONInput(input)
	if err != nil {
		return nil, err
	}
	depth, err := jsonLimit(j.depth.value(c))
	if err != nil {
		return nil, err
	}
	characters, err := jsonLimit(j.characters.value(c))
	if err != nil {
		return nil, err
	}
	p := &jsonParser{symbols: j.symbols, depthLimit: depth, characterLimit: characters}
	return vm.Native("JSON event generator", 0, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		if len(a) != 0 {
			return jsonFailure(c, vm.ErrArity)
		}
		return jsonNext(c, p, source)
	}), nil
}
func jsonNext(c *vm.Context, p *jsonParser, input *jsonInput) (vm.Action, error) {
	for {
		if err := c.Context().Err(); err != nil {
			return vm.Action{}, err
		}
		event, err := p.next()
		if err == nil {
			return vm.Return(event), nil
		}
		if !errors.Is(err, jsonNeedCharacter) {
			return jsonFailure(c, err)
		}
		if input.port != nil {
			ch, err := input.port.ReadChar(false)
			if err != nil && !errors.Is(err, io.EOF) {
				return jsonFailure(c, err)
			}
			if err = p.feed(ch, errors.Is(err, io.EOF)); err != nil {
				return jsonFailure(c, err)
			}
			continue
		}
		return vm.Invoke(input.chars, nil, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
			v, err := r.Single()
			if err != nil {
				return jsonFailure(c, err)
			}
			var ch rune
			if v != scheme.EOF {
				char, ok := v.(scheme.Character)
				if !ok {
					return jsonFailure(c, scheme.ErrType)
				}
				ch = char.Rune()
			}
			if err = p.feed(ch, v == scheme.EOF); err != nil {
				return jsonFailure(c, err)
			}
			return jsonNext(c, p, input)
		}), nil
	}
}

// The builder uses persistent reverse lists: appending a value costs O(1),
// and VM continuations never capture a slice that a later callback overwrites.
type jsonReadValues struct {
	value scheme.Value
	next  *jsonReadValues
}
type jsonReadFrame struct {
	object bool
	values *jsonReadValues
	count  int
	parent *jsonReadFrame
}

func (f *jsonReadFrame) add(v scheme.Value) *jsonReadFrame {
	return &jsonReadFrame{object: f.object, values: &jsonReadValues{value: v, next: f.values}, count: f.count + 1, parent: f.parent}
}
func (j *jsonLibrary) readValue(f *jsonReadFrame) scheme.Value {
	values := make([]scheme.Value, f.count)
	node := f.values
	for i := len(values) - 1; i >= 0; i-- {
		values[i] = node.value
		node = node.next
	}
	if !f.object {
		return scheme.NewVector(values...)
	}
	pairs := make([]scheme.Value, 0, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		pairs = append(pairs, scheme.Cons(j.symbols.Intern(values[i].(*scheme.String).Text()), values[i+1]))
	}
	return scheme.List(pairs...)
}
func (j *jsonLibrary) read(c *vm.Context, g scheme.Value, frame *jsonReadFrame) (vm.Action, error) {
	return vm.Invoke(g, nil, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
		value, err := r.Single()
		if err != nil {
			return jsonFailure(c, err)
		}
		if value == scheme.EOF {
			return vm.Return(value), nil
		}
		current := frame
		if s, ok := value.(*scheme.Symbol); ok {
			switch s.Name() {
			case "array-start", "object-start":
				return j.read(c, g, &jsonReadFrame{object: s.Name() == "object-start", parent: current})
			case "array-end", "object-end":
				value = j.readValue(current)
				current = current.parent
			}
		}
		if current == nil {
			return vm.Return(value), nil
		}
		return j.read(c, g, current.add(value))
	}), nil
}

type jsonFoldFrame struct {
	seed   scheme.Value
	object bool
	parent *jsonFoldFrame
}

func (j *jsonLibrary) fold(c *vm.Context, g scheme.Value, procs []scheme.Value, seed scheme.Value, frame *jsonFoldFrame) (vm.Action, error) {
	return vm.Invoke(g, nil, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
		value, err := r.Single()
		if err != nil {
			return jsonFailure(c, err)
		}
		if value == scheme.EOF {
			return vm.Return(seed), nil
		}
		if s, ok := value.(*scheme.Symbol); ok {
			switch s.Name() {
			case "array-start", "object-start":
				object := s.Name() == "object-start"
				index := 1
				if object {
					index = 3
				}
				return vm.Invoke(procs[index], []scheme.Value{seed}, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
					next, err := r.Single()
					if err != nil {
						return jsonFailure(c, err)
					}
					return j.fold(c, g, procs, next, &jsonFoldFrame{seed: seed, object: object, parent: frame})
				}), nil
			case "array-end", "object-end":
				index := 2
				if frame.object {
					index = 4
				}
				return vm.Invoke(procs[index], []scheme.Value{seed}, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
					value, err := r.Single()
					if err != nil {
						return jsonFailure(c, err)
					}
					return j.foldValue(c, g, procs, value, frame.seed, frame.parent)
				}), nil
			}
		}
		return j.foldValue(c, g, procs, value, seed, frame)
	}), nil
}
func (j *jsonLibrary) foldValue(c *vm.Context, g scheme.Value, procs []scheme.Value, value, seed scheme.Value, frame *jsonFoldFrame) (vm.Action, error) {
	return vm.Invoke(procs[0], []scheme.Value{value, seed}, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
		next, err := r.Single()
		if err != nil {
			return jsonFailure(c, err)
		}
		return j.fold(c, g, procs, next, frame)
	}), nil
}
