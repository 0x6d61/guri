// Package number implements the approved R7RS numeric profile using immutable
// Scheme numbers. It has no evaluator or I/O dependencies.
package number

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/0x6d61/guri/internal/scheme"
)

var ErrDomain = errors.New("undefined numeric operation")
var ErrArity = errors.New("incorrect numeric argument count")

type Arity struct{ Min, Max int }

var arities = func() map[string]Arity {
	m := map[string]Arity{}
	for _, name := range []string{"+", "*", "gcd", "lcm"} {
		m[name] = Arity{0, -1}
	}
	for _, name := range []string{"-", "/", "max", "min"} {
		m[name] = Arity{1, -1}
	}
	for _, name := range []string{"=", "<", "<=", ">", ">="} {
		m[name] = Arity{2, -1}
	}
	for _, name := range []string{"abs", "number?", "complex?", "real?", "rational?", "integer?", "exact-integer?", "exact?", "inexact?", "zero?", "positive?", "negative?", "even?", "odd?", "numerator", "denominator", "floor", "ceiling", "truncate", "round", "exact", "inexact", "exact->inexact", "inexact->exact", "square", "sqrt", "exact-integer-sqrt", "real-part", "imag-part", "magnitude", "angle", "exp", "sin", "cos", "tan", "asin", "acos", "finite?", "infinite?", "nan?"} {
		m[name] = Arity{1, 1}
	}
	for _, name := range []string{"floor/", "floor-quotient", "floor-remainder", "truncate/", "truncate-quotient", "truncate-remainder", "quotient", "remainder", "modulo", "rationalize", "expt", "make-rectangular", "make-polar"} {
		m[name] = Arity{2, 2}
	}
	for _, name := range []string{"atan", "log", "number->string", "string->number"} {
		m[name] = Arity{1, 2}
	}
	return m
}()

func Catalog() map[string]Arity {
	out := map[string]Arity{}
	for k, v := range arities {
		out[k] = v
	}
	return out
}
func integer(n *big.Int) *scheme.Integer {
	v, err := scheme.IntegerFromBig(n)
	if err != nil {
		panic(err)
	}
	return v
}
func rational(q *big.Rat) scheme.Real {
	v, err := scheme.NewRational(q.Num(), q.Denom())
	if err != nil {
		panic(err)
	}
	return v
}
func rat(n scheme.Real) *big.Rat {
	switch v := n.(type) {
	case *scheme.Integer:
		return new(big.Rat).SetInt(v.BigInt())
	case *scheme.Rational:
		return v.BigRat()
	case scheme.Float:
		return new(big.Rat).SetFloat64(v.Float64())
	}
	panic("unknown real representation")
}
func floating(n scheme.Real) float64 {
	if f, ok := n.(scheme.Float); ok {
		return f.Float64()
	}
	v, _ := rat(n).Float64()
	return v
}
func parts(n scheme.Number) (scheme.Real, scheme.Real) {
	if c, ok := n.(*scheme.Complex); ok {
		return c.Parts()
	}
	return n.(scheme.Real), scheme.NewInteger(0)
}
func complexValue(n scheme.Number) complex128 {
	r, i := parts(n)
	return complex(floating(r), floating(i))
}
func fromComplex(c complex128) scheme.Number {
	n, err := scheme.NewComplex(scheme.NewFloat(real(c)), scheme.NewFloat(imag(c)))
	if err != nil {
		panic(err)
	}
	return n
}
func isNaN(n scheme.Real) bool { f, ok := n.(scheme.Float); return ok && math.IsNaN(f.Float64()) }
func isInf(n scheme.Real) int {
	if f, ok := n.(scheme.Float); ok {
		if math.IsInf(f.Float64(), 1) {
			return 1
		}
		if math.IsInf(f.Float64(), -1) {
			return -1
		}
	}
	return 0
}

// Compare preserves transitivity by comparing finite binary64 values as exact
// binary rationals, without rounding the other argument to binary64.
func Compare(a, b scheme.Real) (int, bool) {
	if isNaN(a) || isNaN(b) {
		return 0, false
	}
	ia, ib := isInf(a), isInf(b)
	if ia != 0 || ib != 0 {
		if ia == ib {
			return 0, true
		}
		if ia == -1 || ib == 1 {
			return -1, true
		}
		return 1, true
	}
	return rat(a).Cmp(rat(b)), true
}
func Equal(a, b scheme.Number) bool {
	ar, ai := parts(a)
	br, bi := parts(b)
	r, ok := Compare(ar, br)
	if !ok || r != 0 {
		return false
	}
	i, ok := Compare(ai, bi)
	return ok && i == 0
}

// Equivalent is the numeric eqv? relation. NaN values are equivalent to each
// other (a permitted choice); finite values distinguish signed zero and exactness.
func Equivalent(a, b scheme.Number) bool {
	if a.Exact() != b.Exact() {
		return false
	}
	if a.Exact() {
		return Equal(a, b)
	}
	_, ac := a.(*scheme.Complex)
	_, bc := b.(*scheme.Complex)
	if ac != bc {
		return false
	}
	ar, ai := parts(a)
	br, bi := parts(b)
	same := func(x, y scheme.Real) bool {
		fx, fy := floating(x), floating(y)
		if math.IsNaN(fx) && math.IsNaN(fy) {
			return true
		}
		return math.Float64bits(fx) == math.Float64bits(fy)
	}
	return same(ar, br) && same(ai, bi)
}
func convert(n scheme.Number, exact bool) (scheme.Number, error) {
	if n.Exact() == exact {
		return n, nil
	}
	change := func(r scheme.Real) (scheme.Real, error) {
		if !exact {
			return scheme.NewFloat(floating(r)), nil
		}
		q := rat(r)
		if q == nil {
			return nil, scheme.ErrRange
		}
		return rational(q), nil
	}
	r, i := parts(n)
	real, err := change(r)
	if err != nil {
		return nil, err
	}
	if _, ok := n.(scheme.Real); ok {
		return real, nil
	}
	imag, err := change(i)
	if err != nil {
		return nil, err
	}
	return scheme.NewComplex(real, imag)
}
func binary(op string, a, b scheme.Number) (scheme.Number, error) {
	if !a.Exact() || !b.Exact() {
		ar, aReal := a.(scheme.Real)
		br, bReal := b.(scheme.Real)
		if aReal && bReal {
			x, y := floating(ar), floating(br)
			switch op {
			case "+":
				return scheme.NewFloat(x + y), nil
			case "-":
				return scheme.NewFloat(x - y), nil
			case "*":
				return scheme.NewFloat(x * y), nil
			case "/":
				return scheme.NewFloat(x / y), nil
			}
		}
		x, y := complexValue(a), complexValue(b)
		switch op {
		case "+":
			return fromComplex(x + y), nil
		case "-":
			return fromComplex(x - y), nil
		case "*":
			return fromComplex(x * y), nil
		case "/":
			return fromComplex(x / y), nil
		}
	}
	ar, ai := parts(a)
	br, bi := parts(b)
	x, y, u, v := rat(ar), rat(ai), rat(br), rat(bi)
	var real, imag *big.Rat
	switch op {
	case "+":
		real = new(big.Rat).Add(x, u)
		imag = new(big.Rat).Add(y, v)
	case "-":
		real = new(big.Rat).Sub(x, u)
		imag = new(big.Rat).Sub(y, v)
	case "*":
		real = new(big.Rat).Sub(new(big.Rat).Mul(x, u), new(big.Rat).Mul(y, v))
		imag = new(big.Rat).Add(new(big.Rat).Mul(x, v), new(big.Rat).Mul(y, u))
	case "/":
		den := new(big.Rat).Add(new(big.Rat).Mul(u, u), new(big.Rat).Mul(v, v))
		if den.Sign() == 0 {
			return nil, scheme.ErrDivisionByZero
		}
		real = new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(x, u), new(big.Rat).Mul(y, v)), den)
		imag = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(y, u), new(big.Rat).Mul(x, v)), den)
	default:
		return nil, ErrDomain
	}
	return scheme.NewComplex(rational(real), rational(imag))
}
func asNumber(v scheme.Value) (scheme.Number, error) {
	n, ok := v.(scheme.Number)
	if !ok {
		return nil, scheme.ErrType
	}
	return n, nil
}
func asReal(v scheme.Value) (scheme.Real, error) {
	n, ok := v.(scheme.Real)
	if !ok {
		return nil, scheme.ErrType
	}
	return n, nil
}
func asInteger(v scheme.Value) (*big.Int, bool, error) {
	r, err := asReal(v)
	if err != nil {
		return nil, false, err
	}
	q := rat(r)
	if q == nil || !q.IsInt() {
		return nil, false, scheme.ErrType
	}
	return new(big.Int).Set(q.Num()), !r.Exact(), nil
}
func integerResult(n *big.Int, inexact bool) (scheme.Number, error) {
	if !inexact {
		return integer(n), nil
	}
	f := floating(integer(n))
	if math.IsInf(f, 0) {
		return nil, scheme.ErrRange
	}
	return scheme.NewFloat(f), nil
}
func Call(name string, args ...scheme.Value) (scheme.Results, error) {
	a, ok := arities[name]
	if !ok {
		return scheme.Results{}, fmt.Errorf("unknown numeric procedure %s", name)
	}
	if len(args) < a.Min || a.Max >= 0 && len(args) > a.Max {
		return scheme.Results{}, ErrArity
	}
	values, err := dispatch(name, args)
	if err != nil {
		return scheme.Results{}, fmt.Errorf("%s: %w", name, err)
	}
	return scheme.NewResults(values...), nil
}
func one(v scheme.Value, err error) ([]scheme.Value, error) {
	if err != nil {
		return nil, err
	}
	return []scheme.Value{v}, nil
}
func dispatch(name string, a []scheme.Value) ([]scheme.Value, error) {
	switch name {
	case "number->string", "string->number":
		return textConversion(name, a)
	case "number?", "complex?", "real?", "rational?", "integer?", "exact-integer?":
		_, ok := a[0].(scheme.Number)
		answer := ok
		switch name {
		case "real?":
			_, answer = a[0].(scheme.Real)
		case "rational?":
			r, ok := a[0].(scheme.Real)
			answer = ok && !isNaN(r) && isInf(r) == 0
		case "integer?":
			_, _, err := asInteger(a[0])
			answer = err == nil
		case "exact-integer?":
			_, answer = a[0].(*scheme.Integer)
		}
		return one(scheme.Boolean(answer), nil)
	}
	numbers := make([]scheme.Number, len(a))
	for i, v := range a {
		n, err := asNumber(v)
		if err != nil {
			return nil, err
		}
		numbers[i] = n
	}
	switch name {
	case "+", "-", "*", "/":
		if name == "-" && len(numbers) == 1 {
			negate := func(r scheme.Real) scheme.Real {
				if f, ok := r.(scheme.Float); ok {
					return scheme.NewFloat(-f.Float64())
				}
				return rational(new(big.Rat).Neg(rat(r)))
			}
			if r, ok := numbers[0].(scheme.Real); ok {
				return one(negate(r), nil)
			}
			r, i := parts(numbers[0])
			v, err := scheme.NewComplex(negate(r), negate(i))
			return one(v, err)
		}
		var result scheme.Number = scheme.NewInteger(0)
		start := 0
		if name == "*" {
			result = scheme.NewInteger(1)
		}
		if (name == "+" || name == "*") && len(numbers) > 0 {
			result = numbers[0]
			start = 1
		}
		if name == "-" || name == "/" {
			if len(a) == 1 {
				if name == "/" {
					result = scheme.NewInteger(1)
				}
			} else {
				result = numbers[0]
				start = 1
			}
		}
		for _, n := range numbers[start:] {
			var err error
			result, err = binary(name, result, n)
			if err != nil {
				return nil, err
			}
		}
		return one(result, nil)
	case "=", "<", "<=", ">", ">=":
		answer := true
		for i := 1; i < len(numbers); i++ {
			if name == "=" {
				answer = answer && Equal(numbers[i-1], numbers[i])
				continue
			}
			x, err := asReal(numbers[i-1])
			if err != nil {
				return nil, err
			}
			y, err := asReal(numbers[i])
			if err != nil {
				return nil, err
			}
			cmp, ordered := Compare(x, y)
			match := ordered && (name == "<" && cmp < 0 || name == "<=" && cmp <= 0 || name == ">" && cmp > 0 || name == ">=" && cmp >= 0)
			answer = answer && match
		}
		return one(scheme.Boolean(answer), nil)
	case "exact?", "inexact?":
		return one(scheme.Boolean(numbers[0].Exact() == (name == "exact?")), nil)
	case "exact", "inexact", "exact->inexact", "inexact->exact":
		n, err := convert(numbers[0], name == "exact" || name == "inexact->exact")
		return one(n, err)
	case "zero?":
		return one(scheme.Boolean(Equal(numbers[0], scheme.NewInteger(0))), nil)
	case "finite?", "infinite?", "nan?":
		r, i := parts(numbers[0])
		finite := isInf(r) == 0 && isInf(i) == 0 && !isNaN(r) && !isNaN(i)
		answer := finite
		if name == "infinite?" {
			answer = isInf(r) != 0 || isInf(i) != 0
		}
		if name == "nan?" {
			answer = isNaN(r) || isNaN(i)
		}
		return one(scheme.Boolean(answer), nil)
	case "positive?", "negative?":
		r, err := asReal(a[0])
		if err != nil {
			return nil, err
		}
		cmp, ordered := Compare(r, scheme.NewInteger(0))
		return one(scheme.Boolean(ordered && (name == "positive?" && cmp > 0 || name == "negative?" && cmp < 0)), nil)
	case "abs":
		r, err := asReal(a[0])
		if err != nil {
			return nil, err
		}
		if !r.Exact() {
			return one(scheme.NewFloat(math.Abs(floating(r))), nil)
		}
		return one(rational(new(big.Rat).Abs(rat(r))), nil)
	case "max", "min":
		var best scheme.Real
		inexact := false
		for _, v := range a {
			r, err := asReal(v)
			if err != nil {
				return nil, err
			}
			inexact = inexact || !r.Exact()
			if best == nil {
				best = r
				continue
			}
			cmp, ordered := Compare(r, best)
			if !ordered {
				best = scheme.NewFloat(math.NaN())
			} else if name == "max" && cmp > 0 || name == "min" && cmp < 0 {
				best = r
			}
		}
		if inexact {
			return one(scheme.NewFloat(floating(best)), nil)
		}
		return one(best, nil)
	case "square":
		n, err := binary("*", numbers[0], numbers[0])
		return one(n, err)
	case "numerator", "denominator", "floor", "ceiling", "truncate", "round", "floor/", "floor-quotient", "floor-remainder", "truncate/", "truncate-quotient", "truncate-remainder", "quotient", "remainder", "modulo", "even?", "odd?", "gcd", "lcm", "exact-integer-sqrt", "rationalize":
		return integerOperation(name, a)
	default:
		return mathematical(name, numbers)
	}
}
