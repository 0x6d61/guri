package core

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

var portArities = map[string]pairArity{
	"port?": {1, 1}, "input-port?": {1, 1}, "output-port?": {1, 1}, "textual-port?": {1, 1}, "binary-port?": {1, 1},
	"input-port-open?": {1, 1}, "output-port-open?": {1, 1},
	"close-port": {1, 1}, "close-input-port": {1, 1}, "close-output-port": {1, 1}, "call-with-port": {2, 2},
	"open-input-string": {1, 1}, "open-output-string": {0, 0}, "get-output-string": {1, 1},
	"open-input-bytevector": {1, 1}, "open-output-bytevector": {0, 0}, "get-output-bytevector": {1, 1},
	"eof-object": {0, 0}, "eof-object?": {1, 1},
	"read-char": {0, 1}, "peek-char": {0, 1}, "char-ready?": {0, 1}, "read-line": {0, 1}, "read-string": {1, 2},
	"read-u8": {0, 1}, "peek-u8": {0, 1}, "u8-ready?": {0, 1}, "read-bytevector": {1, 2}, "read-bytevector!": {1, 4},
	"write-char": {1, 2}, "write-string": {1, 4}, "write-u8": {1, 2}, "write-bytevector": {1, 4}, "newline": {0, 1}, "flush-output-port": {0, 1},
}

func portFailure(c *vm.Context, name string, a []scheme.Value, kind ErrorKind, err error) (vm.Action, error) {
	object, e := NewErrorObject(kind, fmt.Sprintf("%s: %v", name, err), a...)
	if e != nil {
		return vm.Action{}, e
	}
	return Raise(c, object, false)
}

func (e *Engine) installPorts() {
	inputs := ports.Value(ports.FileInput(os.Stdin, false, false))
	outputs := ports.Value(ports.Output(os.Stdout, false, ports.Options{}))
	stderr := ports.Value(ports.Output(os.Stderr, false, ports.Options{}))
	current := map[string]*parameter{}
	for _, item := range []struct {
		name    string
		initial scheme.Value
		input   bool
	}{
		{"current-input-port", inputs, true}, {"current-output-port", outputs, false}, {"current-error-port", stderr, false},
	} {
		converter := vm.Native(item.name+"-converter", 1, 1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			p, err := ports.From(a[0])
			if err == nil && (item.input && !p.Input() || !item.input && !p.Output()) {
				err = ports.ErrDirection
			}
			if err != nil {
				return portFailure(c, item.name, a, ErrorOrdinary, err)
			}
			return vm.Return(a[0]), nil
		})
		p := &parameter{initial: item.initial, converter: converter}
		current[item.name] = p
		e.Bind(item.name, scheme.NewProcedure(p))
	}
	e.installDatumIO(current)
	for name, arity := range portArities {
		e.Bind(name, vm.Native(name, arity.min, arity.max, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if name == "call-with-port" {
				p, err := ports.From(a[0])
				if err != nil {
					return portFailure(c, name, a, ErrorOrdinary, err)
				}
				if _, ok := a[1].(*scheme.Procedure); !ok {
					return portFailure(c, name, a, ErrorOrdinary, scheme.ErrType)
				}
				return vm.Invoke(a[1], []scheme.Value{a[0]}, func(ctx *vm.Context, r scheme.Results) (vm.Action, error) {
					if err := p.Close(); err != nil {
						return portFailure(ctx, name, a, ErrorOrdinary, err)
					}
					return vm.Return(r.Values()...), nil
				}), nil
			}
			value, err := portPrimitive(c, current, name, a)
			if err != nil {
				return portFailure(c, name, a, ErrorOrdinary, err)
			}
			return vm.Return(value), nil
		}))
	}
}

func selectedPort(c *vm.Context, params map[string]*parameter, a []scheme.Value, index int, input bool) (*ports.Resource, error) {
	var v scheme.Value
	if len(a) > index {
		v = a[index]
	} else {
		name := "current-output-port"
		if input {
			name = "current-input-port"
		}
		v = params[name].value(c)
	}
	return ports.From(v)
}
func portPrimitive(c *vm.Context, params map[string]*parameter, name string, a []scheme.Value) (scheme.Value, error) {
	switch name {
	case "port?":
		_, ok := a[0].(*scheme.Port)
		return scheme.Boolean(ok), nil
	case "input-port?", "output-port?", "textual-port?", "binary-port?":
		p, err := ports.From(a[0])
		if err != nil {
			return scheme.False, nil
		}
		answer := false
		switch name {
		case "input-port?":
			answer = p.Input()
		case "output-port?":
			answer = p.Output()
		case "textual-port?":
			answer = !p.Binary()
		case "binary-port?":
			answer = p.Binary()
		}
		return scheme.Boolean(answer), nil
	case "input-port-open?", "output-port-open?":
		p, err := ports.From(a[0])
		if err != nil {
			return nil, err
		}
		return scheme.Boolean(p.Open() && (name == "input-port-open?" && p.Input() || name == "output-port-open?" && p.Output())), nil
	case "close-port", "close-input-port", "close-output-port":
		p, err := ports.From(a[0])
		if err != nil {
			return nil, err
		}
		if name == "close-input-port" && !p.Input() || name == "close-output-port" && !p.Output() {
			return nil, ports.ErrDirection
		}
		return scheme.Unspecified, p.Close()
	case "open-input-string":
		s, ok := a[0].(*scheme.String)
		if !ok {
			return nil, scheme.ErrType
		}
		return ports.Value(ports.InputString(s.Text())), nil
	case "open-input-bytevector":
		b, err := bytevectorValue(a[0])
		if err != nil {
			return nil, err
		}
		return ports.Value(ports.InputBytes(b.Bytes())), nil
	case "open-output-string", "open-output-bytevector":
		return ports.Value(ports.MemoryOutput(name == "open-output-bytevector")), nil
	case "get-output-string", "get-output-bytevector":
		p, err := ports.From(a[0])
		if err != nil {
			return nil, err
		}
		b, err := p.OutputBytes(name == "get-output-bytevector")
		if err != nil {
			return nil, err
		}
		if name == "get-output-string" {
			return scheme.NewString(string(b))
		}
		return scheme.NewBytevector(b), nil
	case "eof-object":
		return scheme.EOF, nil
	case "eof-object?":
		return scheme.Boolean(a[0] == scheme.EOF), nil
	}
	input := strings.HasPrefix(name, "read-") || strings.HasPrefix(name, "peek-") || name == "char-ready?" || name == "u8-ready?"
	index := 0
	switch name {
	case "read-string", "read-bytevector", "read-bytevector!", "write-char", "write-string", "write-u8", "write-bytevector":
		index = 1
	}
	p, err := selectedPort(c, params, a, index, input)
	if err != nil {
		return nil, err
	}
	// Validate the type and direction before zero-length operations as well.
	binary := name == "read-u8" || name == "peek-u8" || name == "u8-ready?" || name == "read-bytevector" || name == "read-bytevector!" || name == "write-u8" || name == "write-bytevector"
	if name != "flush-output-port" && p.Binary() != binary {
		return nil, ports.ErrMode
	}
	if input && !p.Input() || !input && !p.Output() {
		return nil, ports.ErrDirection
	}
	if !p.Open() {
		return nil, ports.ErrClosed
	}
	switch name {
	case "read-char", "peek-char":
		r, err := p.ReadChar(name == "peek-char")
		if errors.Is(err, io.EOF) {
			return scheme.EOF, nil
		}
		if err != nil {
			return nil, err
		}
		return scheme.NewCharacter(r)
	case "read-u8", "peek-u8":
		b, err := p.ReadU8(name == "peek-u8")
		if errors.Is(err, io.EOF) {
			return scheme.EOF, nil
		}
		if err != nil {
			return nil, err
		}
		return scheme.NewInteger(int64(b)), nil
	case "char-ready?", "u8-ready?":
		ready, err := p.Ready()
		return scheme.Boolean(ready), err
	case "read-line":
		return readPortLine(p)
	case "read-string", "read-bytevector":
		count, err := nonnegativeIndex(a[0])
		if err != nil {
			return nil, err
		}
		return readPortCount(p, count, binary)
	case "read-bytevector!":
		b, err := bytevectorValue(a[0])
		if err != nil {
			return nil, err
		}
		var bounds []scheme.Value
		if len(a) > 2 {
			bounds = a[2:]
		}
		start, end, err := vectorRange(b.Len(), bounds)
		if err != nil {
			return nil, err
		}
		// Validate constant destinations before consuming any input, even for empty
		// ranges. This also avoids partially consuming a stream before an error.
		if err = b.CopyFrom(start, scheme.NewBytevector(nil), 0, 0); err != nil {
			return nil, err
		}
		n := 0
		for i := start; i < end; i++ {
			x, err := p.ReadU8(false)
			if errors.Is(err, io.EOF) {
				if n == 0 {
					return scheme.EOF, nil
				}
				break
			}
			if err != nil {
				return nil, err
			}
			if err = b.Set(i, int64(x)); err != nil {
				return nil, err
			}
			n++
		}
		return scheme.NewInteger(int64(n)), nil
	case "write-char":
		x, ok := a[0].(scheme.Character)
		if !ok {
			return nil, scheme.ErrType
		}
		return scheme.Unspecified, p.WriteText(string(x.Rune()))
	case "write-u8":
		x, err := byteValue(a[0])
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, p.WriteBytes([]byte{x})
	case "write-string":
		s, ok := a[0].(*scheme.String)
		if !ok {
			return nil, scheme.ErrType
		}
		var bounds []scheme.Value
		if len(a) > 2 {
			bounds = a[2:]
		}
		start, end, err := vectorRange(s.Len(), bounds)
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, p.WriteText(string([]rune(s.Text())[start:end]))
	case "write-bytevector":
		b, err := bytevectorValue(a[0])
		if err != nil {
			return nil, err
		}
		var bounds []scheme.Value
		if len(a) > 2 {
			bounds = a[2:]
		}
		start, end, err := vectorRange(b.Len(), bounds)
		if err != nil {
			return nil, err
		}
		return scheme.Unspecified, p.WriteBytes(b.Bytes()[start:end])
	case "newline":
		return scheme.Unspecified, p.WriteText("\n")
	case "flush-output-port":
		return scheme.Unspecified, p.Flush()
	}
	return nil, scheme.ErrType
}
func readPortLine(p *ports.Resource) (scheme.Value, error) {
	var text strings.Builder
	for {
		r, err := p.ReadChar(false)
		if errors.Is(err, io.EOF) {
			if text.Len() == 0 {
				return scheme.EOF, nil
			}
			break
		}
		if err != nil {
			return nil, err
		}
		if r == '\n' {
			break
		}
		if r == '\r' {
			next, err := p.ReadChar(true)
			if err != nil && !errors.Is(err, io.EOF) {
				return nil, err
			}
			if err == nil && next == '\n' {
				if _, err = p.ReadChar(false); err != nil {
					return nil, err
				}
			}
			break
		}
		text.WriteRune(r)
	}
	return scheme.NewString(text.String())
}
func readPortCount(p *ports.Resource, count *big.Int, binary bool) (scheme.Value, error) {
	var bytes []byte
	var text strings.Builder
	one := big.NewInt(1)
	for count.Sign() > 0 {
		var err error
		if binary {
			var b byte
			b, err = p.ReadU8(false)
			if err == nil {
				bytes = append(bytes, b)
			}
		} else {
			var r rune
			r, err = p.ReadChar(false)
			if err == nil {
				text.WriteRune(r)
			}
		}
		if errors.Is(err, io.EOF) {
			if len(bytes) == 0 && text.Len() == 0 {
				return scheme.EOF, nil
			}
			break
		}
		if err != nil {
			return nil, err
		}
		count.Sub(count, one)
	}
	if binary {
		return scheme.NewBytevector(bytes), nil
	}
	return scheme.NewString(text.String())
}
