package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
	"github.com/0x6d61/guri/internal/writer"
)

// REPL is an internal driver over the persistent interaction environment. Its
// streams are borrowed; the public CLI/API migration is a separate change.
// Prompts belong to the terminal frontend, not this datum/evaluation loop.
func (r *Runtime) REPL(ctx context.Context, filename string, input io.Reader, output, diagnostic io.Writer) (err error) {
	if r.running {
		return vm.ErrReentrant
	}
	r.running = true
	r.context = ctx
	defer func() { r.running = false; r.context = nil }()
	env := r.interactionEnvironment()
	in := ports.Input(input, false, ports.Options{Filename: filename})
	if file, ok := input.(*os.File); ok {
		in = ports.FileInput(file, false, false)
	}
	out := ports.Output(output, false, ports.Options{})
	errorPort := ports.Output(diagnostic, false, ports.Options{})
	bound := parameterBindings{}
	for name, v := range map[string]scheme.Value{"current-input-port": ports.Value(in), "current-output-port": ports.Value(out), "current-error-port": ports.Value(errorPort)} {
		b := r.bootstrap.find(nil, r.bootstrap.primitives, scheme.NewIdentifier(r.bootstrap.symbols.Intern(name)))
		value, e := b.cell.Get()
		if e != nil {
			return e
		}
		bound[value.(*scheme.Procedure).Code().(*parameter)] = v
	}
	dynamic := vm.NewDynamic(nil, bound)
	rd := reader.NewStream(in, r.bootstrap.symbols)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := rd.Read()
		if errors.Is(err, io.EOF) {
			return out.Flush()
		}
		if err != nil {
			if _, writeErr := fmt.Fprintln(diagnostic, err); writeErr != nil {
				return writeErr
			}
			// A malformed token may leave the unexpected character unconsumed. Move
			// past it before retrying, while preserving following independent datums.
			var failure *reader.Error
			if errors.As(err, &failure) && in.Position() == failure.Position {
				if _, _, e := in.ReadRune(); e != nil {
					if errors.Is(e, io.EOF) {
						return nil
					}
					return e
				}
			}
			continue
		}
		entry := vm.Native("repl-input", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) {
			if rawForm(n, "import") {
				deps, err := r.interactionImports(c, env.engine, n)
				if err != nil {
					return vm.Action{}, err
				}
				done := vm.Native("import-result", 0, 0, func(_ *vm.Context, _ []scheme.Value) (vm.Action, error) { return vm.Return(scheme.Unspecified), nil })
				return initializeLibraries(c, dependencyOrder(deps), 0, done)
			}
			code, err := env.engine.compileEvaluation(c.Context(), []*reader.Node{n})
			if err != nil {
				return vm.Action{}, err
			}
			return vm.Invoke(vm.Closure(code), nil, nil), nil
		})
		wrapped := vm.Native("repl-ports", 0, 0, func(_ *vm.Context, _ []scheme.Value) (vm.Action, error) {
			return vm.Invoke(entry, nil, nil).WithDynamic(dynamic), nil
		})
		result, err := r.bootstrap.machine.Run(ctx, wrapped)
		r.finishInitializers(err)
		if err != nil {
			var exit *ExitRequest
			if errors.As(err, &exit) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			if _, writeErr := fmt.Fprintln(diagnostic, err); writeErr != nil {
				return writeErr
			}
			continue
		}
		for _, v := range result.Values() {
			if v == scheme.Unspecified {
				continue
			}
			if err := writer.Write(ctx, textPortWriter{out}, v, writer.Normal); err != nil {
				return err
			}
			if err := out.WriteText("\n"); err != nil {
				return err
			}
		}
		if err := out.Flush(); err != nil {
			return err
		}
	}
}
func (r *Runtime) finishInitializers(err error) {
	for _, l := range r.modules {
		if l.initializing {
			l.initializing = false
			l.err = err
			if l.err == nil {
				l.err = fmt.Errorf("library initializer escaped before returning")
			}
		}
	}
}
