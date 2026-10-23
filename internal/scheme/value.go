// Package scheme contains the R7RS value model shared by the future reader,
// evaluator and standard libraries. It is internal to the interpreter.
package scheme

import "errors"

// Value excludes arbitrary Go values and all legacy Guri-specific types.
type Value interface {
	Kind() Kind
	schemeValue() bool
}

type Kind string

const (
	KindBoolean     Kind = "boolean"
	KindNull        Kind = "null"
	KindNumber      Kind = "number"
	KindCharacter   Kind = "character"
	KindString      Kind = "string"
	KindPair        Kind = "pair"
	KindVector      Kind = "vector"
	KindBytevector  Kind = "bytevector"
	KindSymbol      Kind = "symbol"
	KindProcedure   Kind = "procedure"
	KindEOF         Kind = "eof"
	KindRecord      Kind = "record"
	KindPort        Kind = "port"
	KindUnspecified Kind = "unspecified"
)

type Boolean bool

const (
	False Boolean = false
	True  Boolean = true
)

func (Boolean) Kind() Kind        { return KindBoolean }
func (Boolean) schemeValue() bool { return true }

type singleton uint8

const (
	Null singleton = iota
	EOF
	Unspecified
)

func (s singleton) Kind() Kind {
	switch s {
	case Null:
		return KindNull
	case EOF:
		return KindEOF
	default:
		return KindUnspecified
	}
}
func (singleton) schemeValue() bool { return true }

// Truthy tests a Scheme value. Go nil is an internal programming error, not nil.
func Truthy(v Value) bool { requireValue(v); return v != False }

var (
	ErrIndex          = errors.New("index out of bounds")
	ErrRange          = errors.New("value outside supported range")
	ErrImmutable      = errors.New("cannot modify a literal constant")
	ErrEncoding       = errors.New("invalid UTF-8")
	ErrInvalidValue   = errors.New("invalid Scheme value")
	ErrDivisionByZero = errors.New("division by zero")
	ErrUninitialized  = errors.New("uninitialized binding")
	ErrType           = errors.New("incorrect value type")
	ErrValueCount     = errors.New("incorrect number of values")
)

func requireValue(v Value) {
	if v == nil || !v.schemeValue() {
		panic("scheme: Go nil is not a Scheme value")
	}
}

// FreezeLiteral marks all reachable literal containers, preserving their
// identity and sharing. An explicit work list handles cycles and deep lists.
func FreezeLiteral(root Value) {
	requireValue(root)
	pending := []Value{root}
	seen := make(map[Value]bool)
	for len(pending) > 0 {
		v := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[v] {
			continue
		}
		seen[v] = true
		switch x := v.(type) {
		case *Pair:
			x.frozen = true
			pending = append(pending, x.car, x.cdr)
		case *Vector:
			x.frozen = true
			pending = append(pending, x.values...)
		case *String:
			x.frozen = true
		case *Bytevector:
			x.frozen = true
		}
	}
}
