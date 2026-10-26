//go:build linux || darwin || windows

package ports_test

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/0x6d61/guri/internal/ports"
)

func TestFilePipeReadinessAndPartialCharacter(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	p := ports.FileInput(r, false, false)
	if ready, err := p.Ready(); err != nil || ready {
		t.Fatal("empty pipe", ready, err)
	}
	if _, err = w.Write([]byte{0xe7}); err != nil {
		t.Fatal(err)
	}
	if ready, err := p.Ready(); err != nil || ready {
		t.Fatal("partial character", ready, err)
	}
	if _, err = w.Write([]byte{0x8c, 0xab}); err != nil {
		t.Fatal(err)
	}
	if ready, err := p.Ready(); err != nil || !ready {
		t.Fatal("complete character", ready, err)
	}
	if x, err := p.ReadChar(true); err != nil || x != '猫' {
		t.Fatal(x, err)
	}
	if x, err := p.ReadChar(false); err != nil || x != '猫' {
		t.Fatal(x, err)
	}
	if ready, err := p.Ready(); err != nil || ready {
		t.Fatal("consumed character", ready, err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	if ready, err := p.Ready(); err != nil || !ready {
		t.Fatal("EOF", ready, err)
	}
	if _, err = p.ReadChar(false); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Stat(); err != nil {
		t.Fatal("closed borrowed file", err)
	}
}
func TestFileOwnership(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "owned")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("λ"); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	p := ports.FileInput(f, false, true)
	if ready, err := p.Ready(); err != nil || !ready {
		t.Fatal(ready, err)
	}
	if x, err := p.ReadChar(false); err != nil || x != 'λ' {
		t.Fatal(x, err)
	}
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("owned file left open", err)
	}
}

func TestFilePipeByteReadinessAndBufferedEOF(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	p := ports.FileInput(r, true, false)
	if ready, err := p.Ready(); ready || err != nil {
		t.Fatal("empty", ready, err)
	}
	if _, err := w.Write([]byte{0, 255}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []byte{0, 255} {
		if ready, err := p.Ready(); !ready || err != nil {
			t.Fatal("data before EOF", ready, err)
		}
		b, err := p.ReadU8(false)
		if err != nil || b != want {
			t.Fatal(b, err)
		}
	}
	if ready, err := p.Ready(); !ready || err != nil {
		t.Fatal("EOF", ready, err)
	}
	if _, err := p.ReadU8(false); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
