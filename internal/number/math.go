package number

import (
	"math"
	"math/big"
	"math/cmplx"

	"github.com/0x6d61/guri/internal/scheme"
)

func exactRoot(q *big.Rat) (scheme.Real, bool) {
	if q.Sign() < 0 {
		return nil, false
	}
	n, d := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
	if new(big.Int).Mul(n, n).Cmp(q.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(q.Denom()) != 0 {
		return nil, false
	}
	return rational(new(big.Rat).SetFrac(n, d)), true
}
func squareRoot(n scheme.Number) scheme.Number {
	if r, ok := n.(scheme.Real); ok {
		if r.Exact() {
			q := rat(r)
			if q.Sign() < 0 {
				if root, ok := exactRoot(new(big.Rat).Neg(q)); ok {
					v, err := scheme.NewComplex(scheme.NewInteger(0), root)
					if err != nil {
						panic(err)
					}
					return v
				}
			} else if root, ok := exactRoot(q); ok {
				return root
			}
		}
		f := floating(r)
		if f >= 0 || math.IsNaN(f) {
			return scheme.NewFloat(math.Sqrt(f))
		}
	}
	c := complexValue(n)
	// R7RS requires non-negative imaginary part when the result's real part is zero.
	if imag(c) == 0 && real(c) < 0 {
		return fromComplex(complex(0, math.Sqrt(-real(c))))
	}
	return fromComplex(cmplx.Sqrt(c))
}
func power(a, b scheme.Number) (scheme.Number, error) {
	if Equal(a, scheme.NewInteger(0)) {
		var result scheme.Number = scheme.NewInteger(1)
		if !Equal(b, scheme.NewInteger(0)) {
			r, _ := parts(b)
			cmp, ok := Compare(r, scheme.NewInteger(0))
			if !ok || cmp <= 0 {
				return nil, ErrDomain
			}
			result = scheme.NewInteger(0)
		}
		if !a.Exact() || !b.Exact() {
			return convert(result, false)
		}
		return result, nil
	}
	if exponent, ok := b.(*scheme.Integer); ok && a.Exact() {
		n := exponent.BigInt()
		negative := n.Sign() < 0
		n.Abs(n)
		var result scheme.Number = scheme.NewInteger(1)
		base := a
		for bit := 0; bit < n.BitLen(); bit++ {
			var err error
			if n.Bit(bit) != 0 {
				result, err = binary("*", result, base)
				if err != nil {
					return nil, err
				}
			}
			if bit+1 < n.BitLen() {
				base, err = binary("*", base, base)
				if err != nil {
					return nil, err
				}
			}
		}
		if negative {
			return binary("/", scheme.NewInteger(1), result)
		}
		return result, nil
	}
	x, xReal := a.(scheme.Real)
	y, yReal := b.(scheme.Real)
	if xReal && yReal {
		xf, yf := floating(x), floating(y)
		if xf >= 0 || math.Trunc(yf) == yf {
			return scheme.NewFloat(math.Pow(xf, yf)), nil
		}
	}
	return fromComplex(cmplx.Pow(complexValue(a), complexValue(b))), nil
}
func mathematical(name string, a []scheme.Number) ([]scheme.Value, error) {
	n := a[0]
	switch name {
	case "make-rectangular", "make-polar":
		r, ok := n.(scheme.Real)
		if !ok {
			return nil, scheme.ErrType
		}
		i, ok := a[1].(scheme.Real)
		if !ok {
			return nil, scheme.ErrType
		}
		if name == "make-rectangular" {
			v, err := scheme.NewComplex(r, i)
			return one(v, err)
		}
		if r.Exact() && i.Exact() && (Equal(r, scheme.NewInteger(0)) || Equal(i, scheme.NewInteger(0))) {
			return one(r, nil)
		}
		radius, angle := floating(r), floating(i)
		return one(fromComplex(complex(radius*math.Cos(angle), radius*math.Sin(angle))), nil)
	case "real-part", "imag-part":
		r, i := parts(n)
		if name == "real-part" {
			return one(r, nil)
		}
		return one(i, nil)
	case "angle":
		if n.Exact() && Equal(n, scheme.NewInteger(0)) {
			return nil, ErrDomain
		}
		return one(scheme.NewFloat(cmplx.Phase(complexValue(n))), nil)
	case "magnitude":
		if r, ok := n.(scheme.Real); ok {
			if r.Exact() {
				return one(rational(new(big.Rat).Abs(rat(r))), nil)
			}
			return one(scheme.NewFloat(math.Abs(floating(r))), nil)
		}
		if n.Exact() {
			r, i := parts(n)
			sum := new(big.Rat).Add(new(big.Rat).Mul(rat(r), rat(r)), new(big.Rat).Mul(rat(i), rat(i)))
			return one(squareRoot(rational(sum)), nil)
		}
		return one(scheme.NewFloat(cmplx.Abs(complexValue(n))), nil)
	case "sqrt":
		return one(squareRoot(n), nil)
	case "expt":
		v, err := power(n, a[1])
		return one(v, err)
	}
	if name == "atan" && len(a) == 2 {
		y, ok := n.(scheme.Real)
		if !ok {
			return nil, scheme.ErrType
		}
		x, ok := a[1].(scheme.Real)
		if !ok {
			return nil, scheme.ErrType
		}
		yf, xf := floating(y), floating(x)
		if x.Exact() && Equal(x, scheme.NewInteger(0)) && yf == 0 {
			if y.Exact() {
				return nil, ErrDomain
			}
			return one(scheme.NewFloat(math.Copysign(math.Pi/2, yf)), nil)
		}
		return one(scheme.NewFloat(math.Atan2(yf, xf)), nil)
	}
	if name == "log" && len(a) == 2 {
		left, err := mathematical("log", a[:1])
		if err != nil {
			return nil, err
		}
		right, err := mathematical("log", a[1:])
		if err != nil {
			return nil, err
		}
		v, err := binary("/", left[0].(scheme.Number), right[0].(scheme.Number))
		return one(v, err)
	}
	if n.Exact() {
		if Equal(n, scheme.NewInteger(0)) {
			switch name {
			case "sin", "tan", "asin", "atan":
				return one(scheme.NewInteger(0), nil)
			case "exp", "cos":
				return one(scheme.NewInteger(1), nil)
			case "log":
				return nil, ErrDomain
			}
		}
		if Equal(n, scheme.NewInteger(1)) && (name == "log" || name == "acos") {
			return one(scheme.NewInteger(0), nil)
		}
	}
	if r, ok := n.(scheme.Real); ok {
		f := floating(r)
		var value float64
		switch name {
		case "exp":
			value = math.Exp(f)
		case "sin":
			value = math.Sin(f)
		case "cos":
			value = math.Cos(f)
		case "tan":
			value = math.Tan(f)
		case "asin":
			if f < -1 || f > 1 {
				return one(fromComplex(cmplx.Asin(complex(f, 0))), nil)
			}
			value = math.Asin(f)
		case "acos":
			if f < -1 || f > 1 {
				return one(fromComplex(cmplx.Acos(complex(f, 0))), nil)
			}
			value = math.Acos(f)
		case "atan":
			value = math.Atan(f)
		case "log":
			if f < 0 || f == 0 && math.Signbit(f) {
				return one(fromComplex(cmplx.Log(complex(f, 0))), nil)
			}
			value = math.Log(f)
		default:
			return nil, ErrDomain
		}
		return one(scheme.NewFloat(value), nil)
	}
	c := complexValue(n)
	var result complex128
	switch name {
	case "exp":
		result = cmplx.Exp(c)
	case "sin":
		result = cmplx.Sin(c)
	case "cos":
		result = cmplx.Cos(c)
	case "tan":
		result = cmplx.Tan(c)
	case "asin":
		result = cmplx.Asin(c)
	case "acos":
		result = cmplx.Acos(c)
	case "atan":
		result = cmplx.Atan(c)
	case "log":
		result = cmplx.Log(c)
	default:
		return nil, ErrDomain
	}
	return one(fromComplex(result), nil)
}
