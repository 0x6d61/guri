package core

import (
	"context"
	"fmt"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

// templateTail consumes only the ellipsis tokens after the first element.
// Keeping the original cdr node lets template memoization preserve shared tails.
func (t *transformer) templateTail(n *reader.Node, disabled bool) (int, *reader.Node, error) {
	count := 0
	tail := n.Cdr
	seen := map[scheme.Value]bool{}
	for !disabled && tail != nil && tail.Car != nil && t.isEllipsis(tail.Car) {
		if seen[tail.Datum()] {
			return 0, nil, syntax(tail, "cyclic ellipsis sequence")
		}
		seen[tail.Datum()] = true
		count++
		tail = tail.Cdr
	}
	return count, tail, nil
}
func (t *transformer) validateTemplate(n *reader.Node, vars []patternVariable, depth int, disabled bool, b *builder) error {
	type key struct {
		value    scheme.Value
		depth    int
		disabled bool
	}
	seen := map[key]bool{}
	var walk func(*reader.Node, int, bool) error
	walk = func(n *reader.Node, depth int, disabled bool) error {
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
		kind := n.Datum().Kind()
		if kind != scheme.KindPair && kind != scheme.KindVector {
			return nil
		}
		k := key{n.Datum(), depth, disabled}
		if seen[k] {
			return nil
		}
		seen[k] = true
		if kind == scheme.KindPair {
			if n.Car == nil {
				return nil
			}
			if !disabled && t.isEllipsis(n.Car) {
				xs, err := nodes(n)
				if err != nil {
					return err
				}
				if len(xs) != 2 {
					return syntax(n, "ellipsis escape requires one template")
				}
				return walk(xs[1], depth, true)
			}
			repeats, tail, err := t.templateTail(n, disabled)
			if err != nil {
				return err
			}
			for level := 0; level < repeats; level++ {
				if len(t.drivers(n.Car, vars, depth+level)) == 0 {
					return syntax(n.Car, "template repetition has no driving pattern variable")
				}
			}
			if err := walk(n.Car, depth+repeats, disabled); err != nil {
				return err
			}
			return walk(tail, depth, disabled)
		}
		xs := n.Elements
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
			if err := walk(xs[i], depth+repeats, disabled); err != nil {
				return err
			}
			i += repeats
		}
		return nil
	}
	return walk(n, depth, disabled)
}
func (t *transformer) drivers(n *reader.Node, vars []patternVariable, depth int) []int {
	seen := map[int]bool{}
	visited := map[*reader.Node]bool{}
	var out []int
	pending := []*reader.Node{n}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if visited[node] {
			continue
		}
		visited[node] = true
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
	memo        map[templateKey]*reader.Node
	aliases     map[*reader.Node]*reader.Node
	borrowed    map[*reader.Node]bool
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
		if err == nil {
			result, err = builder.resolveGraph(result)
		}
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
	kind := n.Datum().Kind()
	if kind != scheme.KindPair && kind != scheme.KindVector {
		return b.buildNode(n, path, disabled)
	}
	if b.memo == nil {
		b.memo = map[templateKey]*reader.Node{}
		b.aliases = map[*reader.Node]*reader.Node{}
	}
	key := templateKey{n.Datum(), fmt.Sprint(path), disabled}
	if previous := b.memo[key]; previous != nil {
		return previous, nil
	}
	placeholder := atom(scheme.Unspecified, b.source)
	b.memo[key] = placeholder
	result, err := b.buildNode(n, path, disabled)
	if err != nil {
		return nil, err
	}
	b.aliases[placeholder] = result
	return placeholder, nil
}
func (b *templateBuilder) buildNode(n *reader.Node, path []int, disabled bool) (*reader.Node, error) {
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
			if b.borrowed == nil {
				b.borrowed = map[*reader.Node]bool{}
			}
			b.borrowed[c.node] = true
			return c.node, nil
		}
		scopes := append(ident.Scopes(), b.mark)
		return &reader.Node{Syntax: scheme.NewSyntax(ident.Symbol(), b.source, scopes...)}, nil
	}
	kind := n.Datum().Kind()
	if kind != scheme.KindPair && kind != scheme.KindVector {
		return atom(n.Datum(), b.source), nil
	}
	if kind == scheme.KindPair {
		if n.Car == nil {
			return atom(n.Datum(), b.source), nil
		}
		if !disabled && t.isEllipsis(n.Car) {
			xs, err := nodes(n)
			if err != nil {
				return nil, err
			}
			if len(xs) != 2 {
				return nil, syntax(n, "ellipsis escape requires one template")
			}
			return b.build(xs[1], path, true)
		}
		repeat, rest, err := t.templateTail(n, disabled)
		if err != nil {
			return nil, err
		}
		expanded, err := b.repeated(n.Car, repeat, path, disabled)
		if err != nil {
			return nil, err
		}
		end, err := b.build(rest, path, disabled)
		if err != nil {
			return nil, err
		}
		return list(expanded, end, b.source), nil
	}
	var result []*reader.Node
	xs := n.Elements
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
	values := make([]scheme.Value, len(result))
	for i, x := range result {
		values[i] = x.Datum()
	}
	out := atom(scheme.NewVector(values...), b.source)
	out.Elements = result
	return out, nil
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
