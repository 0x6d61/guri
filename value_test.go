package guri

import (
	"math"
	"strings"
	"testing"
)

func TestEqualValues(t *testing.T) {
	m1, m2 := NewMap(), NewMap()
	if err := m1.Set(KeywordValue("a"), int64(1)); err != nil {
		t.Fatal(err)
	}
	if err := m1.Set(KeywordValue("b"), true); err != nil {
		t.Fatal(err)
	}
	if err := m2.Set(KeywordValue("b"), true); err != nil {
		t.Fatal(err)
	}
	if err := m2.Set(KeywordValue("a"), float64(1)); err != nil {
		t.Fatal(err)
	}
	f := &FunctionValue{}
	for _, tt := range []struct {
		a, b  Value
		equal bool
	}{
		{nil, nil, true}, {false, nil, false}, {int64(1), float64(1), true},
		{int64(1), 1.5, false}, {int64(9007199254740993), float64(9007199254740992), false},
		{int64(math.MaxInt64), float64(math.MaxInt64), false},
		{int64(math.MinInt64), float64(math.MinInt64), true},
		{int64(1), "1", false}, {SymbolValue("a"), KeywordValue("a"), false},
		{NewList(int64(1)), VectorValue{int64(1)}, false},
		{NewList(int64(1)), NewList(float64(1)), true},
		{VectorValue{int64(1)}, VectorValue{float64(1)}, true},
		{m1, m2, true}, {f, f, true}, {f, &FunctionValue{}, false},
	} {
		if got := Equal(tt.a, tt.b); got != tt.equal {
			t.Errorf("Equal(%#v,%#v)=%v", tt.a, tt.b, got)
		}
	}
}

func TestCollectionsAndTruth(t *testing.T) {
	v := VectorValue{int64(1), int64(2)}
	alias := v
	alias[0] = int64(9)
	if v[0] != int64(9) {
		t.Fatal("vector assignment lost sharing")
	}
	m := NewMap()
	list := NewList(m)
	values := list.Values()
	values[0] = nil
	if list.Values()[0] != m {
		t.Fatal("list structure changed through exported copy")
	}
	if err := m.Set("a", v); err != nil {
		t.Fatal(err)
	}
	if list.Values()[0].(*MapValue).entries["a"].(VectorValue)[0] != int64(9) {
		t.Fatal("nested mutable value copied")
	}
	for _, x := range []Value{int64(0), "", VectorValue{}, NewMap(), NewList()} {
		if !Truthy(x) {
			t.Errorf("%#v is false", x)
		}
	}
	if Truthy(nil) || Truthy(false) {
		t.Fatal("nil and false must be false")
	}
}

func TestMapKeys(t *testing.T) {
	m := NewMap()
	for _, key := range []Value{nil, false, int64(1), float64(1), "a", SymbolValue("a"), KeywordValue("a")} {
		if err := m.Set(key, true); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.entries) != 7 {
		t.Fatal("map key types collapsed")
	}
	for _, key := range []Value{NewList(), VectorValue{}, NewMap(), &FunctionValue{}} {
		if err := m.Set(key, true); err == nil {
			t.Errorf("accepted key %#v", key)
		}
	}
}

func TestCyclicEquality(t *testing.T) {
	a, b := VectorValue{nil}, VectorValue{nil}
	a[0], b[0] = a, b
	if !Equal(a, b) {
		t.Fatal("equal cycles differ")
	}
	x, y := NewMap(), NewMap()
	if err := x.Set("self", x); err != nil {
		t.Fatal(err)
	}
	if err := y.Set("self", y); err != nil {
		t.Fatal(err)
	}
	if !Equal(x, y) {
		t.Fatal("equal map cycles differ")
	}
	if err := y.Set("extra", true); err != nil {
		t.Fatal(err)
	}
	if Equal(x, y) {
		t.Fatal("different cycles equal")
	}
}

func TestRuntimeError(t *testing.T) {
	err := &RuntimeError{Kind: TypeError, Position: Position{"sample.guri", 2, 4}, Cause: "expected number"}
	if !strings.Contains(err.Error(), "sample.guri:2:4: TypeError: expected number") {
		t.Fatal(err)
	}
}
