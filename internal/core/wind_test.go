package core_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

func windEngine() (*core.Engine, *[]string) {
	e := engine()
	log := []string{}
	e.Bind("record", vm.Native("record", 1, 1, func(_ *vm.Context, a []scheme.Value) (vm.Action, error) {
		log = append(log, a[0].(*scheme.Symbol).Name())
		return vm.Return(scheme.Unspecified), nil
	}))
	return e, &log
}
func TestWindNormalNestedAndEscapeOrder(t *testing.T) {
	for _, test := range []struct {
		s    string
		want []string
	}{
		{`(dynamic-wind (lambda () (record 'before)) (lambda () (record 'body) 42) (lambda () (record 'after)))`, []string{"before", "body", "after"}},
		{`(call/cc (lambda (out) (dynamic-wind (lambda () (record 'outer-in)) (lambda () (dynamic-wind (lambda () (record 'inner-in)) (lambda () (out 42)) (lambda () (record 'inner-out)))) (lambda () (record 'outer-out)))))`, []string{"outer-in", "inner-in", "inner-out", "outer-out"}},
		{`(call/cc (lambda (out) (dynamic-wind (lambda () (record 'before) (out 42)) (lambda () (record 'body)) (lambda () (record 'after)))))`, []string{"before"}},
		{`(call/cc (lambda (out) (dynamic-wind (lambda () (record 'before)) (lambda () (record 'body)) (lambda () (record 'after) (out 42)))))`, []string{"before", "body", "after"}},
	} {
		t.Run(test.s, func(t *testing.T) {
			e, log := windEngine()
			if number(eval(t, e, test.s)) != 42 || !reflect.DeepEqual(*log, test.want) {
				t.Fatal("wind sequence", *log, test.want)
			}
		})
	}
}
func TestWindMultiShotReentryAndSiblingTransfer(t *testing.T) {
	e, log := windEngine()
	s := `(define saved #f) (dynamic-wind (lambda () (record 'in)) (lambda () (call/cc (lambda (k) (set! saved k) 1))) (lambda () (record 'out)))`
	if number(eval(t, e, s)) != 1 || number(eval(t, e, `(saved 2)`)) != 2 || number(eval(t, e, `(saved 3)`)) != 3 {
		t.Fatal("reentry result")
	}
	if !reflect.DeepEqual(*log, []string{"in", "out", "in", "out", "in", "out"}) {
		t.Fatal("reentry hooks", *log)
	}
	*log = nil
	eval(t, e, `(dynamic-wind (lambda () (record 'a-in)) (lambda () (saved 42)) (lambda () (record 'a-out)))`)
	if !reflect.DeepEqual(*log, []string{"a-in", "a-out", "in", "out"}) {
		t.Fatal("sibling transfer", *log)
	}
}
func TestWindSharedPrefixAndMultipleResults(t *testing.T) {
	e, log := windEngine()
	source := `(dynamic-wind (lambda () (record 'outer-in)) (lambda () (call/cc (lambda (out) (dynamic-wind (lambda () (record 'inner-in)) (lambda () (out 1)) (lambda () (record 'inner-out))))) (record 'still-outer) 42) (lambda () (record 'outer-out)))`
	if number(eval(t, e, source)) != 42 || !reflect.DeepEqual(*log, []string{"outer-in", "inner-in", "inner-out", "still-outer", "outer-out"}) {
		t.Fatal("common prefix", *log)
	}
	r, err := e.Eval(context.Background(), "values.scm", `(dynamic-wind (lambda () (values 1 2)) (lambda () (values 20 22)) (lambda () (values)))`)
	if err != nil || r.Len() != 2 {
		t.Fatal("hook result handling", r, err)
	}
}
func TestWindContinuationsInsideHooks(t *testing.T) {
	e, log := windEngine()
	source := `(define before-k #f) (dynamic-wind (lambda () (record 'before) (call/cc (lambda (k) (set! before-k k) 0))) (lambda () (record 'body) 42) (lambda () (record 'after)))`
	eval(t, e, source)
	*log = nil
	if number(eval(t, e, `(before-k 0)`)) != 42 || !reflect.DeepEqual(*log, []string{"body", "after"}) {
		t.Fatal("before resume", *log)
	}
	e, log = windEngine()
	source = `(define after-k #f) (dynamic-wind (lambda () (record 'before)) (lambda () (record 'body) 42) (lambda () (record 'after) (call/cc (lambda (k) (set! after-k k) 0))))`
	eval(t, e, source)
	*log = nil
	if number(eval(t, e, `(after-k 0)`)) != 42 || len(*log) != 0 {
		t.Fatal("after resume", *log)
	}
	for _, s := range []string{`(dynamic-wind 1 (lambda () 0) (lambda () 0))`, `(dynamic-wind (lambda () 0) 1 (lambda () 0))`, `(dynamic-wind (lambda () 0) (lambda () 0) 1)`} {
		_, err := engine().Eval(context.Background(), "bad.scm", s)
		if err == nil {
			t.Fatal("non-procedure hook", s)
		}
	}
}
