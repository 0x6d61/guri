package scheme_test

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/0x6d61/guri/internal/scheme"
)

func TestArbitraryPrecisionIntegerOwnsItsStorage(t *testing.T) {
	input, ok := new(big.Int).SetString("92233720368547758081234567890", 10)
	if !ok {
		t.Fatal("test integer")
	}
	n, err := scheme.IntegerFromBig(input)
	if err != nil {
		t.Fatal(err)
	}
	input.SetInt64(0)
	copy := n.BigInt()
	copy.SetInt64(1)
	if n.String() != "92233720368547758081234567890" || !n.Exact() {
		t.Fatal("integer rounded or mutable through Go input/output")
	}
	if _, err := scheme.IntegerFromBig(nil); !errors.Is(err, scheme.ErrInvalidValue) {
		t.Fatal("nil big.Int accepted")
	}
}

func TestRationalNormalizationAndStorage(t *testing.T) {
	num, den := big.NewInt(-2), big.NewInt(-4)
	real, err := scheme.NewRational(num, den)
	if err != nil {
		t.Fatal(err)
	}
	q, ok := real.(*scheme.Rational)
	if !ok || q.String() != "1/2" || !q.Exact() {
		t.Fatalf("fraction normalization: %v", real)
	}
	num.SetInt64(9)
	den.SetInt64(9)
	external := q.BigRat()
	external.SetInt64(9)
	if q.String() != "1/2" {
		t.Fatal("rational backing storage exposed")
	}
	real, err = scheme.NewRational(big.NewInt(6), big.NewInt(3))
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := real.(*scheme.Integer); !ok || n.String() != "2" {
		t.Fatalf("integral rational not normalized: %v", real)
	}
	if _, err := scheme.NewRational(big.NewInt(1), big.NewInt(0)); !errors.Is(err, scheme.ErrDivisionByZero) {
		t.Fatalf("zero denominator: %v", err)
	}
}

func TestExactAndInexactComplexParts(t *testing.T) {
	half, _ := scheme.NewRational(big.NewInt(1), big.NewInt(2))
	threeQuarters, _ := scheme.NewRational(big.NewInt(3), big.NewInt(4))
	value, err := scheme.NewComplex(half, threeQuarters)
	if err != nil {
		t.Fatal(err)
	}
	z, ok := value.(*scheme.Complex)
	if !ok || !z.Exact() {
		t.Fatal("exact complex not supported")
	}
	r, i := z.Parts()
	if r.(*scheme.Rational).String() != "1/2" || i.(*scheme.Rational).String() != "3/4" {
		t.Fatal("complex parts rounded")
	}
	value, err = scheme.NewComplex(half, scheme.NewFloat(2))
	if err != nil {
		t.Fatal(err)
	}
	z = value.(*scheme.Complex)
	r, i = z.Parts()
	if z.Exact() || r.Exact() || i.Exact() || r.(scheme.Float).Float64() != 0.5 || i.(scheme.Float).Float64() != 2 {
		t.Fatal("mixed complex exactness")
	}
	value, err = scheme.NewComplex(scheme.NewInteger(5), scheme.NewInteger(0))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := value.(*scheme.Integer); !ok {
		t.Fatal("exact zero imaginary part not normalized")
	}
	value, err = scheme.NewComplex(scheme.NewInteger(5), scheme.NewFloat(math.Copysign(0, -1)))
	if err != nil {
		t.Fatal(err)
	}
	z, ok = value.(*scheme.Complex)
	if !ok {
		t.Fatal("inexact imaginary zero discarded")
	}
	_, i = z.Parts()
	if !math.Signbit(i.(scheme.Float).Float64()) {
		t.Fatal("negative imaginary zero lost")
	}
}

func TestIEEEFloatValuesAndKinds(t *testing.T) {
	for _, input := range []float64{0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN()} {
		n := scheme.NewFloat(input)
		got := n.Float64()
		if n.Exact() || n.Kind() != scheme.KindNumber {
			t.Fatal("float exactness or kind")
		}
		if math.IsNaN(input) {
			if !math.IsNaN(got) {
				t.Fatal("NaN lost")
			}
		} else if got != input || math.Signbit(got) != math.Signbit(input) {
			t.Fatal("float representation changed")
		}
	}
}

func TestInexactRealWithExactImaginaryZeroIsReal(t *testing.T) {
	n, err := scheme.NewComplex(scheme.NewFloat(-2.5), scheme.NewInteger(0))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := n.(scheme.Real); !ok {
		t.Fatal("exact imaginary zero did not produce a real")
	}
}
