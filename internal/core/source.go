package core

import (
	"context"
	"errors"
	"io"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

// SourceLoader owns search policy. The compiler supplies the including filename.
type SourceLoader func(context.Context, string, string) (filename, source string, err error)

var ErrSourceLoader = errors.New("source loader is not configured")

func (e *Engine) SetSourceLoader(loader SourceLoader) { e.loader = loader }
func (e *Engine) SetFeatures(features []string, library func([]scheme.Value) bool) {
	e.features = map[string]bool{}
	for _, f := range features {
		e.features[f] = true
	}
	e.libraries = library
}
func (b *builder) sourceExpansion(n *reader.Node) (*reader.Node, error) {
	xs, err := nodes(n)
	if err != nil {
		return nil, err
	}
	l := b.lowerer(n.Source())
	kind := b.headForm(n)
	if kind == condExpand {
		for i, clause := range xs[1:] {
			parts, err := nodes(clause)
			if err != nil {
				return nil, err
			}
			if len(parts) == 0 {
				return nil, syntax(clause, "empty cond-expand clause")
			}
			isElse := false
			if ident, ok := parts[0].Datum().(*scheme.Symbol); ok {
				isElse = ident.Name() == "else"
			}
			if isElse && i != len(xs)-2 {
				return nil, syntax(clause, "else must be last")
			}
			yes := isElse
			if !isElse {
				yes, err = b.feature(parts[0])
				if err != nil {
					return nil, err
				}
			}
			if yes {
				return l.seq(parts[1:]), nil
			}
		}
		return atom(scheme.Unspecified, l.pos), nil
	}
	if len(xs) < 2 {
		return nil, syntax(n, "include requires at least one filename")
	}
	if b.engine.loader == nil {
		return nil, &Error{Source: n.Source(), Cause: ErrSourceLoader}
	}
	var included []*reader.Node
	for _, node := range xs[1:] {
		path, ok := node.Datum().(*scheme.String)
		if !ok {
			return nil, syntax(node, "include filename must be a string literal")
		}
		filename, source, err := b.engine.loader(b.ctx, n.Source().Filename, path.Text())
		if err != nil {
			return nil, &Error{Source: node.Source(), Cause: err}
		}
		r, err := reader.New(filename, source, b.engine.symbols)
		if err != nil {
			return nil, err
		}
		r.SetFoldCase(kind == includeCI)
		for {
			form, err := r.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			included = append(included, form)
		}
	}
	return l.form("begin", included...), nil
}
func (b *builder) feature(n *reader.Node) (bool, error) {
	if name, ok := n.Datum().(*scheme.Symbol); ok {
		return b.engine.features[name.Name()], nil
	}
	xs, err := nodes(n)
	if err != nil {
		return false, err
	}
	if len(xs) == 0 {
		return false, syntax(n, "empty feature requirement")
	}
	name, ok := xs[0].Datum().(*scheme.Symbol)
	if !ok {
		return false, syntax(n, "invalid feature operator")
	}
	switch name.Name() {
	case "and", "or":
		answer := name.Name() == "and"
		for _, x := range xs[1:] {
			v, err := b.feature(x)
			if err != nil {
				return false, err
			}
			if name.Name() == "and" {
				answer = answer && v
			} else {
				answer = answer || v
			}
		}
		return answer, nil
	case "not":
		if len(xs) != 2 {
			return false, syntax(n, "not feature requires one argument")
		}
		v, err := b.feature(xs[1])
		return !v, err
	case "library":
		if len(xs) != 2 {
			return false, syntax(n, "library feature requires a library name")
		}
		parts, err := nodes(xs[1])
		if err != nil {
			return false, err
		}
		if len(parts) == 0 {
			return false, syntax(n, "empty library name")
		}
		var name []scheme.Value
		for _, p := range parts {
			switch v := p.Datum().(type) {
			case *scheme.Symbol:
			case *scheme.Integer:
				if v.BigInt().Sign() < 0 {
					return false, syntax(p, "negative library component")
				}
			default:
				return false, syntax(p, "invalid library component")
			}
			name = append(name, p.Datum())
		}
		return b.engine.libraries != nil && b.engine.libraries(name), nil
	default:
		return false, syntax(n, "unknown feature operator")
	}
}
