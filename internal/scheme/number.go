package scheme

import "math/big"

type Number interface {
	Value
	Exact() bool
	number()
}
type Real interface {
	Number
	real()
}

// Integer is immutable, including through its Go accessors.
type Integer struct{ value big.Int }

func (*Integer) Kind() Kind          { return KindNumber }
func (v *Integer) schemeValue() bool { return v != nil }
func (*Integer) number()             {}
func (*Integer) real()               {}
func (*Integer) Exact() bool         { return true }
func NewInteger(n int64) *Integer    { return &Integer{value: *big.NewInt(n)} }
func (n *Integer) BigInt() *big.Int  { return new(big.Int).Set(&n.value) }
func (n *Integer) String() string    { return n.value.String() }

func IntegerFromBig(n *big.Int) (*Integer, error) {
	if n == nil {
		return nil, ErrInvalidValue
	}
	return &Integer{value: *new(big.Int).Set(n)}, nil
}

// Rational is reduced and has a positive denominator. Integral fractions use
// Integer instead. No mutable math/big object escapes from an accessor.
type Rational struct{ value big.Rat }

func (*Rational) Kind() Kind          { return KindNumber }
func (v *Rational) schemeValue() bool { return v != nil }
func (*Rational) number()             {}
func (*Rational) real()               {}
func (*Rational) Exact() bool         { return true }
func NewRational(num, den *big.Int) (Real, error) {
	if num == nil || den == nil {
		return nil, ErrInvalidValue
	}
	if den.Sign() == 0 {
		return nil, ErrDivisionByZero
	}
	q := new(big.Rat).SetFrac(new(big.Int).Set(num), new(big.Int).Set(den))
	if q.IsInt() {
		return IntegerFromBig(q.Num())
	}
	return &Rational{value: *q}, nil
}
func (q *Rational) BigRat() *big.Rat { return new(big.Rat).Set(&q.value) }
func (q *Rational) String() string   { return q.value.RatString() }

// Float preserves the IEEE binary64 value, including signed zero, infinities
// and NaN. Printed and parsed representations belong to the numeric library.
type Float struct{ value float64 }

func (Float) Kind() Kind           { return KindNumber }
func (Float) schemeValue() bool    { return true }
func (Float) number()              {}
func (Float) real()                {}
func (Float) Exact() bool          { return false }
func NewFloat(value float64) Float { return Float{value} }
func (n Float) Float64() float64   { return n.value }

// Complex keeps exact rational parts, or two binary64 parts. Inexact imaginary
// zero is retained because its sign can affect complex functions' branch cuts.
type Complex struct{ real, imag Real }

func (*Complex) Kind() Kind          { return KindNumber }
func (v *Complex) schemeValue() bool { return v != nil }
func (*Complex) number()             {}
func (z *Complex) Exact() bool       { return z.real.Exact() && z.imag.Exact() }
func NewComplex(real, imag Real) (Number, error) {
	if real == nil || imag == nil || !real.schemeValue() || !imag.schemeValue() {
		return nil, ErrInvalidValue
	}
	if real.Exact() && imag.Exact() {
		if exactZero(imag) {
			return real, nil
		}
		return &Complex{real: real, imag: imag}, nil
	}
	return &Complex{real: NewFloat(inexact(real)), imag: NewFloat(inexact(imag))}, nil
}
func (z *Complex) Parts() (Real, Real) { return z.real, z.imag }

func exactZero(r Real) bool {
	switch n := r.(type) {
	case *Integer:
		return n.value.Sign() == 0
	case *Rational:
		return n.value.Sign() == 0
	}
	return false
}
func inexact(r Real) float64 {
	switch n := r.(type) {
	case Float:
		return n.value
	case *Integer:
		f, _ := new(big.Float).SetInt(&n.value).Float64()
		return f
	case *Rational:
		f, _ := n.value.Float64()
		return f
	}
	panic("scheme: unknown real representation")
}
