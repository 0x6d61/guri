package number

import (
	"math"
	"strconv"
	"strings"

	"github.com/0x6d61/guri/internal/scheme"
)

func validRadix(radix int) bool { return radix == 2 || radix == 8 || radix == 10 || radix == 16 }
func realText(r scheme.Real, radix int) string {
	if f, ok := r.(scheme.Float); ok {
		value := f.Float64()
		if math.IsInf(value, 1) {
			return "+inf.0"
		}
		if math.IsInf(value, -1) {
			return "-inf.0"
		}
		if math.IsNaN(value) {
			if math.Signbit(value) {
				return "-nan.0"
			}
			return "+nan.0"
		}
		if radix == 10 {
			s := strconv.FormatFloat(value, 'g', -1, 64)
			if !strings.Contains(s, ".") {
				if e := strings.IndexByte(s, 'e'); e >= 0 {
					s = s[:e] + ".0" + s[e:]
				} else {
					s += ".0"
				}
			}
			return s
		}
		if value == 0 && math.Signbit(value) {
			return "-0"
		}
	}
	q := rat(r)
	s := q.Num().Text(radix)
	if !q.IsInt() {
		s += "/" + q.Denom().Text(radix)
	}
	return s
}

// Format returns a round-trippable representation, never an explicit radix prefix.
func Format(n scheme.Number, radix int) (string, error) {
	if !validRadix(radix) {
		return "", scheme.ErrRange
	}
	var text string
	if r, ok := n.(scheme.Real); ok {
		text = realText(r, radix)
	} else {
		r, i := parts(n)
		real, imag := realText(r, radix), realText(i, radix)
		sign := "+"
		if strings.HasPrefix(imag, "-") {
			sign = "-"
			imag = imag[1:]
		} else {
			imag = strings.TrimPrefix(imag, "+")
		}
		text = real + sign + imag + "i"
	}
	if !n.Exact() && radix != 10 {
		text = "#i" + text
	}
	return text, nil
}
func textConversion(name string, a []scheme.Value) ([]scheme.Value, error) {
	radix := 10
	if len(a) == 2 {
		n, ok := a[1].(*scheme.Integer)
		if !ok || !n.BigInt().IsInt64() {
			return nil, scheme.ErrType
		}
		radix = int(n.BigInt().Int64())
		if !validRadix(radix) {
			return nil, scheme.ErrRange
		}
	}
	if name == "number->string" {
		n, err := asNumber(a[0])
		if err != nil {
			return nil, err
		}
		s, err := Format(n, radix)
		if err != nil {
			return nil, err
		}
		v, err := scheme.NewString(s)
		return one(v, err)
	}
	s, ok := a[0].(*scheme.String)
	if !ok {
		return nil, scheme.ErrType
	}
	n, err := scheme.ParseNumberRadix(s.Text(), radix)
	if err != nil {
		return one(scheme.False, nil)
	}
	return one(n, nil)
}
