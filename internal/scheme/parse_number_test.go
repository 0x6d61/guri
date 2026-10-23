package scheme_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/0x6d61/guri/internal/scheme"
)

func numericRepr(n scheme.Number) string {
	switch v := n.(type) {
	case *scheme.Integer:
		return v.String()
	case *scheme.Rational:
		return v.String()
	case scheme.Float:
		return "~" + strconv.FormatFloat(v.Float64(), 'g', -1, 64)
	case *scheme.Complex:
		r, i := v.Parts()
		return "(" + numericRepr(r) + "," + numericRepr(i) + ")"
	}
	panic("unknown numeric representation")
}

func TestReadNumericRepresentations(t *testing.T) {
	cases := map[string]string{
		"42": "42", "-3": "-3", "9223372036854775808123": "9223372036854775808123",
		"#b101": "5", "#o10": "8", "#d10": "10", "#XfF": "255", "#e#xFF": "255", "#x#eFF": "255", "#i#b10": "~2",
		"3.14": "~3.14", "2e3": "~2000", "#e3.14": "157/50", "#e1.25e2": "125", "#e.5": "1/2", "#e1e-2": "1/100", "#e1.": "1",
		"-2/4": "-1/2", "#i1/2": "~0.5", "-0.0": "~-0", "#i-0": "~-0", "+inf.0": "~+Inf", "#i-inf.0": "~-Inf",
		"+i": "(0,1)", "-i": "(0,-1)", "1+2i": "(1,2)", "1/2-3/4i": "(1/2,-3/4)", "#x1e+2i": "(30,2)", "1e-2+2e+3i": "(~0.01,~2000)", "#e1.5+2.5i": "(3/2,5/2)", "#e2@0": "2", "#e0@1": "0",
	}
	for source, want := range cases {
		t.Run(source, func(t *testing.T) {
			n, err := scheme.ParseNumber(source)
			if err != nil {
				t.Fatal(err)
			}
			if got := numericRepr(n); got != want {
				t.Fatalf("%s: got %s want %s", source, got, want)
			}
		})
	}
	n, err := scheme.ParseNumber("+nan.0")
	if err != nil || !math.IsNaN(n.(scheme.Float).Float64()) {
		t.Fatalf("NaN: %v", err)
	}
	n, err = scheme.ParseNumber("2@1")
	if err != nil {
		t.Fatal(err)
	}
	r, i := n.(*scheme.Complex).Parts()
	if math.Abs(r.(scheme.Float).Float64()-2*math.Cos(1)) > 1e-14 || math.Abs(i.(scheme.Float).Float64()-2*math.Sin(1)) > 1e-14 {
		t.Fatal("polar components")
	}
}

func TestRejectInvalidNumericRepresentations(t *testing.T) {
	for _, source := range []string{"", "foo", "i", "#x", "#q1", "#e#e1", "#x#d1", "1/0", "1/-2", "--1", "1+2", "2i", "1e", "1e+", "1s2", "#b102", "+inf.0foo", "#e+nan.0", "#e1@1", "1@@2", "1+2+3i", "0x10", "1_000", "#e1.2.3"} {
		t.Run(source, func(t *testing.T) {
			if _, err := scheme.ParseNumber(source); err == nil {
				t.Fatalf("invalid number accepted: %q", source)
			}
		})
	}
}
