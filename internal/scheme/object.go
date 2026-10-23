package scheme

import "fmt"

// Code and resource handles deliberately have no Invoke or I/O operation.
// The evaluator/port implementations own those protocols and their lifetimes.
type ProcedureCode interface{ ProcedureCode() }
type Procedure struct{ code ProcedureCode }

func (*Procedure) Kind() Kind          { return KindProcedure }
func (v *Procedure) schemeValue() bool { return v != nil }
func NewProcedure(code ProcedureCode) *Procedure {
	if code == nil {
		panic("scheme: procedure needs code")
	}
	return &Procedure{code: code}
}
func (p *Procedure) Code() ProcedureCode { return p.code }

type PortResource interface{ PortResource() }
type Port struct{ resource PortResource }

func (*Port) Kind() Kind          { return KindPort }
func (v *Port) schemeValue() bool { return v != nil }
func NewPort(resource PortResource) *Port {
	if resource == nil {
		panic("scheme: port needs a resource")
	}
	return &Port{resource: resource}
}
func (p *Port) Resource() PortResource { return p.resource }

type Field struct {
	Name    *Symbol
	Mutable bool
}
type RecordType struct {
	name   *Symbol
	fields []Field
}

func NewRecordType(name *Symbol, fields []Field) (*RecordType, error) {
	if name == nil {
		return nil, ErrInvalidValue
	}
	seen := make(map[*Symbol]bool)
	for _, f := range fields {
		if f.Name == nil || seen[f.Name] {
			return nil, ErrInvalidValue
		}
		seen[f.Name] = true
	}
	return &RecordType{name: name, fields: append([]Field(nil), fields...)}, nil
}
func (t *RecordType) Name() *Symbol   { return t.name }
func (t *RecordType) Fields() []Field { return append([]Field(nil), t.fields...) }

type Record struct {
	typ    *RecordType
	values []Value
}

func (*Record) Kind() Kind          { return KindRecord }
func (v *Record) schemeValue() bool { return v != nil }
func NewRecord(typ *RecordType, values ...Value) (*Record, error) {
	if typ == nil {
		return nil, ErrType
	}
	if len(values) != len(typ.fields) {
		return nil, ErrValueCount
	}
	for _, v := range values {
		requireValue(v)
	}
	return &Record{typ: typ, values: append([]Value(nil), values...)}, nil
}
func (r *Record) RecordType() *RecordType { return r.typ }
func (r *Record) Ref(i int) (Value, error) {
	if i < 0 || i >= len(r.values) {
		return nil, ErrIndex
	}
	return r.values[i], nil
}
func (r *Record) Set(i int, value Value) error {
	requireValue(value)
	if i < 0 || i >= len(r.values) {
		return ErrIndex
	}
	if !r.typ.fields[i].Mutable {
		return ErrImmutable
	}
	r.values[i] = value
	return nil
}

// Results keeps zero or more return values separate from all Scheme collections.
type Results struct{ values []Value }

func NewResults(values ...Value) Results {
	for _, v := range values {
		requireValue(v)
	}
	return Results{values: append([]Value(nil), values...)}
}
func (r Results) Len() int        { return len(r.values) }
func (r Results) Values() []Value { return append([]Value(nil), r.values...) }
func (r Results) Single() (Value, error) {
	if len(r.values) != 1 {
		return nil, ErrValueCount
	}
	return r.values[0], nil
}

// Raised carries any Scheme object without requiring it to be an error object.
// Handler search and continuation transfer are evaluator responsibilities.
type Raised struct {
	object      Value
	continuable bool
	source      Source
}

func NewRaised(object Value, continuable bool, source Source) *Raised {
	requireValue(object)
	return &Raised{object: object, continuable: continuable, source: source}
}
func (r *Raised) Object() Value     { return r.object }
func (r *Raised) Continuable() bool { return r.continuable }
func (r *Raised) Source() Source    { return r.source }
func (r *Raised) Error() string {
	return fmt.Sprintf("raised Scheme %s at %s:%d:%d", r.object.Kind(), r.source.Filename, r.source.Line, r.source.Column)
}
