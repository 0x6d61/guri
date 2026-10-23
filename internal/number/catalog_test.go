package number_test

import (
	"github.com/0x6d61/guri/internal/number"
	"github.com/0x6d61/guri/internal/scheme"
	"testing"
)

func TestCatalogProceduresAndArity(t *testing.T) {
	// Every exported name is executed independently, including the compatibility
	// names belonging to (scheme r5rs), rather than treating registration as proof.
	samples := map[string][]string{}
	for _, name := range []string{"number?", "complex?", "real?", "rational?", "integer?", "exact-integer?", "exact?", "inexact?", "zero?", "positive?", "negative?", "even?", "odd?", "abs", "numerator", "denominator", "floor", "ceiling", "truncate", "round", "exact", "inexact", "exact->inexact", "inexact->exact", "square", "sqrt", "exact-integer-sqrt", "real-part", "imag-part", "magnitude", "angle", "exp", "sin", "cos", "tan", "asin", "acos", "finite?", "infinite?", "nan?"} {
		samples[name] = []string{"1"}
	}
	for _, name := range []string{"+", "-", "*", "/", "max", "min", "=", "<", "<=", ">", ">=", "gcd", "lcm", "floor/", "floor-quotient", "floor-remainder", "truncate/", "truncate-quotient", "truncate-remainder", "quotient", "remainder", "modulo", "rationalize", "expt", "make-rectangular", "make-polar", "atan", "log"} {
		samples[name] = []string{"2", "3"}
	}
	samples["number->string"] = []string{"2"}
	for name, arity := range number.Catalog() {
		t.Run(name, func(t *testing.T) {
			var args []scheme.Value
			if name == "string->number" {
				s, _ := scheme.NewString("2")
				args = []scheme.Value{s}
			} else {
				input, ok := samples[name]
				if !ok {
					t.Fatal("missing sample")
				}
				for _, s := range input {
					args = append(args, parse(t, s))
				}
			}
			if _, err := number.Call(name, args...); err != nil {
				t.Fatal(err)
			}
			if arity.Min > 0 {
				if _, err := number.Call(name); err == nil {
					t.Fatal("accepted too few arguments")
				}
			}
			if arity.Max >= 0 {
				extra := make([]scheme.Value, arity.Max+1)
				for i := range extra {
					extra[i] = scheme.NewInteger(1)
				}
				if _, err := number.Call(name, extra...); err == nil {
					t.Fatal("accepted too many arguments")
				}
			}
		})
	}
}
