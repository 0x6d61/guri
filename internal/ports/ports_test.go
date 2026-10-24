package ports_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/0x6d61/guri/internal/ports"
)

type resource struct {
	bytes.Buffer
	closes, flushes    int
	closeErr, flushErr error
}

func (r *resource) Close() error { r.closes++; return r.closeErr }
func (r *resource) Flush() error { r.flushes++; return r.flushErr }

func TestOwnershipFlushAndIdempotentClose(t *testing.T) {
	for _, owned := range []bool{false, true} {
		r := new(resource)
		opts := ports.Options{}
		if owned {
			opts.Owned = r
		}
		p := ports.Output(r, false, opts)
		if err := p.WriteText("猫"); err != nil {
			t.Fatal(err)
		}
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		expected := 0
		if owned {
			expected = 1
		}
		if r.closes != expected || r.flushes != 1 || r.String() != "猫" {
			t.Fatal(owned, r)
		}
		if err := p.WriteText(""); !errors.Is(err, ports.ErrClosed) {
			t.Fatal(err)
		}
	}
	r := &resource{closeErr: errors.New("close failure"), flushErr: errors.New("flush failure")}
	p := ports.Output(r, true, ports.Options{Owned: r})
	err := p.Close()
	if !errors.Is(err, r.closeErr) || !errors.Is(err, r.flushErr) || r.closes != 1 || p.Open() {
		t.Fatal(err, r)
	}
	if err = p.Close(); err != nil || r.closes != 1 {
		t.Fatal(err, r.closes)
	}
}

type fragments struct {
	chunks [][]byte
	final  error
}

func (f *fragments) Read(b []byte) (int, error) {
	if len(f.chunks) == 0 {
		return 0, f.final
	}
	n := copy(b, f.chunks[0])
	f.chunks[0] = f.chunks[0][n:]
	if len(f.chunks[0]) == 0 {
		f.chunks = f.chunks[1:]
	}
	return n, nil
}
func TestPartialUTF8ReadinessPeekAndEOF(t *testing.T) {
	f := &fragments{chunks: [][]byte{{0xe7}, {0x8c}, {0xab}}, final: io.EOF}
	available := 1
	p := ports.Input(f, false, ports.Options{Ready: func() (bool, error) { return available > 0, nil }})
	// One ready byte does not imply one ready Unicode character.
	partialAvailable := 1
	p2 := ports.Input(&fragments{chunks: [][]byte{{0xe7}}, final: io.EOF}, false, ports.Options{Ready: func() (bool, error) { partialAvailable--; return partialAvailable >= 0, nil }})
	ready, err := p2.Ready()
	if err != nil || ready {
		t.Fatal(ready, err)
	}
	ready, err = p.Ready()
	if err != nil || !ready {
		t.Fatal(ready, err)
	}
	r, err := p.ReadChar(true)
	if err != nil || r != '猫' {
		t.Fatal(r, err)
	}
	r, err = p.ReadChar(false)
	if err != nil || r != '猫' {
		t.Fatal(r, err)
	}
	if _, err = p.ReadChar(false); err != io.EOF {
		t.Fatal(err)
	}
	if ready, err = p.Ready(); err != nil || !ready || !p.Open() {
		t.Fatal(ready, err)
	}
}
func TestInvalidUTF8AndPendingReadError(t *testing.T) {
	for _, b := range [][]byte{{0xff}, {0xe7, 0x8c}, {0xc0, 0x80}} {
		p := ports.Input(bytes.NewReader(b), false, ports.Options{Ready: func() (bool, error) { return true, nil }})
		if _, err := p.ReadChar(false); !errors.Is(err, ports.ErrEncoding) {
			t.Fatal(b, err)
		}
	}
	sentinel := errors.New("device failed")
	p := ports.Input(&dataAndError{sentinel}, true, ports.Options{})
	b, err := p.ReadU8(false)
	if b != 42 || err != nil {
		t.Fatal(b, err)
	}
	if _, err = p.ReadU8(false); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}

type dataAndError struct{ err error }

func (r *dataAndError) Read(b []byte) (int, error) { b[0] = 42; return 1, r.err }

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) / 2, nil }
func TestShortWritesAndDirection(t *testing.T) {
	p := ports.Output(shortWriter{}, true, ports.Options{})
	if err := p.WriteBytes([]byte{1, 2, 3}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if _, err := p.ReadU8(false); err == nil {
		t.Fatal("read output port")
	}
	p = ports.Input(bytes.NewReader(nil), true, ports.Options{})
	if err := p.Flush(); err == nil {
		t.Fatal("flush input")
	}
	if err := p.WriteBytes(nil); err == nil {
		t.Fatal("write input")
	}
}
