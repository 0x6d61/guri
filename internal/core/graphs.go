package core

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

type graphVisit struct {
	value      scheme.Value
	expression bool
}
type graphTask struct {
	node             *reader.Node
	expression, exit bool
}

// Syntax graph validation masks actual quote operands and self-evaluating
// vectors. Quasiquote uses the stricter pass: circular templates are an error.
// Binding lookup here never creates globals while inspecting syntax data.
func (b *builder) checkGraph(root *reader.Node, literals bool) error {
	states := map[graphVisit]uint8{}
	pending := []graphTask{{node: root, expression: true}}
	for len(pending) > 0 {
		item := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		n := item.node
		kind := n.Datum().Kind()
		if kind != scheme.KindPair && kind != scheme.KindVector {
			continue
		}
		key := graphVisit{n.Datum(), item.expression}
		if !literals {
			key.expression = false
		}
		if item.exit {
			states[key] = 2
			continue
		}
		if states[key] == 1 {
			return syntax(n, "circular reference outside a literal")
		}
		if states[key] == 2 {
			continue
		}
		if literals {
			if kind == scheme.KindVector {
				continue
			}
			if item.expression && n.Car != nil {
				if id, ok := n.Car.Syntax.Identifier(); ok {
					binding := b.engine.find(b.env, b.engine.globals, id)
					if binding != nil && binding.form == quote {
						continue
					}
				}
			}
		}
		states[key] = 1
		pending = append(pending, graphTask{node: n, expression: item.expression, exit: true})
		if n.Car != nil {
			pending = append(pending, graphTask{node: n.Cdr}, graphTask{node: n.Car, expression: true})
		}
		for _, child := range n.Elements {
			pending = append(pending, graphTask{node: child, expression: true})
		}
	}
	return nil
}
