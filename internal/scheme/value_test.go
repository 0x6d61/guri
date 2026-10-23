package scheme_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/0x6d61/guri/internal/scheme"
)

func TestOnlyFalseIsFalse(t *testing.T) {
	for _, v := range []scheme.Value{scheme.True, scheme.Null, scheme.EOF, scheme.Unspecified, scheme.NewInteger(0), scheme.NewVector(), scheme.NewBytevector(nil)} {
		if !scheme.Truthy(v) {
			t.Fatalf("%v treated as false", v)
		}
	}
	if scheme.Truthy(scheme.False) {
		t.Fatal("#f is true")
	}
	if scheme.Null.Kind() == scheme.False.Kind() || scheme.Null.Kind() == scheme.EOF.Kind() || scheme.Unspecified == scheme.Null {
		t.Fatal("singletons conflated")
	}
}

func TestPairSharingImproperAndCyclicLists(t *testing.T) {
	one, two := scheme.NewInteger(1), scheme.NewInteger(2)
	p := scheme.Cons(one, scheme.Null)
	alias := p
	if err := alias.SetCar(two); err != nil {
		t.Fatal(err)
	}
	if p.Car() != two {
		t.Fatal("pair update lost sharing")
	}
	if err := p.SetCdr(one); err != nil {
		t.Fatal(err)
	}
	if p.Cdr() != one {
		t.Fatal("improper pair rejected")
	}
	if err := p.SetCdr(p); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	if p.Cdr() != p || p.Car() != two {
		t.Fatal("cycle lost")
	}
}

func TestFixedLengthCollectionsAndShallowCopies(t *testing.T) {
	p := scheme.Cons(scheme.True, scheme.Null)
	input := []scheme.Value{p}
	v := scheme.NewVector(input...)
	input[0] = scheme.False
	copy := v.Copy()
	if v == copy || v.Len() != 1 {
		t.Fatal("copy reused vector identity")
	}
	if err := p.SetCar(scheme.False); err != nil {
		t.Fatal(err)
	}
	item, err := copy.Ref(0)
	if err != nil || item != p || item.(*scheme.Pair).Car() != scheme.False {
		t.Fatal("shallow sharing lost")
	}
	if err := copy.Set(0, scheme.True); err != nil {
		t.Fatal(err)
	}
	item, _ = v.Ref(0)
	if item != p {
		t.Fatal("copy shares vector storage")
	}
	if err := v.Set(1, scheme.True); !errors.Is(err, scheme.ErrIndex) {
		t.Fatalf("out of range: %v", err)
	}
	exposed := v.Values()
	exposed[0] = scheme.True
	item, _ = v.Ref(0)
	if item != p {
		t.Fatal("backing storage exposed")
	}
	raw := []byte{1, 2}
	b := scheme.NewBytevector(raw)
	raw[0] = 9
	bc := b.Copy()
	if err := bc.Set(0, 255); err != nil {
		t.Fatal(err)
	}
	got, _ := b.Ref(0)
	if got != 1 || bc == b {
		t.Fatal("bytevector copy or input ownership")
	}
	for _, n := range []int64{-1, 256} {
		if err := b.Set(0, n); !errors.Is(err, scheme.ErrRange) {
			t.Fatalf("u8 range: %v", err)
		}
	}
	if err := b.Set(2, 0); !errors.Is(err, scheme.ErrIndex) {
		t.Fatal("bytevector grew")
	}
	bytes := b.Bytes()
	bytes[0] = 9
	got, _ = b.Ref(0)
	if got != 1 {
		t.Fatal("byte backing exposed")
	}
}

func TestMutableUnicodeStringAndCharacterBoundary(t *testing.T) {
	s, err := scheme.NewString("A日本")
	if err != nil {
		t.Fatal(err)
	}
	if s.Len() != 3 {
		t.Fatal("string length used bytes")
	}
	ch, err := s.Ref(1)
	if err != nil || ch.Rune() != '日' {
		t.Fatal("character access")
	}
	emoji, err := scheme.NewCharacter('😀')
	if err != nil {
		t.Fatal(err)
	}
	alias := s
	copy := s.Copy()
	if err := alias.Set(1, emoji); err != nil {
		t.Fatal(err)
	}
	if s.Text() != "A😀本" || s.Len() != 3 || copy.Text() != "A日本" {
		t.Fatal("string sharing/copy/fixed length")
	}
	for _, r := range []rune{-1, 0xd800, 0x110000} {
		if _, err := scheme.NewCharacter(r); !errors.Is(err, scheme.ErrRange) {
			t.Fatalf("invalid character %x: %v", r, err)
		}
	}
	if _, err := scheme.NewString(string([]byte{0xff})); !errors.Is(err, scheme.ErrEncoding) {
		t.Fatalf("invalid UTF-8: %v", err)
	}
	if _, err := s.Ref(3); !errors.Is(err, scheme.ErrIndex) {
		t.Fatal("string out of bounds")
	}
	if ch.Kind() == s.Kind() {
		t.Fatal("character and string conflated")
	}
}

func TestLiteralGraphIsImmutableAndCopiesAreMutable(t *testing.T) {
	s, _ := scheme.NewString("a")
	b := scheme.NewBytevector([]byte{1})
	v := scheme.NewVector(s, b)
	p := scheme.Cons(v, scheme.Null)
	p.SetCdr(p)
	v.Set(0, p)
	scheme.FreezeLiteral(p)
	if err := p.SetCar(scheme.True); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal("literal pair mutable")
	}
	if err := v.Set(0, scheme.Null); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal("literal vector mutable")
	}
	if err := b.Set(0, 2); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal("nested literal bytevector mutable")
	}
	c, _ := scheme.NewCharacter('b')
	scheme.FreezeLiteral(s)
	if err := s.Set(0, c); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal("literal string mutable")
	}
	if err := v.Copy().Set(0, scheme.True); err != nil {
		t.Fatal("vector copy frozen")
	}
	if err := s.Copy().Set(0, c); err != nil {
		t.Fatal("string copy frozen")
	}
	if err := b.Copy().Set(0, 3); err != nil {
		t.Fatal("bytevector copy frozen")
	}
}

func TestGoNilCannotEnterSchemeValues(t *testing.T) {
	var pair *scheme.Pair
	for _, v := range []scheme.Value{nil, pair, (*scheme.Integer)(nil), (*scheme.String)(nil)} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("Go nil accepted as Scheme value")
				}
			}()
			scheme.NewVector(v)
		}()
	}
}
