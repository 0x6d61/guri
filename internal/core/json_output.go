package core

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/0x6d61/guri/internal/number"
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

type jsonOutput struct {
	port        *ports.Resource
	accumulator *scheme.Procedure
}

func newJSONOutput(v scheme.Value) (*jsonOutput, error) {
	if p, ok := v.(*scheme.Procedure); ok {
		return &jsonOutput{accumulator: p}, nil
	}
	p, err := ports.From(v)
	if err != nil {
		return nil, err
	}
	if err = checkTextPort(p, false); err != nil {
		return nil, err
	}
	return &jsonOutput{port: p}, nil
}
func (out *jsonOutput) emit(c *vm.Context, text string) (vm.Action, error) {
	if err := c.Context().Err(); err != nil {
		return vm.Action{}, err
	}
	if out.port != nil {
		if err := out.port.WriteText(text); err != nil {
			return jsonFailure(c, err)
		}
		return vm.Return(scheme.Unspecified), nil
	}
	v, err := scheme.NewString(text)
	if err != nil {
		return jsonFailure(c, err)
	}
	return vm.Invoke(out.accumulator, []scheme.Value{v}, func(_ *vm.Context, _ scheme.Results) (vm.Action, error) { return vm.Return(scheme.Unspecified), nil }), nil
}
func (j *jsonLibrary) write(c *vm.Context, name string, a []scheme.Value) (vm.Action, error) {
	target := j.output.value(c)
	if name == "json-accumulator" {
		target = a[0]
	} else if len(a) == 2 {
		target = a[1]
	}
	out, err := newJSONOutput(target)
	if err != nil {
		return jsonFailure(c, err)
	}
	if name == "json-accumulator" {
		state := jsonWriteState{}
		return vm.Return(vm.Native("JSON event accumulator", 0, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if len(a) != 1 {
				return jsonFailure(c, vm.ErrArity)
			}
			text, next, err := state.event(a[0])
			if err != nil {
				return jsonFailure(c, err)
			}
			state = next
			return out.emit(c, text)
		})), nil
	}
	chunks, err := jsonEncode(c.Context(), a[0])
	if err != nil {
		if c.Context().Err() != nil {
			return vm.Action{}, c.Context().Err()
		}
		return jsonFailure(c, err)
	}
	// The immutable serialization snapshot prevents an output callback from
	// changing an unvisited value after validation, or replaying a mutable cursor.
	return jsonWriteChunks(c, out, chunks, 0)
}
func jsonWriteChunks(c *vm.Context, out *jsonOutput, chunks []string, index int) (vm.Action, error) {
	if out.port != nil {
		for _, text := range chunks[index:] {
			if err := c.Context().Err(); err != nil {
				return vm.Action{}, err
			}
			if err := out.port.WriteText(text); err != nil {
				return jsonFailure(c, err)
			}
		}
		return vm.Return(scheme.Unspecified), nil
	}
	if index == len(chunks) {
		return vm.Return(scheme.Unspecified), nil
	}
	v, err := scheme.NewString(chunks[index])
	if err != nil {
		return jsonFailure(c, err)
	}
	return vm.Invoke(out.accumulator, []scheme.Value{v}, func(c *vm.Context, _ scheme.Results) (vm.Action, error) {
		return jsonWriteChunks(c, out, chunks, index+1)
	}), nil
}
func jsonScalar(v scheme.Value) (string, error) {
	switch x := v.(type) {
	case scheme.Boolean:
		if x {
			return "true", nil
		}
		return "false", nil
	case *scheme.Symbol:
		if x.Name() == "null" {
			return "null", nil
		}
	case *scheme.String:
		encoded, err := json.Marshal(x.Text())
		return string(encoded), err
	case *scheme.Integer:
		return x.String(), nil
	case scheme.Float:
		if !math.IsNaN(x.Float64()) && !math.IsInf(x.Float64(), 0) {
			return number.Format(x, 10)
		}
	}
	return "", errors.New("value cannot be serialized as JSON")
}

type jsonWriteFrame struct {
	object, hasValue, key bool
	parent                *jsonWriteFrame
}
type jsonWriteState struct {
	frame   *jsonWriteFrame
	started bool
}

func (s jsonWriteState) event(v scheme.Value) (string, jsonWriteState, error) {
	fail := func() (string, jsonWriteState, error) { return "", s, errors.New("invalid JSON event sequence") }
	symbol, _ := v.(*scheme.Symbol)
	name := ""
	if symbol != nil {
		name = symbol.Name()
	}
	if name == "array-end" || name == "object-end" {
		if s.frame == nil || s.frame.object != (name == "object-end") || s.frame.object && !s.frame.key {
			return fail()
		}
		s.frame = s.frame.parent
		if name == "array-end" {
			return "]", s, nil
		}
		return "}", s, nil
	}
	prefix := ""
	if s.frame == nil {
		if s.started {
			return fail()
		}
		s.started = true
	} else {
		f := *s.frame
		if f.object && f.key {
			key, ok := v.(*scheme.String)
			if !ok {
				return fail()
			}
			text, err := jsonScalar(key)
			if err != nil {
				return "", s, err
			}
			if f.hasValue {
				prefix = ","
			}
			f.key = false
			s.frame = &f
			return prefix + text + ":", s, nil
		}
		if !f.object && f.hasValue {
			prefix = ","
		}
		f.hasValue = true
		f.key = f.object
		s.frame = &f
	}
	if name == "array-start" || name == "object-start" {
		object := name == "object-start"
		s.frame = &jsonWriteFrame{object: object, key: object, parent: s.frame}
		if object {
			return prefix + "{", s, nil
		}
		return prefix + "[", s, nil
	}
	text, err := jsonScalar(v)
	if err != nil {
		return "", s, err
	}
	return prefix + text, s, nil
}

// Check all pair/vector edges before serializing. DFS distinguishes shared
// subgraphs from cycles, including cycles through an association list's spine.
func jsonAcyclic(ctx context.Context, value scheme.Value) error {
	type task struct {
		value scheme.Value
		exit  bool
	}
	pending := []task{{value: value}}
	state := map[scheme.Value]byte{}
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		item := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if item.exit {
			state[item.value] = 2
			continue
		}
		switch item.value.(type) {
		case *scheme.Pair, *scheme.Vector:
		default:
			continue
		}
		if state[item.value] == 1 {
			return errors.New("cyclic value cannot be serialized as JSON")
		}
		if state[item.value] == 2 {
			continue
		}
		state[item.value] = 1
		pending = append(pending, task{value: item.value, exit: true})
		switch x := item.value.(type) {
		case *scheme.Pair:
			pending = append(pending, task{value: x.Cdr()}, task{value: x.Car()})
		case *scheme.Vector:
			for i := x.Len() - 1; i >= 0; i-- {
				v, err := x.Ref(i)
				if err != nil {
					return err
				}
				pending = append(pending, task{value: v})
			}
		}
	}
	return nil
}
func jsonEncode(ctx context.Context, value scheme.Value) ([]string, error) {
	if err := jsonAcyclic(ctx, value); err != nil {
		return nil, err
	}
	type task struct {
		value scheme.Value
		text  string
	}
	pending := []task{{value: value}}
	var chunks []string
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if item.value == nil {
			chunks = append(chunks, item.text)
			continue
		}
		switch x := item.value.(type) {
		case *scheme.Vector:
			chunks = append(chunks, "[")
			pending = append(pending, task{text: "]"})
			for i := x.Len() - 1; i >= 0; i-- {
				v, err := x.Ref(i)
				if err != nil {
					return nil, err
				}
				pending = append(pending, task{value: v})
				if i > 0 {
					pending = append(pending, task{text: ","})
				}
			}
		case *scheme.Pair:
			entries, err := processList(ctx, x)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, "{")
			pending = append(pending, task{text: "}"})
			for i := len(entries) - 1; i >= 0; i-- {
				pair, ok := entries[i].(*scheme.Pair)
				if !ok {
					return nil, errors.New("JSON object needs association pairs")
				}
				key, ok := pair.Car().(*scheme.Symbol)
				if !ok {
					return nil, errors.New("JSON object keys must be symbols")
				}
				encoded, err := json.Marshal(key.Name())
				if err != nil {
					return nil, err
				}
				pending = append(pending, task{value: pair.Cdr()}, task{text: string(encoded) + ":"})
				if i > 0 {
					pending = append(pending, task{text: ","})
				}
			}
		default:
			if item.value == scheme.Null {
				chunks = append(chunks, "{}")
				continue
			}
			text, err := jsonScalar(item.value)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, text)
		}
	}
	// Coalesce punctuation and small scalars to reduce output callback overhead.
	var result []string
	var small strings.Builder
	for _, s := range chunks {
		if small.Len()+len(s) > 4096 && small.Len() > 0 {
			result = append(result, small.String())
			small.Reset()
		}
		small.WriteString(s)
	}
	if small.Len() > 0 {
		result = append(result, small.String())
	}
	return result, nil
}
