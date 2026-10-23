package vm

import (
	"context"
	"fmt"

	"github.com/0x6d61/guri/internal/scheme"
)

type frame struct {
	code   *Code
	env    *environment
	pc     int
	stack  []scheme.Value
	resume Resume
	source scheme.Source
}
type continuation struct {
	owner   *Machine
	frames  []frame
	dynamic *Dynamic
}

func (*continuation) ProcedureCode() {}
func copyFrames(frames []frame) []frame {
	out := append([]frame(nil), frames...)
	for i := range out {
		out[i].stack = append([]scheme.Value(nil), out[i].stack...)
	}
	return out
}

type Stats struct{ Steps, PeakFrames int }
type Machine struct {
	frames  []frame
	result  scheme.Results
	dynamic *Dynamic
	stats   Stats
	source  scheme.Source
	running bool
}

func New() *Machine             { return &Machine{} }
func (m *Machine) Stats() Stats { return m.stats }

type Context struct{ m *Machine }

func (c *Context) Capture() *scheme.Procedure {
	return scheme.NewProcedure(&continuation{owner: c.m, frames: copyFrames(c.m.frames), dynamic: c.m.dynamic})
}
func (c *Context) Dynamic() *Dynamic { return c.m.dynamic }
func (m *Machine) push(f frame) {
	m.frames = append(m.frames, f)
	if len(m.frames) > m.stats.PeakFrames {
		m.stats.PeakFrames = len(m.frames)
	}
}

// Run starts a fresh execution. Saved continuations remain usable on this machine
// after earlier Run calls finish; locations are shared, control stacks are copied.
func (m *Machine) Run(ctx context.Context, p *scheme.Procedure, args ...scheme.Value) (result scheme.Results, err error) {
	if m.running {
		return result, ErrReentrant
	}
	m.running = true
	m.frames = nil
	m.result = scheme.NewResults()
	m.dynamic = nil
	m.stats = Stats{}
	m.source = scheme.Source{}
	defer func() {
		m.running = false
		m.frames = nil
		m.dynamic = nil
		if err != nil {
			if _, ok := err.(*Error); !ok {
				err = &Error{m.source, err}
			}
		}
	}()
	c := &Context{m: m}
	a := Invoke(p, args, nil)
	pending := true
	for {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		m.stats.Steps++
		if pending {
			pending = false
			if a.kind == enteredAction {
				m.dynamic = a.dynamic
				a = *a.next
				pending = true
				continue
			}
			if a.switchDynamic && m.dynamic != a.dynamic {
				a = m.advanceDynamic(a)
				pending = true
				continue
			}
			switch a.kind {
			case jumpAction:
				m.frames = copyFrames(a.continuation.frames)
				m.result = a.results
				if len(m.frames) > m.stats.PeakFrames {
					m.stats.PeakFrames = len(m.frames)
				}
			case returnAction:
				m.result = a.results
			case invokeAction:
				proc, ok := a.target.(*scheme.Procedure)
				if !ok || proc == nil {
					return result, scheme.ErrType
				}
				if a.resume != nil {
					m.push(frame{resume: a.resume, source: m.source})
				}
				switch impl := proc.Code().(type) {
				case *closure:
					s := impl.code.spec
					n := s.Required
					if len(a.args) < n || !s.Rest && len(a.args) != n {
						return result, ErrArity
					}
					env := &environment{parent: impl.env, cells: make([]*scheme.Cell, s.Slots)}
					for i := range env.cells {
						env.cells[i] = scheme.NewUninitializedCell()
					}
					for i := 0; i < n; i++ {
						env.cells[i].Set(a.args[i])
					}
					if s.Rest {
						env.cells[n].Set(scheme.List(a.args[n:]...))
					}
					m.push(frame{code: impl.code, env: env})
					m.result = scheme.NewResults()
				case *native:
					if len(a.args) < impl.min || impl.max >= 0 && len(a.args) > impl.max {
						return result, fmt.Errorf("%w: %s", ErrArity, impl.name)
					}
					var e error
					a, e = impl.fn(c, append([]scheme.Value(nil), a.args...))
					if e != nil {
						return result, e
					}
					pending = true
					continue
				case *continuation:
					if impl.owner != m {
						return result, ErrContinuationOwner
					}
					a = Action{kind: jumpAction, continuation: impl, results: scheme.NewResults(a.args...), dynamic: impl.dynamic, switchDynamic: true}
					pending = true
					continue
				default:
					return result, scheme.ErrType
				}
			default:
				return result, ErrBytecode
			}
		}
		if len(m.frames) == 0 {
			return m.result, nil
		}
		top := len(m.frames) - 1
		f := &m.frames[top]
		if f.resume != nil {
			resume := f.resume
			m.source = f.source
			m.frames[top] = frame{}
			m.frames = m.frames[:top]
			var e error
			a, e = resume(c, m.result)
			if e != nil {
				return result, e
			}
			pending = true
			continue
		}
		s := f.code.spec
		if f.pc < 0 || f.pc >= len(s.Instructions) {
			return result, ErrBytecode
		}
		i := s.Instructions[f.pc]
		f.pc++
		m.source = i.Source
		switch i.Op {
		case Const:
			m.result = scheme.NewResults(s.Constants[i.A])
		case Push:
			v, e := m.result.Single()
			if e != nil {
				return result, e
			}
			f.stack = append(f.stack, v)
		case Pop:
			if len(f.stack) == 0 {
				return result, ErrBytecode
			}
			last := len(f.stack) - 1
			m.result = scheme.NewResults(f.stack[last])
			f.stack[last] = nil
			f.stack = f.stack[:last]
		case Local, Global:
			var cell *scheme.Cell
			var e error
			if i.Op == Global {
				cell = s.Globals[i.A]
			} else {
				cell, e = f.env.cell(i.A, i.B)
				if e != nil {
					return result, e
				}
			}
			v, e := cell.Get()
			if e != nil {
				return result, e
			}
			m.result = scheme.NewResults(v)
		case InitLocal, SetLocal, SetGlobal, InitGlobal:
			v, e := m.result.Single()
			if e != nil {
				return result, e
			}
			var cell *scheme.Cell
			if i.Op == SetGlobal || i.Op == InitGlobal {
				cell = s.Globals[i.A]
			} else {
				cell, e = f.env.cell(i.A, i.B)
				if e != nil {
					return result, e
				}
			}
			if i.Op != InitLocal && i.Op != InitGlobal {
				if _, e = cell.Get(); e != nil {
					return result, e
				}
			}
			cell.Set(v)
			m.result = scheme.NewResults(scheme.Unspecified)
		case MakeClosure:
			m.result = scheme.NewResults(scheme.NewProcedure(&closure{code: s.Functions[i.A], env: f.env}))
		case Jump:
			f.pc = i.A
		case JumpFalse:
			v, e := m.result.Single()
			if e != nil {
				return result, e
			}
			if !scheme.Truthy(v) {
				f.pc = i.A
			}
		case Call, TailCall:
			if i.A >= len(f.stack) {
				return result, ErrBytecode
			}
			start := len(f.stack) - i.A - 1
			target := f.stack[start]
			args := append([]scheme.Value(nil), f.stack[start+1:]...)
			clear(f.stack[start:])
			f.stack = f.stack[:start]
			if i.Op == TailCall {
				m.frames[top] = frame{}
				m.frames = m.frames[:top]
			}
			a = Invoke(target, args, nil)
			pending = true
		case ReturnOp:
			m.frames[top] = frame{}
			m.frames = m.frames[:top]
		default:
			return result, ErrBytecode
		}
	}
}
