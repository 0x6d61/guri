package reader

import (
	"strings"

	"github.com/0x6d61/guri/internal/scheme"
)

func (r *Reader) label(pos scheme.Source) (*Node, error) {
	r.advance() // #
	start := r.offset
	for !r.done() && r.peek() >= '0' && r.peek() <= '9' {
		r.advance()
	}
	key := strings.TrimLeft(string(r.input[start:r.offset]), "0")
	if key == "" {
		key = "0"
	}
	if r.done() || (r.peek() != '=' && r.peek() != '#') {
		return r.fail(pos, "expected = or # after datum label")
	}
	marker := r.advance()
	if marker == '#' {
		target := r.labels[key]
		if target == nil {
			return r.fail(pos, "undefined or forward datum label #"+key+"#")
		}
		n := r.atom(scheme.Unspecified, pos)
		n.alias = target
		return n, nil
	}
	placeholder := r.atom(scheme.Unspecified, pos)
	r.labels[key] = placeholder
	value, err := r.datum()
	if err != nil {
		return nil, err
	}
	placeholder.alias = value
	if _, err := resolveAlias(placeholder); err != nil {
		return r.fail(pos, "datum label refers directly to itself")
	}
	return placeholder, nil
}
func resolveAlias(n *Node) (*Node, error) {
	seen := map[*Node]bool{}
	for n.alias != nil {
		if seen[n] {
			return nil, &Error{Position: n.Source(), Message: "datum label has no concrete object"}
		}
		seen[n] = true
		n = n.alias
	}
	return n, nil
}

// Resolution first assigns all syntax values, then fills graph edges. Existing
// containers provide identities before any back edge is installed; no temporary
// Scheme value type escapes the reader. Each reference keeps its own position.
func (r *Reader) resolveGraphs(roots []*Node) error {
	pending := append([]*Node(nil), roots...)
	seen := map[*Node]bool{}
	var order []*Node
	for len(pending) > 0 {
		n := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		target, err := resolveAlias(n)
		if err != nil {
			return err
		}
		n.Syntax = scheme.NewSyntax(target.Datum(), n.Source())
		n.Car, n.Cdr, n.Elements = target.Car, target.Cdr, target.Elements
		order = append(order, n)
		if n.Car != nil {
			pending = append(pending, n.Car, n.Cdr)
		}
		pending = append(pending, n.Elements...)
	}
	for _, n := range order {
		switch value := n.Datum().(type) {
		case *scheme.Pair:
			if err := value.SetCar(n.Car.Datum()); err != nil {
				return err
			}
			if err := value.SetCdr(n.Cdr.Datum()); err != nil {
				return err
			}
		case *scheme.Vector:
			for i, child := range n.Elements {
				if err := value.Set(i, child.Datum()); err != nil {
					return err
				}
			}
		case *scheme.Bytevector:
			for i, child := range n.Elements {
				integer, ok := child.Datum().(*scheme.Integer)
				if !ok {
					return &Error{Position: child.Source(), Message: "bytevector needs exact u8 integers"}
				}
				k := integer.BigInt()
				if !k.IsInt64() || k.Sign() < 0 || k.Int64() > 255 {
					return &Error{Position: child.Source(), Message: "bytevector value outside 0..255"}
				}
				if err := value.Set(i, k.Int64()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
