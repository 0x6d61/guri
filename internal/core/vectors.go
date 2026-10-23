package core

import (
	"unicode/utf8"

	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

var vectorArities = func() map[string]pairArity {
	m := map[string]pairArity{}
	for _, name := range []string{"vector?", "bytevector?", "vector-length", "bytevector-length", "list->vector"} {
		m[name] = pairArity{1, 1}
	}
	for _, name := range []string{"vector", "bytevector", "vector-append", "bytevector-append"} {
		m[name] = pairArity{0, -1}
	}
	for _, name := range []string{"make-vector", "make-bytevector"} {
		m[name] = pairArity{1, 2}
	}
	for _, name := range []string{"vector-ref", "bytevector-u8-ref"} {
		m[name] = pairArity{2, 2}
	}
	for _, name := range []string{"vector-set!", "bytevector-u8-set!"} {
		m[name] = pairArity{3, 3}
	}
	for _, name := range []string{"vector-copy", "bytevector-copy", "vector->list", "vector->string", "string->vector", "utf8->string", "string->utf8"} {
		m[name] = pairArity{1, 3}
	}
	for _, name := range []string{"vector-copy!", "bytevector-copy!"} {
		m[name] = pairArity{3, 5}
	}
	m["vector-fill!"] = pairArity{2, 4}
	m["vector-map"] = pairArity{2, -1}
	m["vector-for-each"] = pairArity{2, -1}
	return m
}()

func (e *Engine) installVectors() {
	for name, arity := range vectorArities {
		e.Bind(name, vm.Native(name, arity.min, arity.max, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if name == "vector-map" || name == "vector-for-each" {
				return iterateVectors(c, name, a)
			}
			value, err := vectorPrimitive(name, a)
			if err != nil {
				return pairFailure(c, name, a, err)
			}
			return vm.Return(value), nil
		}))
	}
}
func vectorValue(v scheme.Value) (*scheme.Vector, error) {
	x, ok := v.(*scheme.Vector)
	if !ok {
		return nil, scheme.ErrType
	}
	return x, nil
}
func bytevectorValue(v scheme.Value) (*scheme.Bytevector, error) {
	x, ok := v.(*scheme.Bytevector)
	if !ok {
		return nil, scheme.ErrType
	}
	return x, nil
}
func byteValue(v scheme.Value) (byte, error) {
	n, ok := v.(*scheme.Integer)
	if !ok {
		return 0, scheme.ErrType
	}
	k := n.BigInt()
	if !k.IsInt64() || k.Sign() < 0 || k.Int64() > 255 {
		return 0, scheme.ErrRange
	}
	return byte(k.Int64()), nil
}
func vectorRange(length int, args []scheme.Value) (int, int, error) {
	start, end := 0, length
	var err error
	if len(args) > 0 {
		start, err = boundedIndex(args[0], length)
		if err != nil {
			return 0, 0, err
		}
	}
	if len(args) > 1 {
		end, err = boundedIndex(args[1], length)
		if err != nil {
			return 0, 0, err
		}
	}
	if start > end {
		return 0, 0, scheme.ErrIndex
	}
	return start, end, nil
}
func vectorPrimitive(name string, a []scheme.Value) (scheme.Value, error) {
	switch name {
	case "vector?":
		_, ok := a[0].(*scheme.Vector)
		return scheme.Boolean(ok), nil
	case "bytevector?":
		_, ok := a[0].(*scheme.Bytevector)
		return scheme.Boolean(ok), nil
	case "vector":
		return scheme.NewVector(a...), nil
	case "bytevector":
		out := make([]byte, len(a))
		for i, v := range a {
			b, err := byteValue(v)
			if err != nil {
				return nil, err
			}
			out[i] = b
		}
		return scheme.NewBytevector(out), nil
	case "make-vector", "make-bytevector":
		k, err := nonnegativeIndex(a[0])
		if err != nil {
			return nil, err
		}
		if !k.IsInt64() || int64(int(k.Int64())) != k.Int64() {
			return nil, scheme.ErrRange
		}
		count := int(k.Int64())
		if name == "make-vector" {
			var fill scheme.Value = scheme.Unspecified
			if len(a) == 2 {
				fill = a[1]
			}
			out := make([]scheme.Value, count)
			for i := range out {
				out[i] = fill
			}
			return scheme.NewVector(out...), nil
		}
		var fill byte
		if len(a) == 2 {
			fill, err = byteValue(a[1])
			if err != nil {
				return nil, err
			}
		}
		out := make([]byte, count)
		for i := range out {
			out[i] = fill
		}
		return scheme.NewBytevector(out), nil
	case "vector-length":
		v, err := vectorValue(a[0])
		if err != nil {
			return nil, err
		}
		return scheme.NewInteger(int64(v.Len())), nil
	case "bytevector-length":
		v, err := bytevectorValue(a[0])
		if err != nil {
			return nil, err
		}
		return scheme.NewInteger(int64(v.Len())), nil
	case "vector-ref", "vector-set!":
		v, err := vectorValue(a[0])
		if err != nil {
			return nil, err
		}
		k, err := boundedIndex(a[1], v.Len())
		if err != nil {
			return nil, err
		}
		if name == "vector-ref" {
			return v.Ref(k)
		}
		return scheme.Unspecified, v.Set(k, a[2])
	case "bytevector-u8-ref", "bytevector-u8-set!":
		v, err := bytevectorValue(a[0])
		if err != nil {
			return nil, err
		}
		k, err := boundedIndex(a[1], v.Len())
		if err != nil {
			return nil, err
		}
		if name == "bytevector-u8-ref" {
			b, err := v.Ref(k)
			if err != nil {
				return nil, err
			}
			return scheme.NewInteger(int64(b)), nil
		}
		b, err := byteValue(a[2])
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, v.Set(k, int64(b))
	case "vector-copy!":
		to, err := vectorValue(a[0])
		if err != nil {
			return nil, err
		}
		at, err := boundedIndex(a[1], to.Len())
		if err != nil {
			return nil, err
		}
		from, err := vectorValue(a[2])
		if err != nil {
			return nil, err
		}
		start, end, err := vectorRange(from.Len(), a[3:])
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, to.CopyFrom(at, from, start, end)
	case "bytevector-copy!":
		to, err := bytevectorValue(a[0])
		if err != nil {
			return nil, err
		}
		at, err := boundedIndex(a[1], to.Len())
		if err != nil {
			return nil, err
		}
		from, err := bytevectorValue(a[2])
		if err != nil {
			return nil, err
		}
		start, end, err := vectorRange(from.Len(), a[3:])
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, to.CopyFrom(at, from, start, end)
	case "vector-fill!":
		v, err := vectorValue(a[0])
		if err != nil {
			return nil, err
		}
		start, end, err := vectorRange(v.Len(), a[2:])
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, v.Fill(start, end, a[1])
	case "vector-copy", "vector->list", "vector->string":
		v, err := vectorValue(a[0])
		if err != nil {
			return nil, err
		}
		start, end, err := vectorRange(v.Len(), a[1:])
		if err != nil {
			return nil, err
		}
		values := v.Values()[start:end]
		if name == "vector-copy" {
			return scheme.NewVector(values...), nil
		}
		if name == "vector->list" {
			return scheme.List(values...), nil
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
	case "list->vector":
		values, err := properValues(a[0])
		if err != nil {
			return nil, err
		}
		return scheme.NewVector(values...), nil
	case "string->vector", "string->utf8":
		s, err := stringValue(a[0])
		if err != nil {
			return nil, err
		}
		start, end, err := vectorRange(s.Len(), a[1:])
		if err != nil {
			return nil, err
		}
		chars := []rune(s.Text())[start:end]
		if name == "string->utf8" {
			return scheme.NewBytevector([]byte(string(chars))), nil
		}
		values := make([]scheme.Value, len(chars))
		for i, r := range chars {
			c, err := scheme.NewCharacter(r)
			if err != nil {
				return nil, err
			}
			values[i] = c
		}
		return scheme.NewVector(values...), nil
	case "bytevector-copy", "utf8->string":
		v, err := bytevectorValue(a[0])
		if err != nil {
			return nil, err
		}
		start, end, err := vectorRange(v.Len(), a[1:])
		if err != nil {
			return nil, err
		}
		bytes := v.Bytes()[start:end]
		if name == "bytevector-copy" {
			return scheme.NewBytevector(bytes), nil
		}
		if !utf8.Valid(bytes) {
			return nil, scheme.ErrEncoding
		}
		return scheme.NewString(string(bytes))
	case "vector-append":
		var out []scheme.Value
		for _, v := range a {
			x, err := vectorValue(v)
			if err != nil {
				return nil, err
			}
			out = append(out, x.Values()...)
		}
		return scheme.NewVector(out...), nil
	case "bytevector-append":
		var out []byte
		for _, v := range a {
			x, err := bytevectorValue(v)
			if err != nil {
				return nil, err
			}
			out = append(out, x.Bytes()...)
		}
		return scheme.NewBytevector(out), nil
	}
	return nil, scheme.ErrType

}
