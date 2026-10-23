package equality_test

import (
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/scheme"
	"testing"
)

func TestGraphUnfoldingsAndDeepStructures(t *testing.T) {
	x, y := scheme.Cons(scheme.NewInteger(1), scheme.Null), scheme.Cons(scheme.NewInteger(1), scheme.Null)
	if err := x.SetCar(x); err != nil {
		t.Fatal(err)
	}
	if err := y.SetCar(y); err != nil {
		t.Fatal(err)
	}
	if !equality.Equal(x, y) || equality.Eqv(x, y) {
		t.Fatal("car cycles")
	}
	var a, b scheme.Value = scheme.Null, scheme.Null
	for i := 0; i < 100000; i++ {
		a = scheme.Cons(a, scheme.Null)
		b = scheme.Cons(b, scheme.Null)
	}
	if !equality.Equal(a, b) {
		t.Fatal("deep comparison")
	}
	p := scheme.NewVector(scheme.NewInteger(1))
	shared := scheme.NewVector(p, p)
	fresh := scheme.NewVector(scheme.NewVector(scheme.NewInteger(1)), scheme.NewVector(scheme.NewInteger(1)))
	if !equality.Equal(shared, fresh) {
		t.Fatal("sharing changed tree equality")
	}
	if err := fresh.Set(1, scheme.NewVector(scheme.NewInteger(2))); err != nil {
		t.Fatal(err)
	}
	if equality.Equal(shared, fresh) {
		t.Fatal("different contents")
	}
	if !equality.Equal(scheme.NewBytevector([]byte{1, 2}), scheme.NewBytevector([]byte{1, 2})) {
		t.Fatal("bytevectors")
	}
}
