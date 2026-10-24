package core

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
	"github.com/0x6d61/guri/internal/writer"
)

var datumIOArities = map[string]pairArity{
	"read": {0, 1}, "write": {1, 2}, "write-shared": {1, 2}, "write-simple": {1, 2}, "display": {1, 2},
	"open-input-file": {1, 1}, "open-output-file": {1, 1}, "open-binary-input-file": {1, 1}, "open-binary-output-file": {1, 1},
	"call-with-input-file": {2, 2}, "call-with-output-file": {2, 2}, "with-input-from-file": {2, 2}, "with-output-to-file": {2, 2},
}

var datumWriteModes = map[string]writer.Mode{"write": writer.Normal, "write-shared": writer.Shared, "write-simple": writer.Simple, "display": writer.Display}

func (e *Engine) installDatumIO(current map[string]*parameter) {
	for name, arity := range datumIOArities {
		e.Bind(name, vm.Native(name, arity.min, arity.max, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if name == "read" {
				p, err := selectedPort(c, current, a, 0, true)
				if err == nil {
					err = checkTextPort(p, true)
				}
				if err != nil {
					return portFailure(c, name, a, ErrorOrdinary, err)
				}
				r := reader.NewStream(p, e.symbols)
				r.SetFoldCase(p.FoldCase())
				n, err := r.Read()
				p.SetFoldCase(r.FoldCase())
				if errors.Is(err, io.EOF) {
					return vm.Return(scheme.EOF), nil
				}
				if err != nil {
					return portFailure(c, name, a, ErrorRead, err)
				}
				return vm.Return(n.Datum()), nil
			}
			mode, writing := datumWriteModes[name]
			if writing {
				p, err := selectedPort(c, current, a, 1, false)
				if err == nil {
					err = checkTextPort(p, false)
				}
				if err == nil {
					err = writer.Write(c.Context(), textPortWriter{p}, a[0], mode)
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return vm.Action{}, err
				}
				if err != nil {
					return portFailure(c, name, a, ErrorOrdinary, err)
				}
				return vm.Return(scheme.Unspecified), nil
			}
			if strings.HasPrefix(name, "open-") {
				p, kind, err := openSchemeFile(name, a[0])
				if err != nil {
					return portFailure(c, name, a, kind, err)
				}
				return vm.Return(p), nil
			}
			if _, ok := a[1].(*scheme.Procedure); !ok {
				return portFailure(c, name, a, ErrorOrdinary, scheme.ErrType)
			}
			input := name == "call-with-input-file" || name == "with-input-from-file"
			openName := "open-output-file"
			if input {
				openName = "open-input-file"
			}
			p, kind, err := openSchemeFile(openName, a[0])
			if err != nil {
				return portFailure(c, name, a, kind, err)
			}
			resource, err := ports.From(p)
			if err != nil {
				return vm.Action{}, err
			}
			withDefault := strings.HasPrefix(name, "with-")
			old := c.Dynamic()
			dynamic := old
			args := []scheme.Value{p}
			if withDefault {
				parameterName := "current-output-port"
				if input {
					parameterName = "current-input-port"
				}
				bound := parameterBindings{current[parameterName]: p}
				dynamic = vm.NewDynamic(old, bound)
				args = nil
			}
			// The immutable resume closes the same port on normal return, including
			// after multi-shot continuation reentry. Escape never runs this callback.
			action := vm.Invoke(a[1], args, func(ctx *vm.Context, r scheme.Results) (vm.Action, error) {
				if err := resource.Close(); err != nil {
					return portFailure(ctx, name, a, ErrorOrdinary, err)
				}
				result := vm.Return(r.Values()...)
				if withDefault {
					result = result.WithDynamic(old)
				}
				return result, nil
			})
			if withDefault {
				action = action.WithDynamic(dynamic)
			}
			return action, nil
		}))
	}
}
func openSchemeFile(name string, path scheme.Value) (*scheme.Port, ErrorKind, error) {
	s, ok := path.(*scheme.String)
	if !ok {
		return nil, ErrorOrdinary, scheme.ErrType
	}
	binary := strings.Contains(name, "binary")
	if strings.HasSuffix(name, "input-file") {
		f, err := os.Open(s.Text())
		if err != nil {
			return nil, ErrorFile, err
		}
		return ports.Value(ports.FileInput(f, binary, true)), ErrorOrdinary, nil
	}
	f, err := os.OpenFile(s.Text(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, ErrorFile, err
	}
	return ports.Value(ports.Output(f, binary, ports.Options{Owned: f, Filename: s.Text()})), ErrorOrdinary, nil
}
func checkTextPort(p *ports.Resource, input bool) error {
	if p.Binary() {
		return ports.ErrMode
	}
	if input && !p.Input() || !input && !p.Output() {
		return ports.ErrDirection
	}
	if !p.Open() {
		return ports.ErrClosed
	}
	return nil
}

type textPortWriter struct{ port *ports.Resource }

func (w textPortWriter) Write(b []byte) (int, error) {
	if err := w.port.WriteText(string(b)); err != nil {
		return 0, err
	}
	return len(b), nil
}
