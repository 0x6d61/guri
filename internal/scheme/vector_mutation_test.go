package scheme_test

import (
	"errors"
	"github.com/0x6d61/guri/internal/scheme"
	"testing"
)

func TestVectorAndBytevectorBulkMutation(t *testing.T) {
	v := scheme.NewVector(scheme.NewInteger(0), scheme.NewInteger(1), scheme.NewInteger(2))
	if err := v.CopyFrom(1, v, 0, 2); err != nil {
		t.Fatal(err)
	}
	if value, err := v.Ref(2); err != nil || value.(*scheme.Integer).String() != "1" {
		t.Fatal(value, err)
	}
	if err := v.Fill(1, 3, scheme.True); err != nil {
		t.Fatal(err)
	}
	if err := v.CopyFrom(2, v, 0, 2); !errors.Is(err, scheme.ErrIndex) {
		t.Fatal(err)
	}
	if value, err := v.Ref(2); err != nil || value != scheme.True {
		t.Fatal("partial copy", value, err)
	}
	b := scheme.NewBytevector([]byte{0, 1, 2})
	if err := b.CopyFrom(1, b, 0, 2); err != nil {
		t.Fatal(err)
	}
	if got := b.Bytes(); got[0] != 0 || got[1] != 0 || got[2] != 1 {
		t.Fatal(got)
	}
	empty := scheme.NewVector()
	scheme.FreezeLiteral(empty)
	if err := empty.Fill(0, 0, scheme.True); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal(err)
	}
	if err := empty.CopyFrom(0, empty, 0, 0); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal(err)
	}
	eb := scheme.NewBytevector(nil)
	scheme.FreezeLiteral(eb)
	if err := eb.CopyFrom(0, eb, 0, 0); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal(err)
	}
}
