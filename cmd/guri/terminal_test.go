package main

import (
	"bytes"
	"github.com/0x6d61/guri/internal/ports"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

const pastedFindFirst = `(import (scheme base)
        (scheme write))

(define (find-first pred items)
  (call/cc
    (lambda (return)
      (for-each
        (lambda (item)
          (if (pred item)
              (return item)))
        items)
      #f)))

(display (find-first even? '(1 3 5 8 9)))
(newline)
`

func TestTerminalPaste(t *testing.T) {
	var screen bytes.Buffer
	input, output := terminalStreams(strings.NewReader("\x1b[200~"+pastedFindFirst+"\x1b[201~\r\x04"), &screen)
	var diagnostics bytes.Buffer
	if code := run(nil, input, output, &diagnostics); code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("exit=%d diagnostics=%s", code, &diagnostics)
	}
	transcript := screen.String()
	if strings.Count(transcript, "guri> ") != 2 {
		t.Fatalf("prompts inserted into paste: %q", transcript)
	}
	if !strings.Contains(transcript, "8\r\nguri> ") {
		t.Fatalf("missing result before next prompt: %q", transcript)
	}
}

func TestTerminalPasteIsOneInputBlock(t *testing.T) {
	var screen bytes.Buffer
	input, _ := terminalStreams(strings.NewReader("\x1b[200~(define x 1)\n(+ x 2)\x1b[201~\r\x04"), &screen)
	data := make([]byte, 4096)
	n, err := input.Read(data)
	if err != nil || string(data[:n]) != "(define x 1)\n(+ x 2)\n" {
		t.Fatalf("paste=%q error=%v", data[:n], err)
	}
	if _, err := input.Read(data); err != io.EOF {
		t.Fatalf("EOF=%v", err)
	}
}

func TestTerminalFragmentedPaste(t *testing.T) {
	var screen bytes.Buffer
	input, _ := terminalStreams(iotest.OneByteReader(strings.NewReader("\x1b[200~日本\n語\x1b[201~\r\x04")), &screen)
	data, err := io.ReadAll(input)
	if err != nil || string(data) != "日本\n語\n" {
		t.Fatalf("input=%q error=%v", data, err)
	}
	if strings.Count(screen.String(), "guri> ") != 2 {
		t.Fatalf("screen=%q", screen.String())
	}
}

func TestTerminalInputReadiness(t *testing.T) {
	var screen bytes.Buffer
	input, _ := terminalStreams(strings.NewReader("42\r\x04"), &screen)
	p := ports.Input(input, false, ports.Options{})
	if ready, err := p.Ready(); ready || err != nil {
		t.Fatalf("before line submission: %t %v", ready, err)
	}
	if _, _, err := p.ReadRune(); err != nil {
		t.Fatal(err)
	}
	if ready, err := p.Ready(); !ready || err != nil {
		t.Fatalf("buffered submitted input: %t %v", ready, err)
	}
}

func TestTerminalLargePaste(t *testing.T) {
	source := "(display \"" + strings.Repeat("あ", 5000) + "\")\n(newline)"
	var screen bytes.Buffer
	input, _ := terminalStreams(strings.NewReader("\x1b[200~"+source+"\x1b[201~\r\x04"), &screen)
	data, err := io.ReadAll(input)
	if err != nil || string(data) != source+"\n" {
		t.Fatalf("paste truncated: bytes=%d expected=%d error=%v", len(data), len(source)+1, err)
	}
}

func TestTerminalPasteAfterTypedPrefix(t *testing.T) {
	var screen bytes.Buffer
	input, _ := terminalStreams(strings.NewReader("(list \x1b[200~1\n2)\n\x1b[201~\r\x04"), &screen)
	p := make([]byte, 4096)
	n, err := input.Read(p)
	if err != nil || string(p[:n]) != "(list 1\n2)\n\n" {
		t.Fatalf("split paste=%q error=%v", p[:n], err)
	}
	if strings.Count(screen.String(), "guri> ") != 1 {
		t.Fatalf("prompt in paste: %q", screen.String())
	}
}
