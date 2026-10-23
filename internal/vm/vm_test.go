package vm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func code(t *testing.T, s vm.Spec) *vm.Code {
	t.Helper()
	c, e := vm.NewCode(s)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func integer(v scheme.Value) int64 { return v.(*scheme.Integer).BigInt().Int64() }
func single(t *testing.T, r scheme.Results, e error) scheme.Value {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
	v, e := r.Single()
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func run(t *testing.T, m *vm.Machine, p *scheme.Procedure, args ...scheme.Value) scheme.Value {
	t.Helper()
	r, e := m.Run(context.Background(), p, args...)
	return single(t, r, e)
}
func TestBytecodeTailCallsAndLexicalCells(t *testing.T) {
	m := vm.New()
	self := scheme.NewUninitializedCell()
	zero := vm.Native("zero", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		return vm.Return(scheme.Boolean(integer(a[0]) == 0)), nil
	})
	dec := vm.Native("dec", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		return vm.Return(scheme.NewInteger(integer(a[0]) - 1)), nil
	})
	c := code(t, vm.Spec{Required: 1, Slots: 1, Globals: []*scheme.Cell{self}, Constants: []scheme.Value{zero, dec, scheme.NewInteger(7)}, Instructions: []vm.Instruction{
		{Op: vm.Const, A: 0}, {Op: vm.Push}, {Op: vm.Local}, {Op: vm.Push}, {Op: vm.Call, A: 1}, {Op: vm.JumpFalse, A: 8}, {Op: vm.Const, A: 2}, {Op: vm.ReturnOp},
		{Op: vm.Global}, {Op: vm.Push}, {Op: vm.Const, A: 1}, {Op: vm.Push}, {Op: vm.Local}, {Op: vm.Push}, {Op: vm.Call, A: 1}, {Op: vm.Push}, {Op: vm.TailCall, A: 1},
	}})
	p := vm.Closure(c)
	self.Set(p)
	if integer(run(t, m, p, scheme.NewInteger(100000))) != 7 {
		t.Fatal("tail result")
	}
	if m.Stats().PeakFrames > 2 {
		t.Fatal("tail calls accumulate frames", m.Stats())
	}
	// A returned closure retains the same mutable binding cell across calls.
	child := code(t, vm.Spec{Instructions: []vm.Instruction{{Op: vm.Local, A: 1}, {Op: vm.ReturnOp}}})
	outer := code(t, vm.Spec{Required: 1, Slots: 1, Functions: []*vm.Code{child}, Instructions: []vm.Instruction{{Op: vm.MakeClosure}, {Op: vm.ReturnOp}}})
	captured := run(t, m, vm.Closure(outer), scheme.NewInteger(42)).(*scheme.Procedure)
	if integer(run(t, m, captured)) != 42 {
		t.Fatal("lexical environment lost")
	}
}
func TestNativeCallbacksMultiShotContinuationAndSharedStore(t *testing.T) {
	m := vm.New()
	var saved *scheme.Procedure
	count := 0
	capture := vm.Native("capture", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) {
		saved = c.Capture()
		return vm.Return(scheme.NewInteger(1)), nil
	})
	wrapper := vm.Native("callback", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		return vm.Invoke(a[0], nil, func(_ *vm.Context, r scheme.Results) (vm.Action, error) {
			count++
			v, e := r.Single()
			if e != nil {
				return vm.Action{}, e
			}
			return vm.Return(scheme.NewInteger(integer(v) + int64(count))), nil
		}), nil
	})
	p := vm.Closure(code(t, vm.Spec{Constants: []scheme.Value{wrapper, capture}, Instructions: []vm.Instruction{{Op: vm.Const}, {Op: vm.Push}, {Op: vm.Const, A: 1}, {Op: vm.Push}, {Op: vm.Call, A: 1}, {Op: vm.ReturnOp}}}))
	if integer(run(t, m, p)) != 2 {
		t.Fatal("callback")
	}
	if integer(run(t, m, saved, scheme.NewInteger(10))) != 12 {
		t.Fatal("first resume")
	}
	if integer(run(t, m, saved, scheme.NewInteger(20))) != 23 {
		t.Fatal("second resume or store rollback")
	}
	_, e := vm.New().Run(context.Background(), saved, scheme.NewInteger(0))
	if !errors.Is(e, vm.ErrContinuationOwner) {
		t.Fatal("cross-machine continuation", e)
	}
}
func TestDeepNonTailCallsMultipleValuesAndDynamicSnapshot(t *testing.T) {
	m := vm.New()
	var recurse *scheme.Procedure
	recurse = vm.Native("recurse", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		n := integer(a[0])
		if n == 0 {
			return vm.Return(scheme.NewInteger(0)), nil
		}
		return vm.Invoke(recurse, []scheme.Value{scheme.NewInteger(n - 1)}, func(_ *vm.Context, r scheme.Results) (vm.Action, error) {
			v, e := r.Single()
			if e != nil {
				return vm.Action{}, e
			}
			return vm.Return(scheme.NewInteger(integer(v) + 1)), nil
		}), nil
	})
	if integer(run(t, m, recurse, scheme.NewInteger(20000))) != 20000 {
		t.Fatal("non-tail callback")
	}
	var saved *scheme.Procedure
	extent := vm.NewDynamic(nil, "parameter-state")
	save := vm.Native("save", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) {
		if c.Dynamic() != extent {
			t.Fatal("dynamic state lost")
		}
		saved = c.Capture()
		return vm.Return(), nil
	})
	enter := vm.Native("enter", 0, 0, func(_ *vm.Context, _ []scheme.Value) (vm.Action, error) {
		return vm.Invoke(save, nil, func(c *vm.Context, r scheme.Results) (vm.Action, error) {
			if c.Dynamic() != extent {
				t.Fatal("dynamic snapshot")
			}
			return vm.Return(r.Values()...), nil
		}).WithDynamic(extent), nil
	})
	r, e := m.Run(context.Background(), enter)
	if e != nil || r.Len() != 0 {
		t.Fatal("zero values", r, e)
	}
	r, e = m.Run(context.Background(), saved, scheme.True, scheme.False)
	if e != nil || r.Len() != 2 {
		t.Fatal("multiple values", r, e)
	}
}
func TestErrorsArityAndCancellation(t *testing.T) {
	m := vm.New()
	pos := scheme.Source{Filename: "test.scm", Line: 3, Column: 2}
	p := vm.Closure(code(t, vm.Spec{Globals: []*scheme.Cell{scheme.NewUninitializedCell()}, Instructions: []vm.Instruction{{Op: vm.Global, Source: pos}, {Op: vm.ReturnOp}}}))
	_, e := m.Run(context.Background(), p)
	var diagnostic *vm.Error
	if !errors.Is(e, scheme.ErrUninitialized) || !errors.As(e, &diagnostic) || diagnostic.Source != pos {
		t.Fatal("source error", e)
	}
	p = vm.Closure(code(t, vm.Spec{Required: 1, Slots: 1, Instructions: []vm.Instruction{{Op: vm.Local}, {Op: vm.ReturnOp}}}))
	_, e = m.Run(context.Background(), p)
	if !errors.Is(e, vm.ErrArity) {
		t.Fatal("arity", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = m.Run(ctx, p, scheme.True)
	if !errors.Is(e, context.Canceled) {
		t.Fatal("cancellation", e)
	}
	_, e = vm.NewCode(vm.Spec{Instructions: []vm.Instruction{{Op: vm.Jump, A: 99}}})
	if !errors.Is(e, vm.ErrBytecode) {
		t.Fatal("bad jump", e)
	}
	p = vm.Closure(code(t, vm.Spec{Instructions: []vm.Instruction{{Op: vm.Push}, {Op: vm.ReturnOp}}}))
	_, e = m.Run(context.Background(), p)
	if !errors.Is(e, scheme.ErrValueCount) {
		t.Fatal("single value context", e)
	}
}

func TestContinuationCopiesPendingOperandsAndSharesCells(t *testing.T) {
	m := vm.New()
	var saved *scheme.Procedure
	cell := scheme.NewCell(scheme.NewInteger(0))
	capture := vm.Native("capture", 0, 0, func(c *vm.Context, _ []scheme.Value) (vm.Action, error) {
		saved = c.Capture()
		return vm.Return(scheme.NewInteger(1)), nil
	})
	add := vm.Native("add-store", 2, 2, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		old, e := cell.Get()
		if e != nil {
			return vm.Action{}, e
		}
		n := integer(old) + integer(a[0]) + integer(a[1])
		cell.Set(scheme.NewInteger(n))
		return vm.Return(scheme.NewInteger(n)), nil
	})
	p := vm.Closure(code(t, vm.Spec{Constants: []scheme.Value{add, capture, scheme.NewInteger(2)}, Instructions: []vm.Instruction{{Op: vm.Const}, {Op: vm.Push}, {Op: vm.Const, A: 1}, {Op: vm.Push}, {Op: vm.Call}, {Op: vm.Push}, {Op: vm.Const, A: 2}, {Op: vm.Push}, {Op: vm.Call, A: 2}, {Op: vm.ReturnOp}}}))
	if integer(run(t, m, p)) != 3 || integer(run(t, m, saved, scheme.NewInteger(10))) != 15 || integer(run(t, m, saved, scheme.NewInteger(20))) != 37 {
		t.Fatal("pending operands or shared cell lost")
	}
}
func TestBytecodeNonTailDepthAndRestLists(t *testing.T) {
	m := vm.New()
	self := scheme.NewUninitializedCell()
	dispatch := vm.Native("dispatch", 2, 2, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		n := integer(a[1])
		if n == 0 {
			return vm.Return(scheme.NewInteger(0)), nil
		}
		return vm.Invoke(a[0], []scheme.Value{scheme.NewInteger(n - 1)}, func(_ *vm.Context, r scheme.Results) (vm.Action, error) {
			v, e := r.Single()
			if e != nil {
				return vm.Action{}, e
			}
			return vm.Return(scheme.NewInteger(integer(v) + 1)), nil
		}), nil
	})
	p := vm.Closure(code(t, vm.Spec{Required: 1, Slots: 1, Constants: []scheme.Value{dispatch}, Globals: []*scheme.Cell{self}, Instructions: []vm.Instruction{{Op: vm.Const}, {Op: vm.Push}, {Op: vm.Global}, {Op: vm.Push}, {Op: vm.Local}, {Op: vm.Push}, {Op: vm.Call, A: 2}, {Op: vm.ReturnOp}}}))
	self.Set(p)
	if integer(run(t, m, p, scheme.NewInteger(10000))) != 10000 {
		t.Fatal("non-tail bytecode returns")
	}
	rest := vm.Closure(code(t, vm.Spec{Required: 1, Rest: true, Slots: 2, Instructions: []vm.Instruction{{Op: vm.Local, B: 1}, {Op: vm.ReturnOp}}}))
	v := run(t, m, rest, scheme.True, scheme.NewInteger(2), scheme.NewInteger(3)).(*scheme.Pair)
	if integer(v.Car()) != 2 || integer(v.Cdr().(*scheme.Pair).Car()) != 3 {
		t.Fatal("rest must be list")
	}
}
