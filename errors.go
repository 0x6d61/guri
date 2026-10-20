package guri

import "fmt"

// ErrorKind is the language's runtime error classification.
type ErrorKind string

const (
	NameError       ErrorKind = "NameError"
	DefinitionError ErrorKind = "DefinitionError"
	ArityError      ErrorKind = "ArityError"
	TypeError       ErrorKind = "TypeError"
	IndexError      ErrorKind = "IndexError"
	ArithmeticError ErrorKind = "ArithmeticError"
	UserError       ErrorKind = "UserError"
)

type RuntimeError struct {
	Kind     ErrorKind
	Position Position
	Cause    string
}

func (e *RuntimeError) Error() string {
	if e.Position.Line == 0 {
		return fmt.Sprintf("%s: %s", e.Kind, e.Cause)
	}
	return fmt.Sprintf("%s:%d:%d: %s: %s", e.Position.Filename, e.Position.Line, e.Position.Column, e.Kind, e.Cause)
}

func failure(kind ErrorKind, format string, args ...any) error {
	return &RuntimeError{Kind: kind, Cause: fmt.Sprintf(format, args...)}
}

func (i *Interpreter) installErrors() {
	i.native("error", 1, 1, func(args []Value) (Value, error) {
		message, ok := args[0].(string)
		if !ok {
			return nil, failure(TypeError, "error requires a String message, got %T", args[0])
		}
		return nil, failure(UserError, "%s", message)
	})
}
