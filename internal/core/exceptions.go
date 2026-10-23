package core

import (
	"fmt"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

type ErrorKind uint8

const (
	ErrorOrdinary ErrorKind = 0
	ErrorRead     ErrorKind = 1
	ErrorFile     ErrorKind = 2
)

var exceptionType = func() *scheme.RecordType {
	symbols := scheme.NewSymbolTable()
	r, err := scheme.NewRecordType(symbols.Intern("error-object"), []scheme.Field{{Name: symbols.Intern("message")}, {Name: symbols.Intern("irritants")}, {Name: symbols.Intern("kind")}})
	if err != nil {
		panic(err)
	}
	return r
}()

// NewErrorObject is the internal bridge for future read/file primitives.
func NewErrorObject(kind ErrorKind, message string, irritants ...scheme.Value) (scheme.Value, error) {
	if kind&^(ErrorRead|ErrorFile) != 0 {
		return nil, scheme.ErrType
	}
	s, err := scheme.NewString(message)
	if err != nil {
		return nil, err
	}
	return scheme.NewRecord(exceptionType, s, scheme.List(irritants...), scheme.NewInteger(int64(kind)))
}
func isErrorObject(v scheme.Value) bool {
	r, ok := v.(*scheme.Record)
	return ok && r.RecordType() == exceptionType
}

type handlerEntry struct {
	procedure scheme.Value
	parent    *handlerEntry
}
type handlerOverride struct{ handler *handlerEntry }

func currentHandler(d *vm.Dynamic) *handlerEntry {
	for ; d != nil; d = d.Parent() {
		switch state := d.Payload().(type) {
		case *handlerEntry:
			return state
		case *handlerOverride:
			return state.handler
		}
	}
	return nil
}

// Raise is the internal signal bridge used by explicit exception primitives.
func Raise(c *vm.Context, object scheme.Value, continuable bool) (vm.Action, error) {
	old := c.Dynamic()
	handler := currentHandler(old)
	if handler == nil {
		raised := scheme.NewRaised(object, continuable, c.Source())
		if isErrorObject(object) {
			message, err := object.(*scheme.Record).Ref(0)
			if err != nil {
				return vm.Action{}, err
			}
			return vm.Fail(fmt.Errorf("%s: %w", message.(*scheme.String).Text(), raised)).WithDynamic(nil), nil
		}
		return vm.Fail(raised).WithDynamic(nil), nil
	}
	masked := vm.NewDynamic(old, &handlerOverride{handler.parent})
	return vm.Invoke(handler.procedure, []scheme.Value{object}, func(ctx *vm.Context, r scheme.Results) (vm.Action, error) {
		if continuable {
			return vm.Return(r.Values()...).WithDynamic(old), nil
		}
		secondary, err := NewErrorObject(ErrorOrdinary, "non-continuable exception handler returned", object)
		if err != nil {
			return vm.Action{}, err
		}
		return Raise(ctx, secondary, false)
	}).WithDynamic(masked), nil
}
func (e *Engine) installExceptions() {
	e.controls["with-exception-handler"] = vm.Native("with-exception-handler", 2, 2, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		for _, v := range a {
			if _, ok := v.(*scheme.Procedure); !ok {
				return vm.Action{}, scheme.ErrType
			}
		}
		old := c.Dynamic()
		entry := &handlerEntry{a[0], currentHandler(old)}
		return vm.Invoke(a[1], nil, func(_ *vm.Context, r scheme.Results) (vm.Action, error) {
			return vm.Return(r.Values()...).WithDynamic(old), nil
		}).WithDynamic(vm.NewDynamic(old, entry)), nil
	})
	for _, name := range []string{"raise", "raise-continuable"} {
		continuable := name == "raise-continuable"
		e.controls[name] = vm.Native(name, 1, 1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) { return Raise(c, a[0], continuable) })
	}
	e.controls["error"] = vm.Native("error", 1, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
		message, ok := a[0].(*scheme.String)
		if !ok {
			return vm.Action{}, scheme.ErrType
		}
		object, err := NewErrorObject(ErrorOrdinary, message.Text(), a[1:]...)
		if err != nil {
			return vm.Action{}, err
		}
		return Raise(c, object, false)
	})
	e.controls["error-object?"] = vm.Native("error-object?", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		return vm.Return(scheme.Boolean(isErrorObject(a[0]))), nil
	})
	for _, item := range []struct {
		name  string
		index int
	}{{"error-object-message", 0}, {"error-object-irritants", 1}} {
		index := item.index
		e.controls[item.name] = vm.Native(item.name, 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
			if !isErrorObject(a[0]) {
				return vm.Action{}, scheme.ErrType
			}
			value, err := a[0].(*scheme.Record).Ref(index)
			if err != nil {
				return vm.Action{}, err
			}
			return vm.Return(value), nil
		})
	}
	for _, item := range []struct {
		name string
		kind ErrorKind
	}{{"read-error?", ErrorRead}, {"file-error?", ErrorFile}} {
		kind := item.kind
		e.controls[item.name] = vm.Native(item.name, 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
			if !isErrorObject(a[0]) {
				return vm.Return(scheme.False), nil
			}
			value, err := a[0].(*scheme.Record).Ref(2)
			if err != nil {
				return vm.Action{}, err
			}
			bits := value.(*scheme.Integer).BigInt().Uint64()
			return vm.Return(scheme.Boolean(bits&uint64(kind) != 0)), nil
		})
	}
	e.controls["make-parameter"] = vm.Native("make-parameter", 1, 2, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		if len(a) == 1 {
			return vm.Return(scheme.NewProcedure(&parameter{initial: a[0]})), nil
		}
		converter, ok := a[1].(*scheme.Procedure)
		if !ok {
			return vm.Action{}, scheme.ErrType
		}
		return vm.Invoke(converter, []scheme.Value{a[0]}, func(_ *vm.Context, r scheme.Results) (vm.Action, error) {
			value, err := r.Single()
			if err != nil {
				return vm.Action{}, err
			}
			return vm.Return(scheme.NewProcedure(&parameter{value, converter})), nil
		}), nil
	})
}
