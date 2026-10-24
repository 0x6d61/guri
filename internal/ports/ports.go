// Package ports owns stream state behind Scheme port values. It is independent
// of the VM: callers translate errors into Scheme conditions at the boundary.
package ports

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/0x6d61/guri/internal/scheme"
)

var (
	ErrClosed    = errors.New("port is closed")
	ErrDirection = errors.New("incorrect port direction")
	ErrMode      = errors.New("incorrect textual/binary port type")
	ErrEncoding  = errors.New("invalid UTF-8 in textual port")
	ErrMemory    = errors.New("incorrect output memory port type")
	ErrReadiness = errors.New("input adapter needs a nonblocking readiness probe")
)

// Options is an internal adapter contract. Owned is supplied only when Guri
// owns the resource; leaving it nil borrows the resource, even if it is a Closer.
// Ready must never wait for input. It reports bytes or EOF available for Read.
// Memory and file adapters supply it; other stream adapters must supply a probe
// to support ready? without starting background reads on borrowed resources.
type Options struct {
	Owned io.Closer
	Ready func() (bool, error)
}

type Resource struct {
	reader         io.Reader
	writer         io.Writer
	options        Options
	binary, closed bool
	buffer         []byte
	pending        error
	memory         *bytes.Buffer
}

func (*Resource) PortResource() {}
func Input(r io.Reader, binary bool, options Options) *Resource {
	if r == nil {
		panic("ports: nil reader")
	}
	return &Resource{reader: r, binary: binary, options: options}
}
func Output(w io.Writer, binary bool, options Options) *Resource {
	if w == nil {
		panic("ports: nil writer")
	}
	return &Resource{writer: w, binary: binary, options: options}
}
func InputString(s string) *Resource {
	return Input(strings.NewReader(s), false, Options{Ready: func() (bool, error) { return true, nil }})
}
func InputBytes(b []byte) *Resource {
	return Input(bytes.NewReader(bytes.Clone(b)), true, Options{Ready: func() (bool, error) { return true, nil }})
}
func MemoryOutput(binary bool) *Resource {
	b := new(bytes.Buffer)
	p := Output(b, binary, Options{})
	p.memory = b
	return p
}
func (p *Resource) Input() bool      { return p.reader != nil }
func (p *Resource) Output() bool     { return p.writer != nil }
func (p *Resource) Binary() bool     { return p.binary }
func (p *Resource) Open() bool       { return !p.closed }
func Value(p *Resource) *scheme.Port { return scheme.NewPort(p) }
func From(v scheme.Value) (*Resource, error) {
	p, ok := v.(*scheme.Port)
	if !ok {
		return nil, scheme.ErrType
	}
	r, ok := p.Resource().(*Resource)
	if !ok {
		return nil, scheme.ErrType
	}
	return r, nil
}
func (p *Resource) check(input, binary bool) error {
	if p.binary != binary {
		return ErrMode
	}
	if input && !p.Input() || !input && !p.Output() {
		return ErrDirection
	}
	if p.closed {
		return ErrClosed
	}
	return nil
}
func (p *Resource) Close() error {
	if p.closed {
		return nil
	}
	// Mark closed even if flush/close fails; retrying close must not release the
	// same owned resource twice. Always attempt close after a failed flush.
	var err error
	if p.Output() {
		err = p.Flush()
	}
	p.closed = true
	if p.options.Owned != nil {
		err = errors.Join(err, p.options.Owned.Close())
	}
	return err
}
func (p *Resource) Flush() error {
	if !p.Output() {
		return ErrDirection
	}
	if p.closed {
		return ErrClosed
	}
	if f, ok := p.writer.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}
func (p *Resource) fill() error {
	if p.pending != nil {
		return p.pending
	}
	var b [4096]byte
	// A Reader may return (0,nil); bound retries instead of spinning forever.
	for i := 0; i < 100; i++ {
		n, err := p.reader.Read(b[:])
		p.pending = err
		if n > 0 {
			p.buffer = append(p.buffer, b[:n]...)
			return nil
		}
		if err != nil {
			return err
		}
	}
	p.pending = io.ErrNoProgress
	return p.pending
}
func (p *Resource) ReadU8(peek bool) (byte, error) {
	if err := p.check(true, true); err != nil {
		return 0, err
	}
	if len(p.buffer) == 0 {
		if err := p.fill(); err != nil {
			return 0, err
		}
	}
	b := p.buffer[0]
	if !peek {
		p.buffer = p.buffer[1:]
	}
	return b, nil
}
func (p *Resource) ReadChar(peek bool) (rune, error) {
	if err := p.check(true, false); err != nil {
		return 0, err
	}
	for !utf8.FullRune(p.buffer) {
		err := p.fill()
		if err != nil {
			if errors.Is(err, io.EOF) && len(p.buffer) > 0 {
				return 0, ErrEncoding
			}
			return 0, err
		}
	}
	r, n := utf8.DecodeRune(p.buffer)
	if r == utf8.RuneError && n == 1 {
		return 0, ErrEncoding
	}
	if !peek {
		p.buffer = p.buffer[n:]
	}
	return r, nil
}
func (p *Resource) Ready() (bool, error) {
	if p.closed {
		return false, ErrClosed
	}
	if !p.Input() {
		return false, ErrDirection
	}
	for {
		if p.binary && len(p.buffer) > 0 || !p.binary && utf8.FullRune(p.buffer) || p.pending != nil {
			return true, nil
		}
		if p.options.Ready == nil {
			return false, ErrReadiness
		}
		ready, err := p.options.Ready()
		if err != nil || !ready {
			return false, err
		}
		// A readiness probe promises the next Read won't hang, but a complete UTF-8
		// character may span several such reads. Keep the bytes for later peek/read.
		if err = p.fill(); err != nil {
			return true, nil
		}
	}
}
func (p *Resource) write(b []byte) error {
	n, err := p.writer.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return err
}
func (p *Resource) WriteText(s string) error {
	if err := p.check(false, false); err != nil {
		return err
	}
	if !utf8.ValidString(s) {
		return ErrEncoding
	}
	return p.write([]byte(s))
}
func (p *Resource) WriteBytes(b []byte) error {
	if err := p.check(false, true); err != nil {
		return err
	}
	return p.write(b)
}
func (p *Resource) OutputBytes(binary bool) ([]byte, error) {
	if p.memory == nil || p.binary != binary {
		return nil, ErrMemory
	}
	return bytes.Clone(p.memory.Bytes()), nil
}
