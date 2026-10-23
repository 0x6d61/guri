package core

import (
	"fmt"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

// SyntaxFailure retains syntax-error arguments as unevaluated datum objects.
type SyntaxFailure struct {
	Message    string
	Irritants  []scheme.Value
	Definition scheme.Source
}

func (e *SyntaxFailure) Error() string {
	if e.Definition.Filename != "" {
		return fmt.Sprintf("%s (transformer at %s:%d:%d)", e.Message, e.Definition.Filename, e.Definition.Line, e.Definition.Column)
	}
	return e.Message
}
func (e *SyntaxFailure) Unwrap() error { return ErrSyntax }
func syntaxFailure(n *reader.Node, args []*reader.Node, message string) error {
	failure := &SyntaxFailure{Message: message}
	for _, node := range args[1:] {
		failure.Irritants = append(failure.Irritants, node.Datum())
	}
	if ident, ok := n.Car.Syntax.Identifier(); ok {
		scopes := ident.Scopes()
		if len(scopes) > 0 {
			if c, ok := scopes[len(scopes)-1].Context().(*definitionContext); ok {
				failure.Definition = c.source
			}
		}
	}
	return &Error{Source: n.Source(), Cause: failure}
}
