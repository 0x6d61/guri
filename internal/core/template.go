package core

import (
	"context"
	"fmt"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func (t *transformer) templateSequence(n *reader.Node) (xs []*reader.Node, tail *reader.Node, vector bool) {
	if n.Datum().Kind() == scheme.KindVector {
		return n.Elements, atom(scheme.Null, n.Source()), true
	}
	xs, tails := split(n)
	return xs, tails[len(tails)-1], false
}
func (t *transformer) escaped(xs []*reader.Node, tail *reader.Node, vector, disabled bool) bool {
	return !disabled && !vector && tail.Datum() == scheme.Null && len(xs) > 0 && t.isEllipsis(xs[0])
}
func (t *transformer) validateTemplate(n *reader.Node, vars []patternVariable, depth int, disabled bool, b *builder) error {
	if ident, ok := n.Syntax.Identifier(); ok {
		index := t.varIndex(vars, ident)
		if index >= 0 {
			if vars[index].depth > depth {
				return syntax(n, "pattern variable is missing template ellipses")
			}
			return nil
		}
		if !disabled && t.isEllipsis(n) {
			return syntax(n, "unexpected template ellipsis")
		}
		if b.engine.find(b.env, b.engine.globals, ident) == nil {
			b.engine.global(ident)
		}
		return nil
	}
	if n.Datum().Kind() != scheme.KindPair && n.Datum().Kind() != scheme.KindVector {
		return nil
	}
	xs, tail, vector := t.templateSequence(n)
	if t.escaped(xs, tail, vector, disabled) {
		if len(xs) != 2 {
			return syntax(n, "ellipsis escape requires one template")
		}
		return t.validateTemplate(xs[1], vars, depth, true, b)
	}
	for i := 0; i < len(xs); i++ {
		repeats := 0
		for !disabled && i+repeats+1 < len(xs) && t.isEllipsis(xs[i+repeats+1]) {
			repeats++
		}
		for level := 0; level < repeats; level++ {
			if len(t.drivers(xs[i], vars, depth+level)) == 0 {
				return syntax(xs[i], "template repetition has no driving pattern variable")
			}
		}
		if err := t.validateTemplate(xs[i], vars, depth+repeats, disabled, b); err != nil {
			return err
		}
		i += repeats
	}
	if !vector && tail.Datum() != scheme.Null {
		return t.validateTemplate(tail, vars, depth, disabled, b)
	}
	return nil
}
func (t *transformer) drivers(n *reader.Node, vars []patternVariable, depth int) []int {
	seen := map[int]bool{}
	var out []int
	pending := []*reader.Node{n}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if ident, ok := node.Syntax.Identifier(); ok {
			index := t.varIndex(vars, ident)
			if index >= 0 && vars[index].depth > depth && !seen[index] {
				seen[index] = true
				out = append(out, index)
			}
			continue
		}
		if node.Car != nil {
			pending = append(pending, node.Car, node.Cdr)
		}
		pending = append(pending, node.Elements...)
	}
	return out
}
func capAt(c *capture, path []int) (*capture, error) {
	for _, i := range path {
		if c == nil || i < 0 || i >= len(c.children) {
			return nil, ErrSyntax
		}
		c = c.children[i]
	}
	if c == nil {
		return nil, ErrSyntax
	}
	return c, nil
}

type templateBuilder struct {
	ctx         context.Context
	transformer *transformer
	rule        *rule
	captures    captures
	mark        *scheme.Scope
	source      scheme.Source
}

func (t *transformer) expand(use *builder, n *reader.Node) (*reader.Node, error) {
	m := &matcher{use: use, transformer: t}
	for i := range t.rules {
		r := &t.rules[i]
		caps, ok := m.match(r.pattern, n.Cdr)
		if m.err != nil {
			return nil, m.err
		}
		if !ok {
			continue
		}
		builder := &templateBuilder{ctx: use.ctx, transformer: t, rule: r, captures: caps, mark: scheme.NewScopeWithContext(t.context), source: n.Source()}
		result, err := builder.build(r.template, nil, false)
		if err != nil {
			return nil, &Error{Source: n.Source(), Cause: fmt.Errorf("%w (transformer at %s:%d:%d)", err, t.source.Filename, t.source.Line, t.source.Column)}
		}
		return result, nil
	}
	return nil, &Error{Source: n.Source(), Cause: fmt.Errorf("%w: no matching syntax-rules pattern (transformer at %s:%d:%d)", ErrSyntax, t.source.Filename, t.source.Line, t.source.Column)}
}
func (b *templateBuilder) build(n *reader.Node, path []int, disabled bool) (*reader.Node, error) {
	if err := b.ctx.Err(); err != nil {
		return nil, err
	}
	t := b.transformer
	if ident, ok := n.Syntax.Identifier(); ok {
		index := t.varIndex(b.rule.variables, ident)
		if index >= 0 {
			depth := b.rule.variables[index].depth
			if len(path) < depth {
				return nil, ErrSyntax
			}
			c, err := capAt(b.captures[index], path[:depth])
			if err != nil || c.node == nil {
				return nil, ErrSyntax
			}
			return c.node, nil
		}
		scopes := append(ident.Scopes(), b.mark)
		return &reader.Node{Syntax: scheme.NewSyntax(ident.Symbol(), b.source, scopes...)}, nil
	}
	if n.Datum().Kind() != scheme.KindPair && n.Datum().Kind() != scheme.KindVector {
		return atom(n.Datum(), b.source), nil
	}
	xs, tail, vector := t.templateSequence(n)
	if t.escaped(xs, tail, vector, disabled) {
		return b.build(xs[1], path, true)
	}
	var result []*reader.Node
	for i := 0; i < len(xs); i++ {
		repeat := 0
		for !disabled && i+repeat+1 < len(xs) && t.isEllipsis(xs[i+repeat+1]) {
			repeat++
		}
		expanded, err := b.repeated(xs[i], repeat, path, disabled)
		if err != nil {
			return nil, err
		}
		result = append(result, expanded...)
		i += repeat
	}
	if vector {
		values := make([]scheme.Value, len(result))
		for i, x := range result {
			values[i] = x.Datum()
		}
		out := atom(scheme.NewVector(values...), b.source)
		out.Elements = result
		return out, nil
	}
	end, err := b.build(tail, path, disabled)
	if err != nil {
		return nil, err
	}
	return list(result, end, b.source), nil
}
func (b *templateBuilder) repeated(n *reader.Node, levels int, path []int, disabled bool) ([]*reader.Node, error) {
	if levels == 0 {
		one, err := b.build(n, path, disabled)
		if err != nil {
			return nil, err
		}
		return []*reader.Node{one}, nil
	}
	drivers := b.transformer.drivers(n, b.rule.variables, len(path))
	count := -1
	for _, index := range drivers {
		c, err := capAt(b.captures[index], path)
		if err != nil {
			return nil, err
		}
		if count >= 0 && count != len(c.children) {
			return nil, fmt.Errorf("%w: incompatible template repetition lengths", ErrSyntax)
		}
		count = len(c.children)
	}
	if count < 0 {
		return nil, ErrSyntax
	}
	var out []*reader.Node
	for i := 0; i < count; i++ {
		next := append(append([]int(nil), path...), i)
		nodes, err := b.repeated(n, levels-1, next, disabled)
		if err != nil {
			return nil, err
		}
		out = append(out, nodes...)
	}
	return out, nil
}
