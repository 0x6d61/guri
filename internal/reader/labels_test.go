package reader_test

import (
	"errors"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"testing"
)

func TestDatumLabelsShareAndCycle(t *testing.T) {
	for _, src := range []string{`#1=(a . #1#)`, `#0001=(a . #1#)`, `#999999999999999999999999999=(a . #999999999999999999999999999#)`} {
		n := read(t, src)[0]
		p := n.Datum().(*scheme.Pair)
		if p.Cdr() != p {
			t.Fatal("lost pair cycle")
		}
		if err := p.SetCar(scheme.True); err != nil {
			t.Fatal("reader froze data", err)
		}
	}
	v := read(t, `#0=#(#0#)`)[0].Datum().(*scheme.Vector)
	x, err := v.Ref(0)
	if err != nil || x != v {
		t.Fatal("lost vector cycle", err)
	}
	xs := read(t, `(#0=(a b) #0# #1="hi" #1# #2=#u8(1 2) #2#)`)[0].Datum().(*scheme.Pair)
	first, second := xs.Car(), xs.Cdr().(*scheme.Pair).Car()
	if first != second {
		t.Fatal("lost shared tail")
	}
	xs = xs.Cdr().(*scheme.Pair).Cdr().(*scheme.Pair)
	if xs.Car() != xs.Cdr().(*scheme.Pair).Car() {
		t.Fatal("lost shared string")
	}
	xs = xs.Cdr().(*scheme.Pair).Cdr().(*scheme.Pair)
	if xs.Car() != xs.Cdr().(*scheme.Pair).Car() {
		t.Fatal("lost shared bytevector")
	}
	n := read(t, `#u8(#0=1 #0#)`)[0]
	if got := n.Datum().(*scheme.Bytevector).Bytes(); len(got) != 2 || got[1] != 1 {
		t.Fatal(got)
	}
}
func TestDatumLabelScopeAndDiagnostics(t *testing.T) {
	for _, src := range []string{`#1#`, `(#1# #1=a)`, `#1=#1#`, `#1=#2=#1#`, `#1=`, `#12x`, `(#1=a) #1#`, `#;#1=a #1#`, `(#;#1=a #1#)`, `#;#u8(256) 42`} {
		_, err := reader.ReadAll("labels.scm", src, nil)
		var diagnostic *reader.Error
		if !errors.As(err, &diagnostic) || diagnostic.Position.Filename != "labels.scm" || diagnostic.Position.Line < 1 || diagnostic.Position.Column < 1 {
			t.Fatalf("%s: %v", src, err)
		}
	}
	if got := read(t, `#1=a #1=b`); len(got) != 2 {
		t.Fatal("separate Read scopes")
	}
	if got := read(t, `#;#1=(a . #1#) 42`)[0].Datum().(*scheme.Integer).String(); got != "42" {
		t.Fatal(got)
	}
	if got := read(t, `(#1=a #;#2=b #1#)`)[0].Datum().(*scheme.Pair).Cdr().(*scheme.Pair).Car(); got.(*scheme.Symbol).Name() != "a" {
		t.Fatal(got)
	}
}
