package guri

import (
	"io"
	"os"
)

// Interpreter owns a persistent top-level environment. It is single-threaded.
type Interpreter struct {
	global *environment
	// Output receives println output. It defaults to standard output.
	Output io.Writer
}

func NewInterpreter() *Interpreter {
	i := &Interpreter{global: newEnvironment(nil), Output: os.Stdout}
	i.installNumbers()
	i.installCollections()
	i.installMaps()
	i.installPrinting()
	i.installErrors()
	i.installApply()
	return i
}

// Eval reads and evaluates every form in order, returning the last value or nil.
func (i *Interpreter) Eval(filename, source string) (Value, error) {
	forms, err := Read(filename, source)
	if err != nil {
		return nil, err
	}
	return i.body(forms, i.global)
}

func (i *Interpreter) body(forms []Expr, env *environment) (Value, error) {
	var result Value
	for _, expr := range forms {
		value, err := i.eval(expr, env)
		if err != nil {
			return nil, err
		}
		result = value
	}
	return result, nil
}

func (i *Interpreter) eval(expr Expr, env *environment) (Value, error) {
	return i.run(i.evalTail(expr, env))
}

func (i *Interpreter) evalTail(expr Expr, env *environment) (result Value, err error) {
	defer func() { err = atPosition(err, expr.Position) }()
	switch expr.Kind {
	case Nil, Boolean, Integer, Float, String:
		return expr.Value, nil
	case Bytes:
		return BytesValue(expr.Value.(string)), nil
	case Keyword:
		return KeywordValue(expr.Value.(string)), nil
	case Symbol:
		name := expr.Value.(string)
		if c, ok := env.lookup(name); ok {
			return c.value, nil
		}
		return nil, failure(NameError, "undefined name %s", name)
	case Vector:
		values := make(VectorValue, len(expr.Elements))
		for index, child := range expr.Elements {
			value, err := i.eval(child, env)
			if err != nil {
				return nil, err
			}
			values[index] = value
		}
		return values, nil
	case Map:
		m := NewMap()
		for index := 0; index < len(expr.Elements); index += 2 {
			key, err := i.eval(expr.Elements[index], env)
			if err != nil {
				return nil, err
			}
			if !validKey(key) {
				return nil, failure(TypeError, "invalid Map key type %T", key)
			}
			value, err := i.eval(expr.Elements[index+1], env)
			if err != nil {
				return nil, err
			}
			if err := m.Set(key, value); err != nil {
				return nil, err
			}
		}
		return m, nil
	case List:
		if len(expr.Elements) == 0 {
			return NewList(), nil
		}
		head, args := expr.Elements[0], expr.Elements[1:]
		if head.Kind == Symbol {
			if value, handled, err := i.special(head.Value.(string), args, env); handled {
				return value, err
			}
		}
		callee, err := i.eval(head, env)
		if err != nil {
			return nil, err
		}
		values := make([]Value, len(args))
		for index, arg := range args {
			v, err := i.eval(arg, env)
			if err != nil {
				return nil, err
			}
			values[index] = v
		}
		return &invocation{callee: callee, args: values, position: expr.Position}, nil
	}
	return nil, failure(TypeError, "unknown expression kind %s", expr.Kind)
}

func (i *Interpreter) callStep(callee Value, args []Value) (Value, error) {
	if key, ok := callee.(KeywordValue); ok {
		if err := arity("Keyword", len(args), 1, 2); err != nil {
			return nil, err
		}
		values := []Value{args[0], key}
		if len(args) == 2 {
			values = append(values, args[1])
		}
		return mapGet(values)
	}
	if f, ok := callee.(*FunctionValue); ok && f != nil {
		if f.closure != nil {
			local, err := f.closure.bind(args)
			if err != nil {
				return nil, err
			}
			return i.bodyTail(f.closure.body, local)
		}
		if f.call != nil {
			return f.call(args)
		}
	}
	return nil, failure(TypeError, "%T is not callable", callee)
}

func callable(value Value) bool {
	switch v := value.(type) {
	case KeywordValue:
		return true
	case *FunctionValue:
		return v != nil && (v.call != nil || v.closure != nil)
	}
	return false
}

func arity(name string, count, min, max int) error {
	if count < min || (max >= 0 && count > max) {
		if max < 0 {
			return failure(ArityError, "%s expects at least %d arguments, got %d", name, min, count)
		}
		return failure(ArityError, "%s expects %d..%d arguments, got %d", name, min, max, count)
	}
	return nil
}

func (i *Interpreter) special(name string, args []Expr, env *environment) (Value, bool, error) {
	var value Value
	var err error
	switch name {
	case "quote":
		err = arity(name, len(args), 1, 1)
		if err == nil {
			value, err = quoted(args[0])
		}
	case "do":
		value, err = i.bodyTail(args, env)
	case "while":
		value, err = i.while(args, env)
	case "cond":
		value, err = i.cond(args, env)
	case "if":
		err = arity(name, len(args), 2, 3)
		if err == nil {
			var condition Value
			condition, err = i.eval(args[0], env)
			if err == nil {
				if Truthy(condition) {
					value, err = i.evalTail(args[1], env)
				} else if len(args) == 3 {
					value, err = i.evalTail(args[2], env)
				}
			}
		}
	case "and", "or":
		if name == "and" {
			value = true
		}
		for index, arg := range args {
			if index == len(args)-1 {
				value, err = i.evalTail(arg, env)
				break
			}
			value, err = i.eval(arg, env)
			if err != nil {
				break
			}
			if (name == "and" && !Truthy(value)) || (name == "or" && Truthy(value)) {
				break
			}
		}
	case "def", "set!":
		value, err = i.binding(name, args, env)
	case "defn":
		err = arity(name, len(args), 2, -1)
		if err == nil {
			// Reuse def and fn so their scope and validation rules stay identical.
			function := Expr{
				Kind: List, Position: args[1].Position,
				Elements: append([]Expr{{Kind: Symbol, Value: "fn", Position: args[1].Position}}, args[1:]...),
			}
			value, err = i.binding("def", []Expr{args[0], function}, env)
		}
	case "fn":
		value, err = i.function(args, env)
	case "let":
		value, err = i.let(args, env)
	default:
		return nil, false, nil
	}
	return value, true, err
}

func (i *Interpreter) while(args []Expr, env *environment) (Value, error) {
	if err := arity("while", len(args), 1, -1); err != nil {
		return nil, err
	}
	for {
		condition, err := i.eval(args[0], env)
		if err != nil {
			return nil, err
		}
		if !Truthy(condition) {
			return nil, nil
		}
		if _, err := i.body(args[1:], env); err != nil {
			return nil, err
		}
	}
}

func (i *Interpreter) cond(args []Expr, env *environment) (Value, error) {
	if len(args)%2 != 0 {
		return nil, failure(ArityError, "cond requires condition/expression pairs, got %d arguments", len(args))
	}
	for index := 0; index < len(args); index += 2 {
		condition, err := i.eval(args[index], env)
		if err != nil {
			return nil, err
		}
		if Truthy(condition) {
			return i.evalTail(args[index+1], env)
		}
	}
	return nil, nil
}

func (i *Interpreter) binding(name string, args []Expr, env *environment) (Value, error) {
	if err := arity(name, len(args), 2, 2); err != nil {
		return nil, err
	}
	if args[0].Kind != Symbol {
		return nil, failure(DefinitionError, "%s requires a symbol", name)
	}
	key := args[0].Value.(string)
	if name == "def" {
		if env != i.global {
			return nil, failure(DefinitionError, "def is only allowed at top level")
		}
		if _, exists := env.bindings[key]; exists {
			return nil, failure(DefinitionError, "%s is already defined; use set!", key)
		}
		value, err := i.eval(args[1], env)
		if err != nil {
			return nil, err
		}
		return value, env.define(key, value)
	}
	c, exists := env.lookup(key)
	if !exists {
		return nil, failure(DefinitionError, "cannot set! undefined name %s", key)
	}
	value, err := i.eval(args[1], env)
	if err != nil {
		return nil, err
	}
	c.value = value
	return value, nil
}

func (i *Interpreter) let(args []Expr, env *environment) (Value, error) {
	if err := arity("let", len(args), 1, -1); err != nil {
		return nil, err
	}
	if args[0].Kind != Vector || len(args[0].Elements)%2 != 0 {
		return nil, failure(DefinitionError, "let requires a vector of name/expression pairs")
	}
	local := newEnvironment(env)
	bindings := args[0].Elements
	for index := 0; index < len(bindings); index += 2 {
		if bindings[index].Kind != Symbol {
			return nil, failure(DefinitionError, "let binding name must be a symbol")
		}
		name := bindings[index].Value.(string)
		if _, exists := local.bindings[name]; exists {
			return nil, failure(DefinitionError, "duplicate let binding %s", name)
		}
		value, err := i.eval(bindings[index+1], local)
		if err != nil {
			return nil, err
		}
		if err := local.define(name, value); err != nil {
			return nil, err
		}
	}
	return i.bodyTail(args[1:], local)
}

func (i *Interpreter) function(args []Expr, env *environment) (Value, error) {
	if err := arity("fn", len(args), 1, -1); err != nil {
		return nil, err
	}
	if args[0].Kind != Vector {
		return nil, failure(DefinitionError, "fn parameters must be a vector")
	}
	params := args[0].Elements
	names := []string{}
	rest := ""
	seen := make(map[string]bool)
	for index := 0; index < len(params); index++ {
		if params[index].Kind != Symbol {
			return nil, failure(DefinitionError, "fn parameter must be a symbol")
		}
		name := params[index].Value.(string)
		if name == "&" {
			if index+2 != len(params) || params[index+1].Kind != Symbol || params[index+1].Value == "&" {
				return nil, failure(DefinitionError, "& must be followed by a final rest parameter")
			}
			rest = params[index+1].Value.(string)
			if seen[rest] {
				return nil, failure(DefinitionError, "duplicate fn parameter %s", rest)
			}
			break
		}
		if seen[name] {
			return nil, failure(DefinitionError, "duplicate fn parameter %s", name)
		}
		seen[name] = true
		names = append(names, name)
	}
	return &FunctionValue{closure: &closure{names: names, rest: rest, body: args[1:], env: env}}, nil
}

func quoted(expr Expr) (Value, error) {
	switch expr.Kind {
	case Bytes:
		return BytesValue(expr.Value.(string)), nil
	case Symbol:
		return SymbolValue(expr.Value.(string)), nil
	case Keyword:
		return KeywordValue(expr.Value.(string)), nil
	case List, Vector, Map:
		values := make(VectorValue, len(expr.Elements))
		for index, child := range expr.Elements {
			value, err := quoted(child)
			if err != nil {
				return nil, err
			}
			values[index] = value
		}
		switch expr.Kind {
		case List:
			return NewList(values...), nil
		case Vector:
			return values, nil
		default:
			m := NewMap()
			for index := 0; index < len(values); index += 2 {
				if err := m.Set(values[index], values[index+1]); err != nil {
					return nil, err
				}
			}
			return m, nil
		}
	default:
		return expr.Value, nil
	}
}
