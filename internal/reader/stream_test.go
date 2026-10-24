package reader_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/conformance"
	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

type selfDelimited struct{ sent bool }

func (s *selfDelimited) Read(b []byte) (int, error) {
	if s.sent {
		panic("read waited beyond complete datum")
	}
	s.sent = true
	return copy(b, "(1)"), nil
}
func TestStreamReaderDoesNotWaitAfterCompleteDatum(t *testing.T) {
	p := ports.Input(new(selfDelimited), false, ports.Options{})
	r := reader.NewStream(p, nil)
	n, err := r.Read()
	if err != nil || n.Datum().(*scheme.Pair).Car().(*scheme.Integer).BigInt().Int64() != 1 {
		t.Fatal(n, err)
	}
}
func TestStreamReaderSharesPositionAndDirectives(t *testing.T) {
	p := ports.InputString("#!fold-case FOO\r\nBAR #!no-fold-case BAZ")
	r := reader.NewStream(p, nil)
	for i, want := range []string{"foo", "bar", "BAZ"} {
		n, err := r.Read()
		if err != nil || n.Datum().(*scheme.Symbol).Name() != want {
			t.Fatal(n, err)
		}
		if i == 0 {
			if c, err := p.ReadChar(false); err != nil || c != '\r' {
				t.Fatal(c, err)
			}
			if c, err := p.ReadChar(false); err != nil || c != '\n' {
				t.Fatal(c, err)
			}
		}
		if i == 1 && n.Source().Line != 2 {
			t.Fatal(n.Source())
		}
	}
	if _, err := r.Read(); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

func TestStreamReaderMatchesSourceCorpus(t *testing.T) {
	for _, path := range []string{"../../testdata/r7rs/suite.json", "../../testdata/r7rs/chibi/suite.json"} {
		suite, err := conformance.LoadSuite(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range suite.Cases {
			t.Run(c.ID, func(t *testing.T) {
				want, err := reader.ReadAll(c.File, c.Source, nil)
				if err != nil {
					t.Fatal(err)
				}
				p := ports.Input(strings.NewReader(c.Source), false, ports.Options{Filename: c.File})
				r := reader.NewStream(p, nil)
				for i, node := range want {
					got, err := r.Read()
					if err != nil {
						t.Fatal(i, err)
					}
					if !equality.Equal(got.Datum(), node.Datum()) || got.Source() != node.Source() {
						t.Fatal(i, got.Source(), node.Source())
					}
				}
				if _, err := r.Read(); err != io.EOF {
					t.Fatal(err)
				}
			})
		}
	}
	for _, src := range []string{`#0=(1 . #0#) #1=#(#1#)`, `#; #0=(1 . #0#) (a . b) #u8(0 255)`, `"a\x3bb;" |a\x20;b| #\x3bb`, "#!fold-case FOO\r\nBAR #!no-fold-case BAZ"} {
		want, err := reader.ReadAll("<string-port>", src, nil)
		if err != nil {
			t.Fatal(err)
		}
		r := reader.NewStream(ports.InputString(src), nil)
		for _, node := range want {
			got, err := r.Read()
			if err != nil || !equality.Equal(got.Datum(), node.Datum()) || got.Source() != node.Source() {
				t.Fatal(src, err)
			}
		}
	}
}
