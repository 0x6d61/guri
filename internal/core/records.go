package core

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

type recordField struct {
	id                scheme.Identifier
	accessor, mutator *reader.Node
}

func (b *builder) lowerRecord(n *reader.Node) (*reader.Node, error) {
	xs, err := nodes(n)
	if err != nil {
		return nil, err
	}
	if len(xs) < 4 {
		return nil, syntax(n, "record definition requires name, constructor and predicate")
	}
	name, err := id(xs[1])
	if err != nil {
		return nil, err
	}
	constructor, err := nodes(xs[2])
	if err != nil {
		return nil, err
	}
	if len(constructor) == 0 {
		return nil, syntax(xs[2], "constructor requires a name")
	}
	bindings := []*reader.Node{xs[1], constructor[0], xs[3]}
	var fields []recordField
	for _, node := range xs[4:] {
		field, err := nodes(node)
		if err != nil {
			return nil, err
		}
		if len(field) < 2 || len(field) > 3 {
			return nil, syntax(node, "field requires name, accessor and optional mutator")
		}
		ident, err := id(field[0])
		if err != nil {
			return nil, err
		}
		for _, old := range fields {
			if old.id.SameContext(ident) {
				return nil, syntax(field[0], "duplicate record field")
			}
		}
		f := recordField{id: ident, accessor: field[1]}
		bindings = append(bindings, field[1])
		if len(field) == 3 {
			f.mutator = field[2]
			bindings = append(bindings, field[2])
		}
		fields = append(fields, f)
	}
	var identifiers []scheme.Identifier
	head, _ := id(xs[0])
	for _, node := range bindings {
		ident, err := id(node)
		if err != nil {
			return nil, err
		}
		if ident.SameContext(head) {
			return nil, syntax(node, "record definition changes its own syntax binding")
		}
		for _, old := range identifiers {
			if old.SameContext(ident) {
				return nil, syntax(node, "duplicate record binding")
			}
		}
		identifiers = append(identifiers, ident)
	}
	var arguments []int
	for _, node := range constructor[1:] {
		ident, err := id(node)
		if err != nil {
			return nil, err
		}
		index := -1
		for i, f := range fields {
			if f.id.SameContext(ident) {
				index = i
				break
			}
		}
		if index < 0 {
			return nil, syntax(node, "constructor refers to an unknown field")
		}
		for _, old := range arguments {
			if old == index {
				return nil, syntax(node, "duplicate constructor field")
			}
		}
		arguments = append(arguments, index)
	}
	metadata := make([]scheme.Field, len(fields))
	accessors, mutators := make([]string, len(fields)), make([]string, len(fields))
	typeName := name.Symbol()
	makeName := constructor[0].Datum().(*scheme.Symbol).Name()
	predName := xs[3].Datum().(*scheme.Symbol).Name()
	for i, f := range fields {
		// Backend field tags are private: distinct hygienic identifiers may have
		// the same spelling, and public Symbol equality does not carry scopes.
		accessors[i] = f.accessor.Datum().(*scheme.Symbol).Name()
		if f.mutator != nil {
			mutators[i] = f.mutator.Datum().(*scheme.Symbol).Name()
		}
		metadata[i] = scheme.Field{Name: scheme.NewSymbolTable().Intern(f.id.Symbol().Name()), Mutable: f.mutator != nil}
	}
	factory := vm.Native("record-type-factory", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) {
		typ, err := scheme.NewRecordType(typeName, metadata)
		if err != nil {
			return vm.Action{}, err
		}
		makeRecord := vm.Native(makeName, len(arguments), len(arguments), func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			values := make([]scheme.Value, len(metadata))
			for i := range values {
				values[i] = scheme.Unspecified
			}
			for i, index := range arguments {
				values[index] = a[i]
			}
			record, err := scheme.NewRecord(typ, values...)
			if err != nil {
				return pairFailure(c, makeName, a, err)
			}
			return vm.Return(record), nil
		})
		predicate := vm.Native(predName, 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
			record, ok := a[0].(*scheme.Record)
			return vm.Return(scheme.Boolean(ok && record.RecordType() == typ)), nil
		})
		functions := []scheme.Value{makeRecord, predicate}
		for i, field := range metadata {
			accessorName := accessors[i]
			functions = append(functions, vm.Native(accessorName, 1, 1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
				record, ok := a[0].(*scheme.Record)
				if !ok || record.RecordType() != typ {
					return pairFailure(c, accessorName, a, scheme.ErrType)
				}
				value, err := record.Ref(i)
				if err != nil {
					return pairFailure(c, accessorName, a, err)
				}
				return vm.Return(value), nil
			}))
			if field.Mutable {
				mutatorName := mutators[i]
				functions = append(functions, vm.Native(mutatorName, 2, 2, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
					record, ok := a[0].(*scheme.Record)
					if !ok || record.RecordType() != typ {
						return pairFailure(c, mutatorName, a, scheme.ErrType)
					}
					if err := record.Set(i, a[1]); err != nil {
						return pairFailure(c, mutatorName, a, err)
					}
					return vm.Return(scheme.Unspecified), nil
				}))
			}
		}
		bundle := scheme.NewVector(functions...)
		scheme.FreezeLiteral(bundle)
		return vm.Return(bundle), nil
	})
	l := b.lowerer(n.Source())
	bundle := l.name("record-bindings")
	// R7RS permits a purely syntactic type-name representation. Runtime type
	// identity is generated by the factory, not by this syntax placeholder.
	definitions := []*reader.Node{l.form("define-syntax", xs[1], l.form("syntax-rules", atom(scheme.Null, l.pos))), l.form("define", bundle, l.call(atom(factory, l.pos)))}
	outputs := append([]*reader.Node{constructor[0], xs[3]}, bindings[3:]...)
	for index, output := range outputs {
		getter := vm.Native("record-procedure", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
			vector, ok := a[0].(*scheme.Vector)
			if !ok {
				return vm.Action{}, scheme.ErrType
			}
			value, err := vector.Ref(index)
			if err != nil {
				return vm.Action{}, err
			}
			return vm.Return(value), nil
		})
		definitions = append(definitions, l.form("define", output, l.call(atom(getter, l.pos), bundle)))
	}
	return l.form("begin", definitions...), nil
}
