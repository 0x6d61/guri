package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/subprocess"
	"github.com/0x6d61/guri/internal/vm"
)

func (r *Runtime) installProcess() {
	exports := libraryExports{}
	for _, name := range []string{"run-process", "run-pipeline"} {
		procedure := vm.Native(name, 0, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if len(a) < 1 || len(a) > 2 {
				return portFailure(c, name, a, ErrorOrdinary, vm.ErrArity)
			}
			commands, options, err := processArguments(c.Context(), name, a)
			if err == nil {
				var result subprocess.Result
				result, err = subprocess.Run(c.Context(), commands, options)
				if err == nil {
					statuses := make([]scheme.Value, len(result.Statuses))
					diagnostic := make([]scheme.Value, len(result.Diagnostics))
					for i, code := range result.Statuses {
						statuses[i] = scheme.False
						if code >= 0 {
							statuses[i] = scheme.NewInteger(code)
						}
						diagnostic[i] = scheme.NewBytevector(result.Diagnostics[i])
					}
					output := scheme.NewBytevector(result.Output)
					if name == "run-process" {
						return vm.Return(statuses[0], output, diagnostic[0]), nil
					}
					return vm.Return(scheme.List(statuses...), output, scheme.List(diagnostic...)), nil
				}
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return vm.Action{}, err
			}
			return portFailure(c, name, a, ErrorOrdinary, err)
		})
		exports[name] = &binding{id: scheme.NewIdentifier(r.bootstrap.symbols.Intern(name)), form: variable, declared: true, cell: scheme.NewCell(procedure)}
	}
	ns, err := reader.ReadAll("<extension-libraries>", "(guri process)", r.bootstrap.symbols)
	if err != nil {
		panic(err)
	}
	key, _, err := parseLibraryName(ns[0])
	if err != nil {
		panic(err)
	}
	r.modules[key] = &library{exports: exports, prepared: true, initialized: true}
}

// Process arguments are copied before spawning any child. Mutable Scheme values
// never cross into I/O goroutines, and invalid later stages have no side effects.
func processArguments(ctx context.Context, name string, a []scheme.Value) ([][]string, subprocess.Options, error) {
	var options subprocess.Options
	values := []scheme.Value{a[0]}
	if name == "run-pipeline" {
		var err error
		values, err = processList(ctx, a[0])
		if err != nil {
			return nil, options, err
		}
	}
	if len(values) == 0 {
		return nil, options, errors.New("commands must not be empty")
	}
	commands := make([][]string, len(values))
	for i, value := range values {
		args, err := processList(ctx, value)
		if err != nil {
			return nil, options, err
		}
		if len(args) == 0 {
			return nil, options, errors.New("argv must not be empty")
		}
		for j, arg := range args {
			text, err := processString(arg)
			if err != nil {
				return nil, options, err
			}
			if j == 0 && text == "" {
				return nil, options, errors.New("empty executable")
			}
			commands[i] = append(commands[i], text)
		}
	}
	if len(a) == 1 {
		return commands, options, nil
	}
	entries, err := processList(ctx, a[1])
	if err != nil {
		return nil, options, err
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		pair, ok := entry.(*scheme.Pair)
		if !ok {
			return nil, options, scheme.ErrType
		}
		key, ok := pair.Car().(*scheme.Symbol)
		if !ok {
			return nil, options, scheme.ErrType
		}
		keyName := key.Name()
		if seen[keyName] {
			return nil, options, fmt.Errorf("duplicate option: %s", keyName)
		}
		seen[keyName] = true
		switch keyName {
		case "input":
			b, ok := pair.Cdr().(*scheme.Bytevector)
			if !ok {
				return nil, options, scheme.ErrType
			}
			options.Input = b.Bytes()
		case "directory":
			options.Directory, err = processString(pair.Cdr())
		case "environment":
			options.Environment, err = processEnvironment(ctx, pair.Cdr())
		default:
			err = fmt.Errorf("unknown option: %s", keyName)
		}
		if err != nil {
			return nil, options, err
		}
	}
	return commands, options, nil
}
func processString(v scheme.Value) (string, error) {
	s, ok := v.(*scheme.String)
	if !ok {
		return "", scheme.ErrType
	}
	text := s.Text()
	if strings.IndexByte(text, 0) >= 0 {
		return "", errors.New("NUL in process argument")
	}
	return text, nil
}
func processList(ctx context.Context, v scheme.Value) ([]scheme.Value, error) {
	var values []scheme.Value
	seen := map[*scheme.Pair]bool{}
	for v != scheme.Null {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p, ok := v.(*scheme.Pair)
		if !ok {
			return nil, scheme.ErrType
		}
		if seen[p] {
			return nil, errors.New("cyclic process argument")
		}
		seen[p] = true
		values = append(values, p.Car())
		v = p.Cdr()
	}
	return values, nil
}
func processEnvironment(ctx context.Context, v scheme.Value) ([]string, error) {
	entries, err := processList(ctx, v)
	if err != nil {
		return nil, err
	}
	overrides := map[string]string{}
	normalize := func(s string) string {
		if runtime.GOOS == "windows" {
			return strings.ToUpper(s)
		}
		return s
	}
	for _, entry := range entries {
		p, ok := entry.(*scheme.Pair)
		if !ok {
			return nil, scheme.ErrType
		}
		key, err := processString(p.Car())
		if err != nil {
			return nil, err
		}
		if key == "" || strings.Contains(key, "=") {
			return nil, errors.New("invalid environment variable name")
		}
		value, err := processString(p.Cdr())
		if err != nil {
			return nil, err
		}
		normalized := normalize(key)
		if _, ok := overrides[normalized]; ok {
			return nil, fmt.Errorf("duplicate environment variable: %s", key)
		}
		overrides[normalized] = key + "=" + value
	}
	env := os.Environ()
	for i, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		normalized := normalize(key)
		if value, ok := overrides[normalized]; ok {
			env[i] = value
			delete(overrides, normalized)
		}
	}
	for _, entry := range overrides {
		env = append(env, entry)
	}
	return env, nil
}
