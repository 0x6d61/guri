package guri

func (i *Interpreter) installApply() {
	i.native("apply", 2, -1, func(args []Value) (Value, error) {
		if !callable(args[0]) {
			return nil, failure(TypeError, "apply requires a callable value, got %T", args[0])
		}
		var expanded []Value
		switch last := args[len(args)-1].(type) {
		case VectorValue:
			expanded = last
		case ListValue:
			expanded = last.Values()
		default:
			return nil, failure(TypeError, "apply requires a final Vector or List, got %T", last)
		}
		values := make([]Value, len(args)-2+len(expanded))
		copy(values, args[1:len(args)-1])
		copy(values[len(args)-2:], expanded)
		return &invocation{callee: args[0], args: values}, nil
	})
}
