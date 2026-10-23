package guri

import "errors"

// invocation is an internal continuation for a call with no remaining work in
// the current evaluation. It never escapes Eval or enters a language value.
type invocation struct {
	callee   Value
	args     []Value
	position Position
}

type closure struct {
	names []string
	rest  string
	body  []Expr
	env   *environment
}

func (f *closure) bind(values []Value) (*environment, error) {
	max := len(f.names)
	if f.rest != "" {
		max = -1
	}
	if err := arity("fn", len(values), len(f.names), max); err != nil {
		return nil, err
	}
	local := newEnvironment(f.env)
	for index, name := range f.names {
		local.bindings[name] = &cell{values[index]}
	}
	if f.rest != "" {
		v := make(VectorValue, len(values)-len(f.names))
		copy(v, values[len(f.names):])
		local.bindings[f.rest] = &cell{v}
	}
	return local, nil
}

func atPosition(err error, position Position) error {
	var e *RuntimeError
	if errors.As(err, &e) && e.Position.Line == 0 && position.Line != 0 {
		copy := *e
		copy.Position = position
		return &copy
	}
	return err
}

// run consumes calls iteratively. A tail call replaces the current invocation
// instead of retaining its Go frame or an environment chain for returning to it.
func (i *Interpreter) run(value Value, err error) (Value, error) {
	for err == nil {
		call, ok := value.(*invocation)
		if !ok {
			return value, nil
		}
		value, err = i.callStep(call.callee, call.args)
		err = atPosition(err, call.position)
		// Native forwarding procedures (such as apply) inherit their call site.
		if next, ok := value.(*invocation); ok && next.position.Line == 0 {
			next.position = call.position
		}
	}
	return nil, err
}

func (i *Interpreter) apply(callee Value, args []Value) (Value, error) {
	return i.run(&invocation{callee: callee, args: args}, nil)
}

// bodyTail evaluates preceding expressions completely and leaves only the last
// expression eligible to forward its result to the surrounding continuation.
func (i *Interpreter) bodyTail(forms []Expr, env *environment) (Value, error) {
	if len(forms) == 0 {
		return nil, nil
	}
	if _, err := i.body(forms[:len(forms)-1], env); err != nil {
		return nil, err
	}
	return i.evalTail(forms[len(forms)-1], env)
}
