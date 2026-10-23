package scheme_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/0x6d61/guri/internal/scheme"
)

type code struct{ cell *scheme.Cell }

func (*code) ProcedureCode() {}

type portResource struct{ state bool }

func (*portResource) PortResource() {}

func TestRecordsAreGenerativeAndRespectFieldMutability(t *testing.T) {
	table := scheme.NewSymbolTable()
	fields := []scheme.Field{{Name: table.Intern("value"), Mutable: true}, {Name: table.Intern("fixed")}}
	typ, err := scheme.NewRecordType(table.Intern("box"), fields)
	if err != nil {
		t.Fatal(err)
	}
	other, err := scheme.NewRecordType(table.Intern("box"), fields)
	if err != nil {
		t.Fatal(err)
	}
	fields[0].Mutable = false
	exported := typ.Fields()
	exported[0].Mutable = false
	values := []scheme.Value{scheme.NewInteger(1), scheme.True}
	r, err := scheme.NewRecord(typ, values...)
	if err != nil {
		t.Fatal(err)
	}
	values[0] = scheme.False
	if r.RecordType() == other {
		t.Fatal("record types not generative")
	}
	if err := r.Set(0, r); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	value, err := r.Ref(0)
	if err != nil || value != r {
		t.Fatal("record sharing/cycle")
	}
	if err := r.Set(1, scheme.False); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal("immutable field updated")
	}
	if _, err := scheme.NewRecord(typ, scheme.True); !errors.Is(err, scheme.ErrValueCount) {
		t.Fatal("record constructor count")
	}
	if _, err := scheme.NewRecordType(table.Intern("bad"), []scheme.Field{{Name: table.Intern("x")}, {Name: table.Intern("x")}}); err == nil {
		t.Fatal("duplicate record field")
	}
}

func TestProcedureAndPortIdentitiesRetainTheirPayload(t *testing.T) {
	shared := scheme.NewCell(scheme.True)
	body := &code{cell: shared}
	p, q := scheme.NewProcedure(body), scheme.NewProcedure(body)
	if p == q || p.Kind() != scheme.KindProcedure {
		t.Fatal("procedure identity")
	}
	resource := &portResource{state: true}
	port := scheme.NewPort(resource)
	runtime.GC()
	p.Code().(*code).cell.Set(scheme.False)
	got, err := shared.Get()
	if err != nil || got != scheme.False || !port.Resource().(*portResource).state {
		t.Fatal("captured payload lost")
	}
	if port.Kind() != scheme.KindPort {
		t.Fatal("port type boundary")
	}
}

func TestMultipleValuesAndRaisedPayloadAreNotOrdinaryValues(t *testing.T) {
	input := []scheme.Value{scheme.True, scheme.Null}
	results := scheme.NewResults(input...)
	input[0] = scheme.False
	exposed := results.Values()
	exposed[0] = scheme.False
	if results.Len() != 2 || results.Values()[0] != scheme.True {
		t.Fatal("multiple value storage exposed")
	}
	if _, err := results.Single(); !errors.Is(err, scheme.ErrValueCount) {
		t.Fatal("multiple values coerced to one")
	}
	if _, err := scheme.NewResults().Single(); !errors.Is(err, scheme.ErrValueCount) {
		t.Fatal("zero values became nil")
	}
	if v, err := scheme.NewResults(scheme.Null).Single(); err != nil || v != scheme.Null {
		t.Fatal("single null result")
	}
	if _, ok := any(results).(scheme.Value); ok {
		t.Fatal("multiple values became a Scheme collection")
	}
	source := scheme.Source{Filename: "fail.scm", Line: 2, Column: 4}
	raised := scheme.NewRaised(scheme.NewInteger(7), true, source)
	if raised.Object().(*scheme.Integer).String() != "7" || !raised.Continuable() || raised.Source() != source {
		t.Fatal("raised arbitrary value/source lost")
	}
}
