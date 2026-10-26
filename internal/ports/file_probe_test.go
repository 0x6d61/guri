//go:build linux || darwin || windows

package ports

import (
	"errors"
	"io"
	"os"
	"testing"
)

func TestFileProbeDoesNotConsumeOrCloseBorrowedInput(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if ready, err := fileReady(r); ready || err != nil {
		t.Fatal("empty", ready, err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if ready, err := fileReady(r); !ready || err != nil {
			t.Fatal("available", ready, err)
		}
	}
	b := make([]byte, 1)
	if n, err := r.Read(b); n != 1 || err != nil || b[0] != 'x' {
		t.Fatal("native input consumed", n, b, err)
	}
	if ready, err := fileReady(r); ready || err != nil {
		t.Fatal("consumed", ready, err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if ready, err := fileReady(r); !ready || err != nil {
		t.Fatal("EOF", ready, err)
	}
	if _, err := r.Read(b); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := fileReady(r); err == nil {
		t.Fatal("closed", err)
	}
}
