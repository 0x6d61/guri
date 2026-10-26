//go:build linux || darwin

package ports

import (
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"golang.org/x/term"
)

func ttyReady(t *testing.T, p *Resource) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		ready, err := p.Ready()
		if err != nil {
			t.Fatal(err)
		}
		if ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("terminal never became ready")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestTTYCanonicalReadinessAndEOF(t *testing.T) {
	master, slave := testPTY(t)
	p := FileInput(slave, false, false)
	mode, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if ready, err := p.Ready(); ready || err != nil {
		t.Fatal("empty terminal", ready, err)
	}
	if _, err := master.WriteString("x"); err != nil {
		t.Fatal(err)
	}
	if ready, err := p.Ready(); ready || err != nil {
		t.Fatal("incomplete line", ready, err)
	}
	if _, err := master.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	ttyReady(t, p)
	if got, err := p.ReadChar(false); got != 'x' || err != nil {
		t.Fatal(got, err)
	}
	if got, err := p.ReadChar(false); got != '\n' || err != nil {
		t.Fatal(got, err)
	}
	if _, err := master.Write([]byte{4}); err != nil {
		t.Fatal(err)
	}
	ttyReady(t, p)
	if _, err := p.ReadChar(false); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil || !reflect.DeepEqual(mode, after) {
		t.Fatal("probe changed terminal mode", err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := slave.Stat(); errors.Is(err, os.ErrClosed) {
		t.Fatal("closed borrowed tty", err)
	}
}
func TestTTYRawPartialUTF8AndNonconsumingProbe(t *testing.T) {
	master, slave := testPTY(t)
	state, err := term.MakeRaw(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := term.Restore(int(slave.Fd()), state); err != nil {
			t.Error(err)
		}
	}()
	if _, err := master.Write([]byte{'a'}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		ready, err := fileReady(slave)
		if err != nil {
			t.Fatal(err)
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("raw terminal never ready")
		}
		time.Sleep(time.Millisecond)
	}
	if ready, err := fileReady(slave); !ready || err != nil {
		t.Fatal(ready, err)
	}
	b := make([]byte, 1)
	if n, err := slave.Read(b); n != 1 || b[0] != 'a' || err != nil {
		t.Fatal("probe consumed input", b, n, err)
	}
	p := FileInput(slave, false, false)
	if _, err := master.Write([]byte{0xe7}); err != nil {
		t.Fatal(err)
	}
	if ready, err := p.Ready(); ready || err != nil {
		t.Fatal("partial UTF-8", ready, err)
	}
	if _, err := master.Write([]byte{0x8c, 0xab}); err != nil {
		t.Fatal(err)
	}
	ttyReady(t, p)
	if ch, err := p.ReadChar(false); ch != '猫' || err != nil {
		t.Fatal(ch, err)
	}
}
