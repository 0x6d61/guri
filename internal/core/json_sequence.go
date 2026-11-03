package core

import (
	"errors"
	"io"

	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func (in *jsonInput) character(c *vm.Context, done func(*vm.Context, rune, bool) (vm.Action, error)) (vm.Action, error) {
	if err := c.Context().Err(); err != nil {
		return vm.Action{}, err
	}
	if in.port != nil {
		ch, err := in.port.ReadChar(false)
		if err != nil && !errors.Is(err, io.EOF) {
			return jsonFailure(c, err)
		}
		return done(c, ch, errors.Is(err, io.EOF))
	}
	return vm.Invoke(in.chars, nil, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
		v, err := r.Single()
		if err != nil {
			return jsonFailure(c, err)
		}
		if v == scheme.EOF {
			return done(c, 0, true)
		}
		ch, ok := v.(scheme.Character)
		if !ok {
			return jsonFailure(c, scheme.ErrType)
		}
		return done(c, ch.Rune(), false)
	}), nil
}

func (j *jsonLibrary) sequence(c *vm.Context, name string, a []scheme.Value) (vm.Action, error) {
	value := j.input.value(c)
	if len(a) > 0 {
		value = a[0]
	}
	input, err := newJSONInput(value)
	if err != nil {
		return jsonFailure(c, err)
	}
	if name == "json-lines-read" {
		finished := false
		return vm.Return(vm.Native(name+" generator", 0, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if len(a) != 0 {
				return jsonFailure(c, vm.ErrArity)
			}
			if finished {
				return vm.Return(scheme.EOF), nil
			}
			g, err := j.generator(c, value)
			if err != nil {
				return jsonFailure(c, err)
			}
			read := vm.Native("read JSON line", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) { return j.read(c, g, nil) })
			return vm.Invoke(read, nil, func(_ *vm.Context, results scheme.Results) (vm.Action, error) {
				v, err := results.Single()
				if err != nil {
					return vm.Action{}, err
				}
				if v == scheme.EOF {
					finished = true
				}
				return vm.Return(v), nil
			}), nil
		})), nil
	}
	sequence := &jsonSequence{library: j, input: input}
	return vm.Return(vm.Native(name+" generator", 0, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		if len(a) != 0 {
			return jsonFailure(c, vm.ErrArity)
		}
		return sequence.next(c)
	})), nil
}

// RFC 7464 frames records with RS. Build only the current value while reading,
// discard damaged records, and resume at the next RS. A complete value followed
// by LF can be delivered without waiting for the next record or physical EOF.
type jsonSequence struct {
	library                      *jsonLibrary
	input                        *jsonInput
	started, eof                 bool
	parser                       *jsonParser
	frame                        *jsonReadFrame
	value                        scheme.Value
	invalid, complete, separator bool
}

func (s *jsonSequence) start(c *vm.Context) error {
	depth, err := jsonLimit(s.library.depth.value(c))
	if err != nil {
		return err
	}
	chars, err := jsonLimit(s.library.characters.value(c))
	if err != nil {
		return err
	}
	s.parser = &jsonParser{symbols: s.library.symbols, depthLimit: depth, characterLimit: chars}
	s.started = true
	s.invalid = false
	s.complete = false
	s.separator = false
	s.frame = nil
	s.value = nil
	return nil
}
func (s *jsonSequence) feed(ch rune, eof bool) error {
	if err := s.parser.feed(ch, eof); err != nil {
		return err
	}
	if s.invalid {
		return nil
	}
	if s.complete {
		if !eof {
			if !jsonSpace(ch) {
				s.invalid = true
				s.value = nil
			} else {
				s.separator = true
			}
		}
		return nil
	}
	for {
		event, err := s.parser.next()
		if errors.Is(err, jsonNeedCharacter) {
			return nil
		}
		if errors.Is(err, jsonDepthLimit) {
			return err
		}
		if err != nil {
			s.invalid = true
			s.frame = nil
			s.value = nil
			return nil
		}
		if event == scheme.EOF {
			s.complete = true
			if !eof && jsonSpace(ch) {
				s.separator = true
			}
			return nil
		}
		if symbol, ok := event.(*scheme.Symbol); ok {
			switch symbol.Name() {
			case "array-start", "object-start":
				s.frame = &jsonReadFrame{object: symbol.Name() == "object-start", parent: s.frame}
				continue
			case "array-end", "object-end":
				event = s.library.readValue(s.frame)
				s.frame = s.frame.parent
			}
		}
		if s.frame == nil {
			s.value = event
		} else {
			s.frame = s.frame.add(event)
		}
	}
}
func (s *jsonSequence) valid() bool {
	if s.invalid || !s.complete || s.value == nil {
		return false
	}
	switch s.value.(type) {
	case scheme.Boolean, *scheme.Integer, scheme.Float, *scheme.Symbol:
		return s.separator
	}
	return true
}
func (s *jsonSequence) next(c *vm.Context) (vm.Action, error) {
	// The trampoline bounds Go stack use even when skipping long invalid records.
	step := vm.Native("JSON sequence scan", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) {
		if s.eof {
			return vm.Return(scheme.EOF), nil
		}
		return s.input.character(c, func(c *vm.Context, ch rune, eof bool) (vm.Action, error) {
			if !eof && ch != 0x1e {
				if s.started {
					if err := s.feed(ch, false); err != nil {
						return jsonFailure(c, err)
					}
					if ch == '\n' && s.valid() {
						v := s.value
						s.started = false
						s.parser = nil
						s.frame = nil
						s.value = nil
						return vm.Return(v), nil
					}
				}
				return s.next(c)
			}
			s.eof = eof
			var result scheme.Value
			if s.started {
				if err := s.feed(0, true); err != nil {
					return jsonFailure(c, err)
				}
				if s.valid() {
					result = s.value
				}
			}
			s.started = false
			s.parser = nil
			s.frame = nil
			s.value = nil
			if !eof {
				if err := s.start(c); err != nil {
					return jsonFailure(c, err)
				}
			}
			if result != nil {
				return vm.Return(result), nil
			}
			if eof {
				return vm.Return(scheme.EOF), nil
			}
			return s.next(c)
		})
	})
	return vm.Invoke(step, nil, nil), nil
}
