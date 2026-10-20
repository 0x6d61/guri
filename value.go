package guri

import "math/big"

// Value is a Guri value: nil, bool, int64, float64, string, or one of the
// collection, identifier and function types defined here. Evaluation is single-threaded.
type Value = any
type SymbolValue string
type KeywordValue string

// BytesValue is an immutable sequence of arbitrary bytes, distinct from String.
type BytesValue string
type VectorValue []Value

type listNode struct {
	value Value
	next  *listNode
}

// ListValue exposes no operation that changes its linked structure.
type ListValue struct {
	head   *listNode
	length int
}

func NewList(values ...Value) ListValue {
	list := ListValue{length: len(values)}
	for i := len(values) - 1; i >= 0; i-- {
		list.head = &listNode{values[i], list.head}
	}
	return list
}

func (l ListValue) Values() []Value {
	values := make([]Value, 0, l.length)
	for n := l.head; n != nil; n = n.next {
		values = append(values, n.value)
	}
	return values
}

// MapValue is shared by reference. Go interface-key semantics keep key types distinct.
type MapValue struct{ entries map[Value]Value }

func NewMap() *MapValue { return &MapValue{entries: make(map[Value]Value)} }

func validKey(key Value) bool {
	switch key.(type) {
	case nil, bool, int64, float64, string, BytesValue, SymbolValue, KeywordValue:
		return true
	}
	return false
}

func (m *MapValue) Set(key, value Value) error {
	if !validKey(key) {
		return failure(TypeError, "invalid Map key type %T", key)
	}
	m.entries[key] = value
	return nil
}

func (m *MapValue) Get(key Value) (Value, error) {
	if !validKey(key) {
		return nil, failure(TypeError, "invalid Map key type %T", key)
	}
	return m.entries[key], nil
}

// FunctionValue has reference identity, including for native functions.
type FunctionValue struct{ call func([]Value) (Value, error) }

func Truthy(value Value) bool {
	if value == nil {
		return false
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return true
}

// compareNumbers uses exact rational representations, avoiding int64-to-float64 rounding.
func compareNumbers(a, b Value) (int, bool) {
	toRat := func(v Value) *big.Rat {
		switch n := v.(type) {
		case int64:
			return new(big.Rat).SetInt64(n)
		case float64:
			return new(big.Rat).SetFloat64(n)
		}
		return nil
	}
	x, y := toRat(a), toRat(b)
	if x == nil || y == nil {
		return 0, false
	}
	return x.Cmp(y), true
}

type equalityPair struct {
	left, right any
	length      int
}

func Equal(a, b Value) bool { return equal(a, b, make(map[equalityPair]bool)) }

func equal(a, b Value, seen map[equalityPair]bool) bool {
	if comparison, ok := compareNumbers(a, b); ok {
		return comparison == 0
	}
	var av, bv []Value
	var pair equalityPair
	switch x := a.(type) {
	case nil:
		return b == nil
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case BytesValue:
		y, ok := b.(BytesValue)
		return ok && x == y
	case SymbolValue:
		y, ok := b.(SymbolValue)
		return ok && x == y
	case KeywordValue:
		y, ok := b.(KeywordValue)
		return ok && x == y
	case *FunctionValue:
		y, ok := b.(*FunctionValue)
		return ok && x == y
	case ListValue:
		y, ok := b.(ListValue)
		if !ok || x.length != y.length {
			return false
		}
		pair = equalityPair{x.head, y.head, x.length}
		if seen[pair] {
			return true
		}
		seen[pair] = true
		for l, r := x.head, y.head; l != nil; l, r = l.next, r.next {
			if !equal(l.value, r.value, seen) {
				return false
			}
		}
		return true
	case VectorValue:
		y, ok := b.(VectorValue)
		if !ok || len(x) != len(y) {
			return false
		}
		if len(x) == 0 {
			return true
		}
		pair = equalityPair{&x[0], &y[0], len(x)}
		av, bv = x, y
	case *MapValue:
		y, ok := b.(*MapValue)
		if !ok || len(x.entries) != len(y.entries) {
			return false
		}
		pair = equalityPair{x, y, 0}
		if seen[pair] {
			return true
		}
		seen[pair] = true
		for key, value := range x.entries {
			other, exists := y.entries[key]
			if !exists || !equal(value, other, seen) {
				return false
			}
		}
		return true
	default:
		return false
	}
	if seen[pair] {
		return true
	}
	seen[pair] = true
	for i := range av {
		if !equal(av[i], bv[i], seen) {
			return false
		}
	}
	return true
}
