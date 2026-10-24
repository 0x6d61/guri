package core

import (
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

type templateKey struct {
	value    scheme.Value
	path     string
	disabled bool
}

// A template can change list length at each ellipsis. Temporary syntax nodes
// preserve its references until every resulting root and tail is known. Resolve
// those aliases before installing edges, and never modify captured input nodes.
func (b *templateBuilder) resolveGraph(root *reader.Node) (*reader.Node, error) {
	canonical := map[*reader.Node]*reader.Node{}
	resolve := func(n *reader.Node) (*reader.Node, error) {
		var path []*reader.Node
		seen := map[*reader.Node]bool{}
		for b.aliases[n] != nil {
			if final := canonical[n]; final != nil {
				n = final
				break
			}
			if seen[n] {
				return nil, syntax(n, "template repetition leaves a cycle without a datum")
			}
			seen[n] = true
			path = append(path, n)
			n = b.aliases[n]
		}
		for _, node := range path {
			canonical[node] = n
		}
		return n, nil
	}
	pending := []*reader.Node{root}
	visited := map[*reader.Node]bool{}
	var fresh []*reader.Node
	for len(pending) > 0 {
		n := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if visited[n] {
			continue
		}
		visited[n] = true
		target, err := resolve(n)
		if err != nil {
			return nil, err
		}
		n.Syntax, n.Car, n.Cdr, n.Elements = target.Syntax, target.Car, target.Cdr, target.Elements
		if b.borrowed[n] || b.borrowed[target] {
			continue
		}
		fresh = append(fresh, n)
		if n.Car != nil {
			pending = append(pending, n.Car, n.Cdr)
		}
		pending = append(pending, n.Elements...)
	}
	for _, n := range fresh {
		switch x := n.Datum().(type) {
		case *scheme.Pair:
			if n.Car == nil {
				continue
			}
			if err := x.SetCar(n.Car.Datum()); err != nil {
				return nil, err
			}
			if err := x.SetCdr(n.Cdr.Datum()); err != nil {
				return nil, err
			}
		case *scheme.Vector:
			for i, child := range n.Elements {
				if err := x.Set(i, child.Datum()); err != nil {
					return nil, err
				}
			}
		}
	}
	return root, nil
}
