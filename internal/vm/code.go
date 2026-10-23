// Package vm executes internal Scheme bytecode without traversing syntax trees.
package vm

import (
	"errors"
	"fmt"

	"github.com/0x6d61/guri/internal/scheme"
)

type Op uint8

const (
	Const Op = iota
	Push
	Pop
	Local
	Global
	InitLocal
	SetLocal
	SetGlobal
	InitGlobal
	MakeClosure
	Jump
	JumpFalse
	Call
	TailCall
	ReturnOp
)

// A/B encode an index, depth/slot, absolute target, or argument count.
type Instruction struct {
	Op     Op
	A, B   int
	Source scheme.Source
}
type Spec struct {
	Name         string
	Required     int
	Rest         bool
	Slots        int
	Instructions []Instruction
	Constants    []scheme.Value
	Functions    []*Code
	Globals      []*scheme.Cell
}

// Code owns its instruction tables. Scheme constants and binding cells retain identity.
type Code struct{ spec Spec }

func NewCode(s Spec) (*Code, error) {
	bad := func() (*Code, error) { return nil, ErrBytecode }
	params := s.Required
	if s.Rest {
		params++
	}
	if s.Required < 0 || s.Slots < params || len(s.Instructions) == 0 {
		return bad()
	}
	for _, c := range s.Globals {
		if c == nil {
			return bad()
		}
	}
	for _, c := range s.Functions {
		if c == nil {
			return bad()
		}
	}
	for _, v := range s.Constants {
		if v == nil {
			return bad()
		}
	}
	for _, i := range s.Instructions {
		if i.Op > ReturnOp {
			return bad()
		}
		switch i.Op {
		case Const:
			if i.A < 0 || i.A >= len(s.Constants) {
				return bad()
			}
		case MakeClosure:
			if i.A < 0 || i.A >= len(s.Functions) {
				return bad()
			}
		case Global, SetGlobal, InitGlobal:
			if i.A < 0 || i.A >= len(s.Globals) {
				return bad()
			}
		case Local, InitLocal, SetLocal:
			if i.A < 0 || i.B < 0 || i.A == 0 && i.B >= s.Slots {
				return bad()
			}
		case Jump, JumpFalse:
			if i.A < 0 || i.A >= len(s.Instructions) {
				return bad()
			}
		case Call, TailCall:
			if i.A < 0 {
				return bad()
			}
		}
	}
	s.Instructions = append([]Instruction(nil), s.Instructions...)
	s.Constants = append([]scheme.Value(nil), s.Constants...)
	s.Functions = append([]*Code(nil), s.Functions...)
	s.Globals = append([]*scheme.Cell(nil), s.Globals...)
	return &Code{spec: s}, nil
}

var (
	ErrBytecode          = errors.New("invalid bytecode")
	ErrArity             = errors.New("incorrect argument count")
	ErrContinuationOwner = errors.New("continuation belongs to another machine")
	ErrReentrant         = errors.New("machine is already running")
)

type Error struct {
	Source scheme.Source
	Cause  error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: VM: %v", e.Source.Filename, e.Source.Line, e.Source.Column, e.Cause)
}
func (e *Error) Unwrap() error { return e.Cause }

type environment struct {
	parent *environment
	cells  []*scheme.Cell
}

func (e *environment) cell(depth, slot int) (*scheme.Cell, error) {
	for ; depth > 0 && e != nil; depth-- {
		e = e.parent
	}
	if e == nil || slot < 0 || slot >= len(e.cells) {
		return nil, ErrBytecode
	}
	return e.cells[slot], nil
}

type closure struct {
	code *Code
	env  *environment
}

func (*closure) ProcedureCode() {}
func Closure(code *Code) *scheme.Procedure {
	if code == nil {
		panic("vm: nil code")
	}
	return scheme.NewProcedure(&closure{code: code})
}

// NativeFunc must return an Action rather than recursively invoking Machine.Run.
type NativeFunc func(*Context, []scheme.Value) (Action, error)

// Callable is an internal resumable native code object (including parameters).
type Callable interface {
	scheme.ProcedureCode
	Call(*Context, []scheme.Value) (Action, error)
}
type native struct {
	name     string
	min, max int
	fn       NativeFunc
}

func (*native) ProcedureCode() {}
func (n *native) Call(c *Context, args []scheme.Value) (Action, error) {
	if len(args) < n.min || n.max >= 0 && len(args) > n.max {
		return Action{}, fmt.Errorf("%w: %s", ErrArity, n.name)
	}
	return n.fn(c, args)
}
func Native(name string, min, max int, fn NativeFunc) *scheme.Procedure {
	if min < 0 || max >= 0 && max < min || fn == nil {
		panic("vm: invalid native procedure")
	}
	return scheme.NewProcedure(&native{name, min, max, fn})
}

// Resume is immutable continuation state: captured mutable Scheme locations may
// be shared, but Go counters/slices that track remaining control must not change.
type Resume func(*Context, scheme.Results) (Action, error)
type actionKind uint8

const (
	returnAction actionKind = iota
	invokeAction
	jumpAction
	enteredAction
	failAction
)

type Action struct {
	cause         error
	continuation  *continuation
	next          *Action
	kind          actionKind
	results       scheme.Results
	target        scheme.Value
	args          []scheme.Value
	resume        Resume
	dynamic       *Dynamic
	switchDynamic bool
}

func Return(values ...scheme.Value) Action { return Action{results: scheme.NewResults(values...)} }
func Invoke(target scheme.Value, args []scheme.Value, resume Resume) Action {
	return Action{kind: invokeAction, target: target, args: append([]scheme.Value(nil), args...), resume: resume}
}
func (a Action) WithDynamic(d *Dynamic) Action { a.dynamic = d; a.switchDynamic = true; return a }

// Dynamic is an immutable identity chain for wind, handler and parameter state.
type Dynamic struct {
	before, after *scheme.Procedure
	parent        *Dynamic
	payload       any
}

func NewDynamic(parent *Dynamic, payload any) *Dynamic {
	return &Dynamic{parent: parent, payload: payload}
}
func (d *Dynamic) Parent() *Dynamic { return d.parent }
func (d *Dynamic) Payload() any     { return d.payload }

// NewWind creates a distinct dynamic extent; callbacks run outside that extent.
func NewWind(parent *Dynamic, before, after *scheme.Procedure) *Dynamic {
	return &Dynamic{parent: parent, before: before, after: after}
}

// Fail delivers a host error after any requested dynamic transfer has finished.
func Fail(err error) Action {
	if err == nil {
		panic("vm: nil failure")
	}
	return Action{kind: failAction, cause: err}
}
