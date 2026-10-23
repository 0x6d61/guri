// Package equality implements the Scheme equivalence relations without an
// evaluator dependency. Equal compares finite graphs by their unfoldings.
package equality

import (
	"bytes"
	numeric "github.com/0x6d61/guri/internal/number"
	"github.com/0x6d61/guri/internal/scheme"
)

func Eqv(a, b scheme.Value) bool {
	if a.Kind() != b.Kind() {
		return false
	}
	switch x := a.(type) {
	case scheme.Number:
		return numeric.Equivalent(x, b.(scheme.Number))
	case *scheme.Symbol:
		return x.Name() == b.(*scheme.Symbol).Name()
	default:
		return a == b
	}
}

// Eq uses the same permitted numeric/character choices as Eqv.
func Eq(a, b scheme.Value) bool { return Eqv(a, b) }

type comparison struct{ a, b scheme.Value }

func Equal(a, b scheme.Value) bool {
	pending := []comparison{{a, b}}
	seen := map[comparison]bool{}
	for len(pending) > 0 {
		item := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		a, b = item.a, item.b
		if Eqv(a, b) {
			continue
		}
		if a.Kind() != b.Kind() {
			return false
		}
		if seen[item] {
			continue
		}
		seen[item] = true
		switch x := a.(type) {
		case *scheme.Pair:
			y := b.(*scheme.Pair)
			pending = append(pending, comparison{x.Car(), y.Car()}, comparison{x.Cdr(), y.Cdr()})
		case *scheme.Vector:
			y := b.(*scheme.Vector)
			if x.Len() != y.Len() {
				return false
			}
			left, right := x.Values(), y.Values()
			for i, av := range left {
				pending = append(pending, comparison{av, right[i]})
			}
		case *scheme.String:
			if x.Text() != b.(*scheme.String).Text() {
				return false
			}
		case *scheme.Bytevector:
			if !bytes.Equal(x.Bytes(), b.(*scheme.Bytevector).Bytes()) {
				return false
			}
		default:
			return false
		}
	}
	return true
}
