package scheme

import "unicode/utf8"

// Pair has two independently mutable locations. Its cdr need not be a list.
type Pair struct {
	car, cdr Value
	frozen   bool
}

func (*Pair) Kind() Kind          { return KindPair }
func (v *Pair) schemeValue() bool { return v != nil }
func Cons(car, cdr Value) *Pair {
	requireValue(car)
	requireValue(cdr)
	return &Pair{car: car, cdr: cdr}
}
func (p *Pair) Car() Value { return p.car }
func (p *Pair) Cdr() Value { return p.cdr }
func (p *Pair) SetCar(v Value) error {
	requireValue(v)
	if p.frozen {
		return ErrImmutable
	}
	p.car = v
	return nil
}
func (p *Pair) SetCdr(v Value) error {
	requireValue(v)
	if p.frozen {
		return ErrImmutable
	}
	p.cdr = v
	return nil
}

// List allocates fresh pairs, returning Null for no arguments.
func List(values ...Value) Value {
	var result Value = Null
	for i := len(values) - 1; i >= 0; i-- {
		result = Cons(values[i], result)
	}
	return result
}

type Vector struct {
	values []Value
	frozen bool
}

func (*Vector) Kind() Kind          { return KindVector }
func (v *Vector) schemeValue() bool { return v != nil }
func NewVector(values ...Value) *Vector {
	for _, v := range values {
		requireValue(v)
	}
	return &Vector{values: append([]Value(nil), values...)}
}
func (v *Vector) Len() int { return len(v.values) }
func (v *Vector) Ref(i int) (Value, error) {
	if i < 0 || i >= len(v.values) {
		return nil, ErrIndex
	}
	return v.values[i], nil
}
func (v *Vector) Set(i int, x Value) error {
	requireValue(x)
	if v.frozen {
		return ErrImmutable
	}
	if i < 0 || i >= len(v.values) {
		return ErrIndex
	}
	v.values[i] = x
	return nil
}
func (v *Vector) Values() []Value { return append([]Value(nil), v.values...) }
func (v *Vector) Copy() *Vector   { return NewVector(v.values...) }

type Bytevector struct {
	values []byte
	frozen bool
}

func (*Bytevector) Kind() Kind          { return KindBytevector }
func (v *Bytevector) schemeValue() bool { return v != nil }
func NewBytevector(values []byte) *Bytevector {
	return &Bytevector{values: append([]byte(nil), values...)}
}
func (v *Bytevector) Len() int { return len(v.values) }
func (v *Bytevector) Ref(i int) (byte, error) {
	if i < 0 || i >= len(v.values) {
		return 0, ErrIndex
	}
	return v.values[i], nil
}
func (v *Bytevector) Set(i int, x int64) error {
	if v.frozen {
		return ErrImmutable
	}
	if i < 0 || i >= len(v.values) {
		return ErrIndex
	}
	if x < 0 || x > 255 {
		return ErrRange
	}
	v.values[i] = byte(x)
	return nil
}
func (v *Bytevector) Bytes() []byte     { return append([]byte(nil), v.values...) }
func (v *Bytevector) Copy() *Bytevector { return NewBytevector(v.values) }

// Character contains a Unicode scalar value, distinct from a one-character String.
type Character struct{ code rune }

func (Character) Kind() Kind        { return KindCharacter }
func (Character) schemeValue() bool { return true }
func NewCharacter(code rune) (Character, error) {
	if !utf8.ValidRune(code) {
		return Character{}, ErrRange
	}
	return Character{code}, nil
}
func (c Character) Rune() rune { return c.code }

// String uses a fixed-length rune array so changing UTF-8 width never changes
// its sequence of character locations. Constructors and accessors own storage.
type String struct {
	chars  []rune
	frozen bool
}

func (*String) Kind() Kind          { return KindString }
func (v *String) schemeValue() bool { return v != nil }
func NewString(text string) (*String, error) {
	if !utf8.ValidString(text) {
		return nil, ErrEncoding
	}
	return &String{chars: []rune(text)}, nil
}
func (s *String) Len() int     { return len(s.chars) }
func (s *String) Text() string { return string(s.chars) }
func (s *String) Ref(i int) (Character, error) {
	if i < 0 || i >= len(s.chars) {
		return Character{}, ErrIndex
	}
	return Character{s.chars[i]}, nil
}
func (s *String) Set(i int, c Character) error {
	if s.frozen {
		return ErrImmutable
	}
	if i < 0 || i >= len(s.chars) {
		return ErrIndex
	}
	s.chars[i] = c.code
	return nil
}
func (s *String) Copy() *String { return &String{chars: append([]rune(nil), s.chars...)} }
