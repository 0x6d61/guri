package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Return at most one physical line per transport read. This keeps paste state
// aligned with ReadLine even when a single OS read contains the entire paste.
type terminalSource struct {
	input       *bufio.Reader
	marker      string
	pasteActive bool
	afterCR     bool
}

func (s *terminalSource) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		b, err := s.input.ReadByte()
		if err != nil {
			if n != 0 {
				return n, nil
			}
			return 0, err
		}
		if b == '\n' && s.afterCR {
			s.afterCR = false
			continue
		}
		s.afterCR = b == '\r'
		s.marker += string(b)
		if len(s.marker) > 6 {
			s.marker = s.marker[len(s.marker)-6:]
		}
		switch s.marker {
		case "\x1b[200~":
			s.pasteActive = true
		case "\x1b[201~":
			s.pasteActive = false
		}
		p[n] = b
		n++
		if b == '\n' || b == '\r' || s.input.Buffered() == 0 {
			break
		}
	}
	return n, nil
}

type terminalConnection struct {
	io.Reader
	io.Writer
}

type terminalInput struct {
	terminal *term.Terminal
	source   *terminalSource
	pending  string
	err      error
	enter    func() (func() error, error)
}

// A bracketed paste is one submission, including its embedded newlines. The
// Enter following the paste submits it, so evaluation and prompts cannot break
// up the terminal's echo of the source.
func (in *terminalInput) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if in.pending == "" && in.err == nil {
		leave := func() error { return nil }
		if in.enter != nil {
			var err error
			leave, err = in.enter()
			if err != nil {
				in.err = err
				return 0, err
			}
		}
		in.pending, in.err = in.submission()
		if err := leave(); err != nil {
			in.pending = ""
			in.err = errors.Join(in.err, err)
		}
	}
	if in.pending != "" {
		n := copy(p, in.pending)
		in.pending = in.pending[n:]
		return n, nil
	}
	return 0, in.err
}

func (in *terminalInput) submission() (string, error) {
	var block strings.Builder
	for {
		line, err := in.terminal.ReadLine()
		if line != "" || err == nil || errors.Is(err, term.ErrPasteIndicator) {
			block.WriteString(line)
			block.WriteByte('\n')
		}
		if errors.Is(err, term.ErrPasteIndicator) || err == nil && in.source.pasteActive {
			in.terminal.SetPrompt("")
			continue
		}
		in.terminal.SetPrompt("guri> ")
		return block.String(), err
	}
}

// Only completed, submitted input is readable without waiting for line editing.
func (in *terminalInput) Ready() (bool, error) {
	return in.pending != "" || in.err != nil, nil
}

func terminalStreams(input io.Reader, output io.Writer) (io.Reader, io.Writer) {
	source := &terminalSource{input: bufio.NewReader(input)}
	terminal := term.NewTerminal(terminalConnection{source, output}, "guri> ")
	return &terminalInput{terminal: terminal, source: source}, terminal
}

// Only a terminal REPL owns terminal mode changes. Redirected streams retain
// byte-for-byte output and never receive prompts or terminal escape sequences.
func interactiveStreams(input io.Reader, output, diagnostic io.Writer) (io.Reader, io.Writer, io.Writer) {
	in, inputOK := input.(*os.File)
	out, outputOK := output.(*os.File)
	if !inputOK || !outputOK || !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return input, output, diagnostic
	}
	reader, _ := terminalStreams(input, output)
	editor := reader.(*terminalInput)
	editor.enter = func() (func() error, error) {
		state, err := term.MakeRaw(int(in.Fd()))
		if err != nil {
			return nil, err
		}
		editor.terminal.SetBracketedPasteMode(true)
		return func() error {
			editor.terminal.SetBracketedPasteMode(false)
			return term.Restore(int(in.Fd()), state)
		}, nil
	}
	// Evaluation uses ordinary terminal settings and the original output streams.
	// In particular, Ctrl+C continues to generate SIGINT while Scheme is running.
	return reader, output, diagnostic
}
