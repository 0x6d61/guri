package core

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

type pairArity struct{ min, max int }

var pairArities = func() map[string]pairArity {
	out := map[string]pairArity{}
	for _, n := range []string{"pair?", "null?", "list?", "length", "reverse", "list-copy", "boolean?", "not", "symbol?", "symbol->string", "string->symbol"} {
		out[n] = pairArity{1, 1}
	}
	for _, n := range []string{"cons", "set-car!", "set-cdr!", "list-ref", "list-tail", "eq?", "eqv?", "equal?", "memq", "memv", "assq", "assv"} {
		out[n] = pairArity{2, 2}
	}
	out["list-set!"] = pairArity{3, 3}
	out["member"] = pairArity{2, 3}
	out["assoc"] = pairArity{2, 3}
	out["make-list"] = pairArity{1, 2}
	out["list"] = pairArity{0, -1}
	out["append"] = pairArity{0, -1}
	out["symbol=?"] = pairArity{2, -1}
	out["boolean=?"] = pairArity{2, -1}
	out["map"] = pairArity{2, -1}
	out["for-each"] = pairArity{2, -1}
	for depth := 1; depth <= 4; depth++ {
		for mask := 0; mask < 1<<depth; mask++ {
			var name strings.Builder
			name.WriteByte('c')
			for bit := 0; bit < depth; bit++ {
				if mask&(1<<bit) == 0 {
					name.WriteByte('a')
				} else {
					name.WriteByte('d')
				}
			}
			name.WriteByte('r')
			out[name.String()] = pairArity{1, 1}
		}
	}
	return out
}()

func pairFailure(c *vm.Context, name string, args []scheme.Value, err error) (vm.Action, error) {
	object, e := NewErrorObject(ErrorOrdinary, fmt.Sprintf("%s: %v", name, err), args...)
	if e != nil {
		return vm.Action{}, e
	}
	return Raise(c, object, false)
}
func (e *Engine) installPairs() {
	for name, arity := range pairArities {
		e.Bind(name, vm.Native(name, arity.min, arity.max, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if name == "map" || name == "for-each" {
				return iterateLists(c, name, a)
			}
			if name == "member" || name == "memq" || name == "memv" || name == "assoc" || name == "assq" || name == "assv" {
				return searchList(c, name, a)
			}
			value, err := e.pairPrimitive(name, a)
			if err != nil {
				return pairFailure(c, name, a, err)
			}
			return vm.Return(value), nil
		}))
	}
}
func nonnegativeIndex(v scheme.Value) (*big.Int, error) {
	n, ok := v.(*scheme.Integer)
	if !ok {
		return nil, scheme.ErrType
	}
	k := n.BigInt()
	if k.Sign() < 0 {
		return nil, scheme.ErrIndex
	}
	return k, nil
}
func listTail(v scheme.Value, k *big.Int) (scheme.Value, error) {
	remaining := new(big.Int).Set(k)
	seen := map[*scheme.Pair]int{}
	steps := 0
	for remaining.Sign() > 0 {
		p, ok := v.(*scheme.Pair)
		if !ok {
			return nil, scheme.ErrIndex
		}
		if first, ok := seen[p]; ok {
			count := new(big.Int).Mod(remaining, big.NewInt(int64(steps-first))).Int64()
			for ; count > 0; count-- {
				v = v.(*scheme.Pair).Cdr()
			}
			return v, nil
		}
		seen[p] = steps
		steps++
		remaining.Sub(remaining, big.NewInt(1))
		v = p.Cdr()
	}
	return v, nil
}
func (e *Engine) pairPrimitive(name string, a []scheme.Value) (scheme.Value, error) {
	switch name {
	case "eq?":
		return scheme.Boolean(equality.Eq(a[0], a[1])), nil
	case "eqv?":
		return scheme.Boolean(equality.Eqv(a[0], a[1])), nil
	case "equal?":
		return scheme.Boolean(equality.Equal(a[0], a[1])), nil
	case "pair?":
		_, ok := a[0].(*scheme.Pair)
		return scheme.Boolean(ok), nil
	case "null?":
		return scheme.Boolean(a[0] == scheme.Null), nil
	case "list?":
		_, err := properValues(a[0])
		return scheme.Boolean(err == nil), nil
	case "boolean?":
		_, ok := a[0].(scheme.Boolean)
		return scheme.Boolean(ok), nil
	case "not":
		return scheme.Boolean(!scheme.Truthy(a[0])), nil
	case "symbol?":
		_, ok := a[0].(*scheme.Symbol)
		return scheme.Boolean(ok), nil
	case "boolean=?", "symbol=?":
		same := true
		for _, v := range a {
			if name == "boolean=?" {
				b, ok := v.(scheme.Boolean)
				if !ok {
					return nil, scheme.ErrType
				}
				same = same && b == a[0]
			} else {
				b, ok := v.(*scheme.Symbol)
				if !ok {
					return nil, scheme.ErrType
				}
				same = same && b.Name() == a[0].(*scheme.Symbol).Name()
			}
		}
		return scheme.Boolean(same), nil
	case "symbol->string":
		s, ok := a[0].(*scheme.Symbol)
		if !ok {
			return nil, scheme.ErrType
		}
		str, err := scheme.NewString(s.Name())
		if err != nil {
			return nil, err
		}
		scheme.FreezeLiteral(str)
		return str, nil
	case "string->symbol":
		s, ok := a[0].(*scheme.String)
		if !ok {
			return nil, scheme.ErrType
		}
		return e.symbols.Intern(s.Text()), nil
	case "cons":
		return scheme.Cons(a[0], a[1]), nil
	case "set-car!", "set-cdr!":
		p, ok := a[0].(*scheme.Pair)
		if !ok {
			return nil, scheme.ErrType
		}
		var err error
		if name == "set-car!" {
			err = p.SetCar(a[1])
		} else {
			err = p.SetCdr(a[1])
		}
		return scheme.Unspecified, err
	case "list":
		return scheme.List(a...), nil
	case "make-list":
		k, err := nonnegativeIndex(a[0])
		if err != nil {
			return nil, err
		}
		if !k.IsInt64() || int64(int(k.Int64())) != k.Int64() {
			return nil, scheme.ErrRange
		}
		var fill scheme.Value = scheme.Unspecified
		if len(a) == 2 {
			fill = a[1]
		}
		var out scheme.Value = scheme.Null
		for count := k.Int64(); count > 0; count-- {
			out = scheme.Cons(fill, out)
		}
		return out, nil
	case "length", "reverse":
		values, err := properValues(a[0])
		if err != nil {
			return nil, err
		}
		if name == "length" {
			return scheme.NewInteger(int64(len(values))), nil
		}
		var out scheme.Value = scheme.Null
		for _, v := range values {
			out = scheme.Cons(v, out)
		}
		return out, nil
	case "append":
		if len(a) == 0 {
			return scheme.Null, nil
		}
		var values []scheme.Value
		for _, v := range a[:len(a)-1] {
			xs, err := properValues(v)
			if err != nil {
				return nil, err
			}
			values = append(values, xs...)
		}
		out := a[len(a)-1]
		for i := len(values) - 1; i >= 0; i-- {
			out = scheme.Cons(values[i], out)
		}
		return out, nil
	case "list-copy":
		seen := map[*scheme.Pair]bool{}
		var values []scheme.Value
		tail := a[0]
		for {
			p, ok := tail.(*scheme.Pair)
			if !ok {
				break
			}
			if seen[p] {
				return nil, scheme.ErrType
			}
			seen[p] = true
			values = append(values, p.Car())
			tail = p.Cdr()
		}
		for i := len(values) - 1; i >= 0; i-- {
			tail = scheme.Cons(values[i], tail)
		}
		return tail, nil
	case "list-tail", "list-ref", "list-set!":
		k, err := nonnegativeIndex(a[1])
		if err != nil {
			return nil, err
		}
		tail, err := listTail(a[0], k)
		if err != nil {
			return nil, err
		}
		if name == "list-tail" {
			return tail, nil
		}
		p, ok := tail.(*scheme.Pair)
		if !ok {
			return nil, scheme.ErrIndex
		}
		if name == "list-ref" {
			return p.Car(), nil
		}
		return scheme.Unspecified, p.SetCar(a[2])
	default:
		v := a[0]
		for i := len(name) - 2; i >= 1; i-- {
			p, ok := v.(*scheme.Pair)
			if !ok {
				return nil, scheme.ErrType
			}
			if name[i] == 'a' {
				v = p.Car()
			} else {
				v = p.Cdr()
			}
		}
		return v, nil
	}
}
