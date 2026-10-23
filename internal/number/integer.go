package number

import (
	"math"
	"math/big"

	"github.com/0x6d61/guri/internal/scheme"
)

func rounded(q *big.Rat, mode string) *big.Int {
	n, d := q.Num(), q.Denom()
	quot, rem := new(big.Int), new(big.Int)
	quot.QuoRem(n, d, rem)
	switch mode {
	case "floor":
		if rem.Sign() < 0 {
			quot.Sub(quot, big.NewInt(1))
		}
	case "ceiling":
		if rem.Sign() > 0 {
			quot.Add(quot, big.NewInt(1))
		}
	case "round":
		twice := new(big.Int).Lsh(new(big.Int).Abs(rem), 1)
		cmp := twice.Cmp(d)
		if cmp > 0 || cmp == 0 && quot.Bit(0) == 1 {
			quot.Add(quot, big.NewInt(int64(n.Sign())))
		}
	}
	return quot
}
func integerOperation(name string, a []scheme.Value) ([]scheme.Value, error) {
	switch name {
	case "gcd", "lcm":
		answer := big.NewInt(0)
		if name == "lcm" {
			answer.SetInt64(1)
		}
		inexact := false
		for _, v := range a {
			n, inex, err := asInteger(v)
			if err != nil {
				return nil, err
			}
			inexact = inexact || inex
			n.Abs(n)
			g := new(big.Int).GCD(nil, nil, answer, n)
			if name == "gcd" {
				answer = g
			} else if answer.Sign() == 0 || n.Sign() == 0 {
				answer = big.NewInt(0)
			} else {
				answer = new(big.Int).Mul(new(big.Int).Quo(answer, g), n)
			}
		}
		v, err := integerResult(answer, inexact)
		return one(v, err)
	case "even?", "odd?":
		n, _, err := asInteger(a[0])
		if err != nil {
			return nil, err
		}
		return one(scheme.Boolean(n.Bit(0) == 0 == (name == "even?")), nil)
	case "exact-integer-sqrt":
		n, ok := a[0].(*scheme.Integer)
		if !ok || n.BigInt().Sign() < 0 {
			return nil, scheme.ErrType
		}
		root := new(big.Int).Sqrt(n.BigInt())
		rem := new(big.Int).Sub(n.BigInt(), new(big.Int).Mul(root, root))
		return []scheme.Value{integer(root), integer(rem)}, nil
	case "numerator", "denominator":
		r, err := asReal(a[0])
		if err != nil {
			return nil, err
		}
		q := rat(r)
		if q == nil {
			return nil, ErrDomain
		}
		v := q.Num()
		if name == "denominator" {
			v = q.Denom()
		}
		n, err := integerResult(v, !r.Exact())
		return one(n, err)
	case "floor", "ceiling", "truncate", "round":
		r, err := asReal(a[0])
		if err != nil {
			return nil, err
		}
		if !r.Exact() {
			f := floating(r)
			switch name {
			case "floor":
				f = math.Floor(f)
			case "ceiling":
				f = math.Ceil(f)
			case "truncate":
				f = math.Trunc(f)
			case "round":
				f = math.RoundToEven(f)
			}
			return one(scheme.NewFloat(f), nil)
		}
		return one(integer(rounded(rat(r), name)), nil)
	case "rationalize":
		x, err := asReal(a[0])
		if err != nil {
			return nil, err
		}
		y, err := asReal(a[1])
		if err != nil {
			return nil, err
		}
		if isNaN(x) || isNaN(y) || isInf(x) != 0 {
			return nil, ErrDomain
		}
		if isInf(y) != 0 {
			return one(scheme.NewFloat(0), nil)
		}
		tolerance := new(big.Rat).Abs(rat(y))
		low := new(big.Rat).Sub(rat(x), tolerance)
		high := new(big.Rat).Add(rat(x), tolerance)
		result := rational(simplest(low, high))
		if !x.Exact() || !y.Exact() {
			return one(scheme.NewFloat(floating(result)), nil)
		}
		return one(result, nil)
	}
	x, ix, err := asInteger(a[0])
	if err != nil {
		return nil, err
	}
	y, iy, err := asInteger(a[1])
	if err != nil {
		return nil, err
	}
	if y.Sign() == 0 {
		return nil, scheme.ErrDivisionByZero
	}
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(x, y, r)
	floor := name == "floor/" || name == "floor-quotient" || name == "floor-remainder" || name == "modulo"
	if floor && r.Sign() != 0 && x.Sign() != y.Sign() {
		q.Sub(q, big.NewInt(1))
		r.Sub(x, new(big.Int).Mul(q, y))
	}
	qv, err := integerResult(q, ix || iy)
	if err != nil {
		return nil, err
	}
	rv, err := integerResult(r, ix || iy)
	if err != nil {
		return nil, err
	}
	if ix || iy {
		negative := func(v scheme.Value) bool { r, _ := asReal(v); return math.Signbit(floating(r)) }
		if q.Sign() == 0 && negative(a[0]) != negative(a[1]) {
			qv = scheme.NewFloat(math.Copysign(0, -1))
		}
		if r.Sign() == 0 {
			sign := negative(a[0])
			if floor {
				sign = negative(a[1])
			}
			if sign {
				rv = scheme.NewFloat(math.Copysign(0, -1))
			}
		}
	}
	switch name {
	case "floor/", "truncate/":
		return []scheme.Value{qv, rv}, nil
	case "floor-remainder", "truncate-remainder", "remainder", "modulo":
		return one(rv, nil)
	default:
		return one(qv, nil)
	}
}
func simplest(low, high *big.Rat) *big.Rat {
	if low.Sign() <= 0 && high.Sign() >= 0 {
		return new(big.Rat)
	}
	if high.Sign() < 0 {
		return new(big.Rat).Neg(simplest(new(big.Rat).Neg(high), new(big.Rat).Neg(low)))
	}
	floor := rounded(low, "floor")
	if low.IsInt() {
		return new(big.Rat).SetInt(floor)
	}
	ceil := new(big.Int).Add(new(big.Int).Set(floor), big.NewInt(1))
	if new(big.Rat).SetInt(ceil).Cmp(high) <= 0 {
		return new(big.Rat).SetInt(ceil)
	}
	a := new(big.Rat).SetInt(floor)
	loRem := new(big.Rat).Sub(low, a)
	hiRem := new(big.Rat).Sub(high, a)
	inner := simplest(new(big.Rat).Inv(hiRem), new(big.Rat).Inv(loRem))
	return new(big.Rat).Add(a, new(big.Rat).Inv(inner))
}
