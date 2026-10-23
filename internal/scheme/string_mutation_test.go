package scheme_test

import (
	"errors"
	"github.com/0x6d61/guri/internal/scheme"
	"testing"
)

func TestStringBulkMutationAndEmptyConstants(t *testing.T) {
	s, err := scheme.NewString("aλ😀bc")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CopyFrom(1, s, 0, 4); err != nil {
		t.Fatal(err)
	}
	if s.Text() != "aaλ😀b" {
		t.Fatal(s.Text())
	}
	c, err := scheme.NewCharacter('雪')
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Fill(1, 3, c); err != nil {
		t.Fatal(err)
	}
	if s.Text() != "a雪雪😀b" {
		t.Fatal(s.Text())
	}
	before := s.Text()
	if err := s.CopyFrom(4, s, 0, 2); !errors.Is(err, scheme.ErrIndex) || s.Text() != before {
		t.Fatal("partial invalid copy", err)
	}
	empty, err := scheme.NewString("")
	if err != nil {
		t.Fatal(err)
	}
	scheme.FreezeLiteral(empty)
	if err := empty.CopyFrom(0, s, 0, 0); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal(err)
	}
	if err := empty.Fill(0, 0, c); !errors.Is(err, scheme.ErrImmutable) {
		t.Fatal(err)
	}
}
