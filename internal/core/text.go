package core

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/textcase"
	"github.com/0x6d61/guri/internal/vm"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var textArities = func() map[string]pairArity {
	out := map[string]pairArity{}
	for _, name := range []string{"char?", "string?", "char->integer", "integer->char", "char-upcase", "char-downcase", "char-foldcase", "char-alphabetic?", "char-numeric?", "char-whitespace?", "char-upper-case?", "char-lower-case?", "digit-value", "string-length", "string-upcase", "string-downcase", "string-foldcase", "list->string"} {
		out[name] = pairArity{1, 1}
	}
	for _, prefix := range []string{"char", "char-ci", "string", "string-ci"} {
		for _, op := range []string{"=?", "<?", "<=?", ">?", ">=?"} {
			out[prefix+op] = pairArity{2, -1}
		}
	}
	out["make-string"] = pairArity{1, 2}
	out["string"] = pairArity{0, -1}
	out["string-append"] = pairArity{0, -1}
	out["string-ref"] = pairArity{2, 2}
	out["string-set!"] = pairArity{3, 3}
	out["substring"] = pairArity{3, 3}
	out["string-copy"] = pairArity{1, 3}
	out["string->list"] = pairArity{1, 3}
	out["string-copy!"] = pairArity{3, 5}
	out["string-fill!"] = pairArity{2, 4}
	out["string-map"] = pairArity{2, -1}
	out["string-for-each"] = pairArity{2, -1}
	return out
}()

func (e *Engine) installText() {
	for name, arity := range textArities {
		e.Bind(name, vm.Native(name, arity.min, arity.max, func(c *vm.Context, args []scheme.Value) (vm.Action, error) {
			if name == "string-map" || name == "string-for-each" {
				return iterateStrings(c, name, args)
			}
			value, err := textPrimitive(name, args)
			if err != nil {
				return pairFailure(c, name, args, err)
			}
			return vm.Return(value), nil
		}))
	}
}

func charValue(v scheme.Value) (scheme.Character, error) {
	c, ok := v.(scheme.Character)
	if !ok {
		return scheme.Character{}, scheme.ErrType
	}
	return c, nil
}
func stringValue(v scheme.Value) (*scheme.String, error) {
	s, ok := v.(*scheme.String)
	if !ok {
		return nil, scheme.ErrType
	}
	return s, nil
}
func boundedIndex(v scheme.Value, limit int) (int, error) {
	k, err := nonnegativeIndex(v)
	if err != nil {
		return 0, err
	}
	if !k.IsInt64() || k.Int64() > int64(limit) {
		return 0, scheme.ErrIndex
	}
	return int(k.Int64()), nil
}
func textRange(s *scheme.String, a []scheme.Value) (int, int, error) {
	start, end := 0, s.Len()
	var err error
	if len(a) > 0 {
		start, err = boundedIndex(a[0], s.Len())
		if err != nil {
			return 0, 0, err
		}
	}
	if len(a) > 1 {
		end, err = boundedIndex(a[1], s.Len())
		if err != nil {
			return 0, 0, err
		}
	}
	if start > end {
		return 0, 0, scheme.ErrIndex
	}
	return start, end, nil
}
func decimalDigit(r rune) int {
	for _, interval := range unicode.Digit.R16 {
		if uint32(r) >= uint32(interval.Lo) && uint32(r) <= uint32(interval.Hi) && (uint32(r)-uint32(interval.Lo))%uint32(interval.Stride) == 0 {
			return int((uint32(r) - uint32(interval.Lo)) / uint32(interval.Stride) % 10)
		}
	}
	for _, interval := range unicode.Digit.R32 {
		if uint32(r) >= interval.Lo && uint32(r) <= interval.Hi && (uint32(r)-interval.Lo)%interval.Stride == 0 {
			return int((uint32(r) - interval.Lo) / interval.Stride % 10)
		}
	}
	return -1
}
func textPrimitive(name string, a []scheme.Value) (scheme.Value, error) {
	switch name {
	case "char?":
		_, ok := a[0].(scheme.Character)
		return scheme.Boolean(ok), nil
	case "string?":
		_, ok := a[0].(*scheme.String)
		return scheme.Boolean(ok), nil
	case "integer->char":
		k, ok := a[0].(*scheme.Integer)
		if !ok {
			return nil, scheme.ErrType
		}
		if !k.BigInt().IsInt64() || k.BigInt().Sign() < 0 || k.BigInt().Int64() > utf8.MaxRune {
			return nil, scheme.ErrRange
		}
		return scheme.NewCharacter(rune(k.BigInt().Int64()))
	case "char->integer", "char-upcase", "char-downcase", "char-foldcase", "char-alphabetic?", "char-numeric?", "char-whitespace?", "char-upper-case?", "char-lower-case?", "digit-value":
		c, err := charValue(a[0])
		if err != nil {
			return nil, err
		}
		r := c.Rune()
		switch name {
		case "char->integer":
			return scheme.NewInteger(int64(r)), nil
		case "char-upcase":
			return scheme.NewCharacter(unicode.ToUpper(r))
		case "char-downcase":
			return scheme.NewCharacter(unicode.ToLower(r))
		case "char-foldcase":
			return scheme.NewCharacter(textcase.Simple(r))
		case "char-alphabetic?":
			return scheme.Boolean(unicode.In(r, unicode.L, unicode.Nl, unicode.Other_Alphabetic)), nil
		case "char-numeric?":
			return scheme.Boolean(unicode.Is(unicode.Digit, r)), nil
		case "char-whitespace?":
			return scheme.Boolean(unicode.Is(unicode.White_Space, r)), nil
		case "char-upper-case?":
			return scheme.Boolean(unicode.In(r, unicode.Lu, unicode.Other_Uppercase)), nil
		case "char-lower-case?":
			return scheme.Boolean(unicode.In(r, unicode.Ll, unicode.Other_Lowercase)), nil
		default:
			value := decimalDigit(r)
			if value < 0 {
				return scheme.False, nil
			}
			return scheme.NewInteger(int64(value)), nil
		}
	case "string", "list->string":
		values := a
		if name == "list->string" {
			var err error
			values, err = properValues(a[0])
			if err != nil {
				return nil, err
			}
		}
		chars := make([]rune, len(values))
		for i, v := range values {
			c, err := charValue(v)
			if err != nil {
				return nil, err
			}
			chars[i] = c.Rune()
		}
		return scheme.NewString(string(chars))
	case "make-string":
		k, err := nonnegativeIndex(a[0])
		if err != nil {
			return nil, err
		}
		if !k.IsInt64() || int64(int(k.Int64())) != k.Int64() {
			return nil, scheme.ErrRange
		}
		var fill rune
		if len(a) == 2 {
			c, err := charValue(a[1])
			if err != nil {
				return nil, err
			}
			fill = c.Rune()
		}
		chars := make([]rune, int(k.Int64()))
		for i := range chars {
			chars[i] = fill
		}
		return scheme.NewString(string(chars))
	case "string-append":
		var out strings.Builder
		for _, v := range a {
			s, err := stringValue(v)
			if err != nil {
				return nil, err
			}
			out.WriteString(s.Text())
		}
		return scheme.NewString(out.String())
	case "string-copy!":
		to, err := stringValue(a[0])
		if err != nil {
			return nil, err
		}
		at, err := boundedIndex(a[1], to.Len())
		if err != nil {
			return nil, err
		}
		from, err := stringValue(a[2])
		if err != nil {
			return nil, err
		}
		start, end, err := textRange(from, a[3:])
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, to.CopyFrom(at, from, start, end)
	}
	if strings.HasSuffix(name, "?") {
		return compareText(name, a)
	}
	s, err := stringValue(a[0])
	if err != nil {
		return nil, err
	}
	switch name {
	case "string-length":
		return scheme.NewInteger(int64(s.Len())), nil
	case "string-ref", "string-set!":
		k, err := boundedIndex(a[1], s.Len())
		if err != nil {
			return nil, err
		}
		if name == "string-ref" {
			return s.Ref(k)
		}
		c, err := charValue(a[2])
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, s.Set(k, c)
	case "string-upcase":
		return scheme.NewString(cases.Upper(language.Und).String(s.Text()))
	case "string-downcase":
		return scheme.NewString(cases.Lower(language.Und).String(s.Text()))
	case "string-foldcase":
		return scheme.NewString(textcase.Fold(s.Text()))
	case "string-fill!":
		c, err := charValue(a[1])
		if err != nil {
			return nil, err
		}
		start, end, err := textRange(s, a[2:])
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, s.Fill(start, end, c)
	default:
		start, end, err := textRange(s, a[1:])
		if err != nil {
			return nil, err
		}
		chars := []rune(s.Text())[start:end]
		if name == "string->list" {
			values := make([]scheme.Value, len(chars))
			for i, r := range chars {
				c, err := scheme.NewCharacter(r)
				if err != nil {
					return nil, err
				}
				values[i] = c
			}
			return scheme.List(values...), nil
		}
		return scheme.NewString(string(chars))
	}
}
func compareText(name string, a []scheme.Value) (scheme.Value, error) {
	values := make([]string, len(a))
	character := strings.HasPrefix(name, "char")
	fold := strings.Contains(name, "-ci")
	for i, v := range a {
		if character {
			c, err := charValue(v)
			if err != nil {
				return nil, err
			}
			r := c.Rune()
			if fold {
				r = textcase.Simple(r)
			}
			values[i] = string(r)
		} else {
			s, err := stringValue(v)
			if err != nil {
				return nil, err
			}
			values[i] = s.Text()
			if fold {
				values[i] = textcase.Fold(values[i])
			}
		}
	}
	// Valid UTF-8 byte order is the lexicographic Unicode scalar-value order.
	prefix := "string"
	if character {
		prefix = "char"
	}
	if fold {
		prefix += "-ci"
	}
	op := strings.TrimPrefix(name, prefix)
	for i := 1; i < len(values); i++ {
		cmp := strings.Compare(values[i-1], values[i])
		match := op == "=?" && cmp == 0 || op == "<?" && cmp < 0 || op == "<=?" && cmp <= 0 || op == ">?" && cmp > 0 || op == ">=?" && cmp >= 0
		if !match {
			return scheme.False, nil
		}
	}
	return scheme.True, nil
}
