package scheme

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var ErrNumber = errors.New("invalid or unrepresentable number")
var decimalNumber = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:e[+-]?[0-9]+)?$`)

// ParseNumber consumes an entire R7RS numeric token. The same conversion is
// intended for the reader and string->number; it never falls back to Symbol.
func ParseNumber(text string) (Number, error) { return ParseNumberRadix(text, 10) }

// ParseNumberRadix applies a default radix; an explicit prefix overrides it.
func ParseNumberRadix(text string, radix int) (Number, error) {
	if radix != 2 && radix != 8 && radix != 10 && radix != 16 {
		return nil, ErrNumber
	}
	original := text
	text = strings.ToLower(text)
	exactness := byte(0)
	hasRadix := false
	bad := func() (Number, error) { return nil, fmt.Errorf("%w: %q", ErrNumber, original) }
	for strings.HasPrefix(text, "#") {
		if len(text) < 2 {
			return bad()
		}
		prefix := text[1]
		text = text[2:]
		switch prefix {
		case 'e', 'i':
			if exactness != 0 {
				return bad()
			}
			exactness = prefix
		case 'b', 'o', 'd', 'x':
			if hasRadix {
				return bad()
			}
			hasRadix = true
			radix = map[byte]int{'b': 2, 'o': 8, 'd': 10, 'x': 16}[prefix]
		default:
			return bad()
		}
	}
	if text == "" {
		return bad()
	}
	if strings.Contains(text, "@") {
		parts := strings.Split(text, "@")
		if len(parts) != 2 {
			return bad()
		}
		radius, err := parseReal(parts[0], radix, exactness)
		if err != nil {
			return bad()
		}
		angle, err := parseReal(parts[1], radix, exactness)
		if err != nil {
			return bad()
		}
		if radius.Exact() && exactZero(radius) && angle.Exact() {
			return radius, nil
		}
		if angle.Exact() && exactZero(angle) {
			return radius, nil
		}
		if exactness == 'e' {
			return nil, fmt.Errorf("%w: exact polar angle cannot be represented as rational components: %q", ErrNumber, original)
		}
		r, a := inexact(radius), inexact(angle)
		return NewComplex(NewFloat(r*math.Cos(a)), NewFloat(r*math.Sin(a)))
	}
	if strings.HasSuffix(text, "i") {
		body := text[:len(text)-1]
		split := -1
		for i := 1; i < len(body); i++ {
			if (body[i] == '+' || body[i] == '-') && !(radix == 10 && body[i-1] == 'e') {
				split = i
			}
		}
		var real Real = NewInteger(0)
		imagText := body
		if split >= 0 {
			var err error
			real, err = parseReal(body[:split], radix, exactness)
			if err != nil {
				return bad()
			}
			imagText = body[split:]
		} else if len(body) == 0 || (body[0] != '+' && body[0] != '-') {
			return bad()
		}
		if imagText == "+" || imagText == "-" {
			imagText += "1"
		}
		imag, err := parseReal(imagText, radix, exactness)
		if err != nil {
			return bad()
		}
		return NewComplex(real, imag)
	}
	real, err := parseReal(text, radix, exactness)
	if err != nil {
		return bad()
	}
	return real, nil
}

func parseReal(text string, radix int, exactness byte) (Real, error) {
	switch text {
	case "+inf.0", "-inf.0", "+nan.0", "-nan.0":
		if exactness == 'e' {
			return nil, ErrNumber
		}
		var f float64
		if strings.Contains(text, "inf") {
			f = math.Inf(1)
		} else {
			f = math.NaN()
		}
		if text[0] == '-' {
			f = math.Copysign(f, -1)
		}
		return NewFloat(f), nil
	}
	negative := strings.HasPrefix(text, "-")
	body := text
	if strings.HasPrefix(body, "+") || negative {
		body = body[1:]
	}
	var value Real
	switch {
	case strings.Contains(body, "/"):
		parts := strings.Split(body, "/")
		if len(parts) != 2 || !radixInteger(parts[0], radix) || !radixInteger(parts[1], radix) {
			return nil, ErrNumber
		}
		num, _ := new(big.Int).SetString(parts[0], radix)
		den, _ := new(big.Int).SetString(parts[1], radix)
		if negative {
			num.Neg(num)
		}
		var err error
		value, err = NewRational(num, den)
		if err != nil {
			return nil, err
		}
	case radixInteger(body, radix):
		integer, _ := new(big.Int).SetString(body, radix)
		if negative {
			integer.Neg(integer)
		}
		value, _ = IntegerFromBig(integer)
	case radix == 10 && decimalNumber.MatchString(body):
		if exactness == 'e' {
			q, ok := new(big.Rat).SetString(text)
			if !ok {
				return nil, ErrNumber
			}
			return NewRational(q.Num(), q.Denom())
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return nil, err
		}
		return NewFloat(f), nil
	default:
		return nil, ErrNumber
	}
	if exactness == 'i' {
		f := inexact(value)
		if negative && f == 0 {
			f = math.Copysign(0, -1)
		}
		return NewFloat(f), nil
	}
	return value, nil
}

func radixInteger(text string, radix int) bool {
	if text == "" {
		return false
	}
	for _, c := range text {
		digit := -1
		switch {
		case c >= '0' && c <= '9':
			digit = int(c - '0')
		case c >= 'a' && c <= 'f':
			digit = int(c-'a') + 10
		}
		if digit < 0 || digit >= radix {
			return false
		}
	}
	return true
}
