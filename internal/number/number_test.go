package number_test

import (
	"math"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/number"
	"github.com/0x6d61/guri/internal/scheme"
)

func parse(t *testing.T, s string) scheme.Number {
	t.Helper()
	n, err := scheme.ParseNumber(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
func call(t *testing.T, name string, args ...string) scheme.Results {
	t.Helper()
	var a []scheme.Value
	for _, s := range args {
		a = append(a, parse(t, s))
	}
	r, err := number.Call(name, a...)
	if err != nil {
		t.Fatal(name, args, err)
	}
	return r
}
func TestExactArithmeticAndDivision(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"+", []string{"9223372036854775808", "1"}, "9223372036854775809"}, {"+", nil, "0"}, {"*", nil, "1"}, {"-", []string{"3/2+i"}, "-3/2-i"}, {"/", []string{"3"}, "1/3"}, {"/", []string{"3", "4", "5"}, "3/20"}, {"*", []string{"1+2i", "3-4i"}, "11+2i"}, {"/", []string{"1+2i", "3-4i"}, "-1/5+2/5i"},
		{"floor-quotient", []string{"-5", "2"}, "-3"}, {"floor-remainder", []string{"-5", "2"}, "1"}, {"truncate-quotient", []string{"-5", "2"}, "-2"}, {"truncate-remainder", []string{"-5", "2"}, "-1"}, {"modulo", []string{"13", "-4"}, "-3"}, {"remainder", []string{"-13", "4"}, "-1"}, {"quotient", []string{"-5", "2"}, "-2"},
		{"gcd", []string{"32", "-36"}, "4"}, {"gcd", nil, "0"}, {"lcm", []string{"32", "-36"}, "288"}, {"lcm", nil, "1"}, {"floor", []string{"-7/2"}, "-4"}, {"ceiling", []string{"-7/2"}, "-3"}, {"truncate", []string{"-7/2"}, "-3"}, {"round", []string{"-7/2"}, "-4"}, {"round", []string{"5/2"}, "2"}, {"abs", []string{"-7"}, "7"}, {"square", []string{"3+4i"}, "-7+24i"}, {"expt", []string{"2", "-70"}, "1/1180591620717411303424"},
		{"numerator", []string{"3/2"}, "3"}, {"denominator", []string{"3/2"}, "2"}, {"sqrt", []string{"9/4"}, "3/2"}, {"sqrt", []string{"-1"}, "+i"}, {"magnitude", []string{"3+4i"}, "5"}, {"real-part", []string{"3+4i"}, "3"}, {"imag-part", []string{"3+4i"}, "4"}, {"make-rectangular", []string{"3", "4"}, "3+4i"}, {"rationalize", []string{"3/10", "1/10"}, "1/3"},
	} {
		t.Run(test.name+strings.Join(test.args, ","), func(t *testing.T) {
			r := call(t, test.name, test.args...)
			v, err := r.Single()
			if err != nil {
				t.Fatal(err)
			}
			want := parse(t, test.want)
			if !number.Equivalent(v.(scheme.Number), want) {
				t.Fatalf("got %v want %s", v, test.want)
			}
		})
	}
	r := call(t, "floor/", "-5", "-2")
	v := r.Values()
	if len(v) != 2 || v[0].(*scheme.Integer).String() != "2" || v[1].(*scheme.Integer).String() != "-1" {
		t.Fatal("floor values", r)
	}
	r = call(t, "exact-integer-sqrt", "5")
	v = r.Values()
	if len(v) != 2 || v[0].(*scheme.Integer).String() != "2" || v[1].(*scheme.Integer).String() != "1" {
		t.Fatal("sqrt values", r)
	}
}
func TestComparisonsPredicatesAndExactness(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want bool
	}{
		{"=", []string{"9007199254740993", "9007199254740992.0"}, false}, {">", []string{"9007199254740993", "9007199254740992.0"}, true}, {"<", []string{"1/10", "0.1"}, true}, {"=", []string{"1", "1.0"}, true}, {"<=", []string{"1", "1.0", "2"}, true}, {">=", []string{"2", "1"}, true}, {"=", []string{"+nan.0", "+nan.0"}, false},
		{"real?", []string{"-2.5+0i"}, true}, {"real?", []string{"-2.5+0.0i"}, false}, {"rational?", []string{"+inf.0"}, false}, {"integer?", []string{"3.0"}, true}, {"exact-integer?", []string{"3.0"}, false}, {"number?", []string{"1+2i"}, true}, {"complex?", []string{"1"}, true}, {"exact?", []string{"1/2"}, true}, {"inexact?", []string{"0.5"}, true}, {"positive?", []string{"1"}, true}, {"negative?", []string{"-1"}, true}, {"zero?", []string{"0.0+0.0i"}, true}, {"odd?", []string{"3.0"}, true}, {"even?", []string{"4"}, true}, {"finite?", []string{"3+4i"}, true}, {"infinite?", []string{"3.0+inf.0i"}, true}, {"nan?", []string{"3.0+nan.0i"}, true},
	} {
		t.Run(test.name+strings.Join(test.args, ","), func(t *testing.T) {
			v, err := call(t, test.name, test.args...).Single()
			if err != nil || v != scheme.Boolean(test.want) {
				t.Fatal(v, err)
			}
		})
	}
	v, err := call(t, "exact", "0.5").Single()
	if err != nil || v.(*scheme.Rational).String() != "1/2" {
		t.Fatal(v, err)
	}
	v, err = call(t, "inexact", "1/2").Single()
	if err != nil || v.(scheme.Float).Float64() != 0.5 {
		t.Fatal(v, err)
	}
	for _, name := range []string{"floor", "ceiling", "round", "truncate"} {
		v, err := call(t, name, "+inf.0").Single()
		if err != nil || !math.IsInf(v.(scheme.Float).Float64(), 1) {
			t.Fatal(name, v, err)
		}
	}
}
func TestFloatComplexBranchesAndFormatting(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want float64
	}{
		{"sin", []string{"1.5707963267948966"}, 1}, {"cos", []string{"0.0"}, 1}, {"tan", []string{"0.0"}, 0}, {"exp", []string{"0.0"}, 1}, {"log", []string{"100", "10"}, 2}, {"atan", []string{"1", "1"}, math.Pi / 4}, {"asin", []string{"1.0"}, math.Pi / 2}, {"acos", []string{"-1.0"}, math.Pi}, {"angle", []string{"+i"}, math.Pi / 2}, {"max", []string{"3", "4.0"}, 4}, {"min", []string{"3", "4.0"}, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			v, err := call(t, test.name, test.args...).Single()
			if err != nil {
				t.Fatal(err)
			}
			n := v.(scheme.Float).Float64()
			if math.Abs(n-test.want) > 1e-14 {
				t.Fatal(n, test.want)
			}
		})
	}
	for _, f := range []float64{0, math.Copysign(0, -1), math.SmallestNonzeroFloat64, math.MaxFloat64, math.Pi, math.Inf(1), math.NaN()} {
		for _, base := range []int{2, 8, 10, 16} {
			original := scheme.NewFloat(f)
			s, err := number.Format(original, base)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := scheme.ParseNumberRadix(s, base)
			if err != nil || !number.Equivalent(original, parsed) {
				t.Fatal("roundtrip", base, s, err)
			}
			if strings.Contains(s, "#x") || strings.Contains(s, "#b") {
				t.Fatal("explicit radix prefix", s)
			}
		}
	}
	v, err := call(t, "sqrt", "-1.0-0.0i").Single()
	if err != nil {
		t.Fatal(err)
	}
	r, i := v.(*scheme.Complex).Parts()
	if r.(scheme.Float).Float64() != 0 || i.(scheme.Float).Float64() != 1 {
		t.Fatal("principal sqrt")
	}
	v, err = call(t, "log", "-0.0").Single()
	if err != nil {
		t.Fatal(err)
	}
	r, i = v.(*scheme.Complex).Parts()
	if !math.IsInf(r.(scheme.Float).Float64(), -1) || i.(scheme.Float).Float64() != math.Pi {
		t.Fatal("log negative zero")
	}
}

func TestUnaryNegationSignedZeroAndConversions(t *testing.T) {
	for _, s := range []string{"0.0", "-0.0"} {
		v, _ := call(t, "-", s).Single()
		want := !math.Signbit(parse(t, s).(scheme.Float).Float64())
		if math.Signbit(v.(scheme.Float).Float64()) != want {
			t.Fatal("negation sign", s)
		}
	}
}

func TestNumericTextAndErrors(t *testing.T) {
	for _, tc := range []struct {
		text  string
		radix int64
		want  string
	}{
		{"101", 2, "5"}, {"#x10", 2, "16"}, {"#e0.5", 10, "1/2"}, {"+i", 10, "+i"},
	} {
		s, _ := scheme.NewString(tc.text)
		r, err := number.Call("string->number", s, scheme.NewInteger(tc.radix))
		if err != nil {
			t.Fatal(err)
		}
		v, _ := r.Single()
		if !number.Equivalent(v.(scheme.Number), parse(t, tc.want)) {
			t.Fatal(tc, v)
		}
	}
	for _, s := range []string{"", " 1", "1 ", "1/0", "#e+inf.0", "1 2", "#true", "hello"} {
		str, _ := scheme.NewString(s)
		r, err := number.Call("string->number", str)
		v, _ := r.Single()
		if err != nil || v != scheme.False {
			t.Fatal(s, v, err)
		}
	}
	for _, s := range []string{"3+4i", "3.0-0.0i", "0.0+nan.0i", "-0.0+inf.0i", "2/3-7/9i"} {
		for _, radix := range []int{2, 8, 10, 16} {
			n := parse(t, s)
			str, err := number.Format(n, radix)
			if err != nil {
				t.Fatal(err)
			}
			got, err := scheme.ParseNumberRadix(str, radix)
			if err != nil || !number.Equivalent(n, got) {
				t.Fatal(s, radix, str, err)
			}
		}
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"/", []string{"1", "0"}}, {"floor/", []string{"1", "0"}}, {"exact", []string{"+inf.0"}}, {"<", []string{"1+i", "2"}}, {"exact-integer-sqrt", []string{"-1"}},
	} {
		var args []scheme.Value
		for _, s := range tc.args {
			args = append(args, parse(t, s))
		}
		if _, err := number.Call(tc.name, args...); err == nil {
			t.Fatal("accepted invalid operation", tc)
		}
	}
	for _, name := range []string{"exact->inexact", "inexact->exact"} {
		r := call(t, name, "2")
		v, _ := r.Single()
		if !number.Equal(v.(scheme.Number), parse(t, "2")) {
			t.Fatal(name)
		}
	}
	if _, err := number.Call("+", scheme.True); err == nil {
		t.Fatal("accepted boolean")
	}
	if _, err := number.Call("/", scheme.NewInteger(1), scheme.NewInteger(2), scheme.True); err == nil {
		t.Fatal("accepted boolean")
	}
}
