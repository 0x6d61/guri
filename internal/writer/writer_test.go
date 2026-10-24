package writer_test

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/writer"
)

func datum(t *testing.T, src string) scheme.Value {
	t.Helper()
	ns, err := reader.ReadAll("graph.scm", src, nil)
	if err != nil || len(ns) != 1 {
		t.Fatal(src, err)
	}
	return ns[0].Datum()
}
func encode(t *testing.T, v scheme.Value, mode writer.Mode) string {
	t.Helper()
	var b strings.Builder
	if err := writer.Write(context.Background(), &b, v, mode); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
func TestGraphWritingModesAndRoundTrips(t *testing.T) {
	for _, src := range []string{`(a . b)`, `#(1 "a\n\"b" #\x3bb #u8(0 255))`, `(1/3 -0.0 3+4i +inf.0 +nan.0)`, `(|123| |two words| |λ| |a\|b| ||)`, `#0=(a . #0#)`, `#0=#(#0#)`, `#0=(#1=#(#0#) . #1#)`} {
		original := datum(t, src)
		for _, mode := range []writer.Mode{writer.Normal, writer.Shared} {
			output := encode(t, original, mode)
			copy := datum(t, output)
			if !equality.Equal(original, copy) {
				t.Fatal("roundtrip", src, output)
			}
			if strings.Contains(output, "#<cycle>") {
				t.Fatal(output)
			}
		}
	}
	original := datum(t, `(#0=(a b) #0#)`)
	if output := encode(t, original, writer.Normal); strings.Contains(output, "#0=") {
		t.Fatal("normal write labelled an acyclic object", output)
	}
	if output := encode(t, original, writer.Simple); strings.Contains(output, "#0=") {
		t.Fatal("simple write labelled sharing", output)
	}
	copied := datum(t, encode(t, original, writer.Shared)).(*scheme.Pair)
	if copied.Car() != copied.Cdr().(*scheme.Pair).Car() {
		t.Fatal("shared output lost identity")
	}
	cycle := datum(t, encode(t, datum(t, `#0=(a . #0#)`), writer.Normal)).(*scheme.Pair)
	if cycle.Cdr() != cycle {
		t.Fatal("cycle lost identity")
	}
}
func TestWriterDeepDataAndMutableSharing(t *testing.T) {
	var value scheme.Value = scheme.Null
	for i := 0; i < 100000; i++ {
		value = scheme.Cons(scheme.NewInteger(1), value)
	}
	output := encode(t, value, writer.Normal)
	if strings.Count(output, "1") != 100000 {
		t.Fatal("deep list")
	}
	s, err := scheme.NewString("λ\n")
	if err != nil {
		t.Fatal(err)
	}
	bytes := scheme.NewBytevector([]byte{0, 255})
	v := scheme.NewVector(s, s, bytes, bytes)
	restored := datum(t, encode(t, v, writer.Shared)).(*scheme.Vector)
	xs := restored.Values()
	if xs[0] != xs[1] || xs[2] != xs[3] {
		t.Fatal("mutable leaves lost identity")
	}
}

type failedWriter struct{ err error }

func (f failedWriter) Write([]byte) (int, error) { return 0, f.err }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return 0, nil }

type cancelWriter struct {
	cancel context.CancelFunc
	writes int
}

func (w *cancelWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == 32 {
		w.cancel()
	}
	return len(p), nil
}
func TestWriterFailuresAndSimpleCycleCancellation(t *testing.T) {
	sentinel := errors.New("sink failed")
	if err := writer.Write(context.Background(), failedWriter{sentinel}, scheme.True, writer.Normal); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if err := writer.Write(context.Background(), shortWriter{}, scheme.True, writer.Normal); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &cancelWriter{cancel: cancel}
	if err := writer.Write(ctx, out, datum(t, `#0=(a . #0#)`), writer.Simple); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if out.writes != 32 {
		t.Fatal("cancellation ignored", out.writes)
	}
}

func TestFiniteGraphRoundTrips(t *testing.T) {
	for seed := int64(0); seed < 50; seed++ {
		random := rand.New(rand.NewSource(seed))
		objects := make([]scheme.Value, 8)
		for i := range objects {
			if i%2 == 0 {
				objects[i] = scheme.Cons(scheme.False, scheme.Null)
			} else {
				objects[i] = scheme.NewVector(scheme.False, scheme.False)
			}
		}
		choose := func() scheme.Value {
			index := random.Intn(10)
			if index < 8 {
				return objects[index]
			}
			return scheme.NewInteger(int64(index))
		}
		for _, value := range objects {
			switch x := value.(type) {
			case *scheme.Pair:
				if err := x.SetCar(choose()); err != nil {
					t.Fatal(err)
				}
				if err := x.SetCdr(choose()); err != nil {
					t.Fatal(err)
				}
			case *scheme.Vector:
				for i := 0; i < 2; i++ {
					if err := x.Set(i, choose()); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		for _, mode := range []writer.Mode{writer.Normal, writer.Shared} {
			restored := datum(t, encode(t, objects[0], mode))
			if !equality.Equal(objects[0], restored) {
				t.Fatal("contents", seed, mode)
			}
			if mode != writer.Shared {
				continue
			}
			pending := [][2]scheme.Value{{objects[0], restored}}
			forward, reverse := map[scheme.Value]scheme.Value{}, map[scheme.Value]scheme.Value{}
			for len(pending) > 0 {
				item := pending[len(pending)-1]
				pending = pending[:len(pending)-1]
				a, b := item[0], item[1]
				if a.Kind() == scheme.KindNumber {
					continue
				}
				if old, ok := forward[a]; ok {
					if old != b {
						t.Fatal("lost sharing", seed)
					}
					continue
				}
				if old, ok := reverse[b]; ok && old != a {
					t.Fatal("merged identities", seed)
				}
				forward[a], reverse[b] = b, a
				switch x := a.(type) {
				case *scheme.Pair:
					y := b.(*scheme.Pair)
					pending = append(pending, [2]scheme.Value{x.Car(), y.Car()}, [2]scheme.Value{x.Cdr(), y.Cdr()})
				case *scheme.Vector:
					ys := b.(*scheme.Vector).Values()
					for i, v := range x.Values() {
						pending = append(pending, [2]scheme.Value{v, ys[i]})
					}
				}
			}
		}
	}
}
