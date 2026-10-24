package core

import (
	"bytes"
	numeric "github.com/0x6d61/guri/internal/number"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

type patternKind uint8

const (
	pVariable patternKind = iota
	pWildcard
	pLiteral
	pConstant
	pList
	pVector
)

type pattern struct {
	kind     patternKind
	variable int
	literal  scheme.Identifier
	datum    scheme.Value
	elements []patternElement
	tail     *pattern
	vars     []int
}
type patternElement struct {
	pattern *pattern
	repeat  bool
}
type patternVariable struct {
	id    scheme.Identifier
	depth int
}
type rule struct {
	pattern   *pattern
	template  *reader.Node
	variables []patternVariable
}
type transformer struct {
	context  *definitionContext
	source   scheme.Source
	ellipsis scheme.Identifier
	literals []scheme.Identifier
	rules    []rule
}

func (t *transformer) sameIdentifier(a, b scheme.Identifier) bool {
	if a.SameContext(b) {
		return true
	}
	if a.Symbol().Name() != b.Symbol().Name() || t.context == nil {
		return false
	}
	x := t.context.engine.find(t.context.env, t.context.globals, a)
	y := t.context.engine.find(t.context.env, t.context.globals, b)
	if x != nil && !x.declared {
		x = nil
	}
	if y != nil && !y.declared {
		y = nil
	}
	return x == y
}
func (t *transformer) literal(id scheme.Identifier) bool {
	for _, l := range t.literals {
		if t.sameIdentifier(id, l) {
			return true
		}
	}
	return false
}
func (t *transformer) isEllipsis(n *reader.Node) bool {
	ident, ok := n.Syntax.Identifier()
	return ok && t.sameIdentifier(ident, t.ellipsis) && !t.literal(ident)
}
func (t *transformer) varIndex(vars []patternVariable, id scheme.Identifier) int {
	for i, v := range vars {
		if v.id.SameContext(id) {
			return i
		}
	}
	return -1
}
func (b *builder) parseTransformer(n *reader.Node) (*transformer, error) {
	if b.headForm(n) != syntaxRules {
		return nil, syntax(n, "expected syntax-rules transformer")
	}
	xs, err := nodes(n)
	if err != nil {
		return nil, err
	}
	if len(xs) < 2 {
		return nil, syntax(n, "syntax-rules requires literals")
	}
	t := &transformer{source: n.Source(), ellipsis: scheme.NewIdentifier(b.engine.symbols.Intern("...")), context: &definitionContext{engine: b.engine, env: b.env, source: n.Source(), globals: append([]*binding(nil), b.engine.globals...)}}
	offset := 1
	if _, ok := xs[1].Datum().(*scheme.Symbol); ok {
		t.ellipsis, _ = id(xs[1])
		offset++
	}
	if len(xs) <= offset {
		return nil, syntax(n, "missing literal list")
	}
	lits, err := nodes(xs[offset])
	if err != nil {
		return nil, err
	}
	for _, node := range lits {
		ident, err := id(node)
		if err != nil {
			return nil, err
		}
		t.literals = append(t.literals, ident)
		if b.engine.find(b.env, b.engine.globals, ident) == nil {
			b.engine.global(ident)
		}
	}
	for _, node := range xs[offset+1:] {
		parts, err := nodes(node)
		if err != nil {
			return nil, err
		}
		if len(parts) != 2 {
			return nil, syntax(node, "rule requires pattern and template")
		}
		pat := parts[0]
		if pat.Datum().Kind() != scheme.KindPair || pat.Car == nil || pat.Cdr == nil {
			return nil, syntax(pat, "rule pattern requires a keyword")
		}
		if _, err := id(pat.Car); err != nil {
			return nil, err
		}
		if err := b.checkGraph(pat, false); err != nil {
			return nil, err
		}
		r := rule{template: parts[1]}
		r.pattern, err = t.compilePattern(pat.Cdr, 0, &r.variables)
		if err != nil {
			return nil, err
		}
		if err := t.validateTemplate(r.template, r.variables, 0, false, b); err != nil {
			return nil, err
		}
		t.rules = append(t.rules, r)
	}
	t.context = &definitionContext{engine: b.engine, env: b.env, source: n.Source(), globals: append([]*binding(nil), b.engine.globals...)}
	return t, nil
}
func (t *transformer) compilePattern(n *reader.Node, depth int, vars *[]patternVariable) (*pattern, error) {
	if ident, ok := n.Syntax.Identifier(); ok {
		if t.literal(ident) {
			return &pattern{kind: pLiteral, literal: ident}, nil
		}
		if ident.Symbol().Name() == "_" {
			return &pattern{kind: pWildcard}, nil
		}
		if t.isEllipsis(n) {
			return nil, syntax(n, "unexpected pattern ellipsis")
		}
		if t.varIndex(*vars, ident) >= 0 {
			return nil, syntax(n, "duplicate pattern variable")
		}
		index := len(*vars)
		*vars = append(*vars, patternVariable{ident, depth})
		return &pattern{kind: pVariable, variable: index, vars: []int{index}}, nil
	}
	p := &pattern{kind: pConstant, datum: n.Datum()}
	var xs []*reader.Node
	switch n.Datum().Kind() {
	case scheme.KindPair, scheme.KindNull:
		p.kind = pList
		var tails []*reader.Node
		xs, tails = split(n)
		tail, err := t.compileTail(tails[len(tails)-1], depth, vars)
		if err != nil {
			return nil, err
		}
		p.tail = tail
	case scheme.KindVector:
		p.kind = pVector
		xs = n.Elements
	default:
		return p, nil
	}
	repeated := false
	for i := 0; i < len(xs); i++ {
		rep := i+1 < len(xs) && t.isEllipsis(xs[i+1])
		d := depth
		if rep {
			if repeated {
				return nil, syntax(xs[i+1], "multiple repetitions at the same pattern list level")
			}
			repeated = true
			d++
		}
		child, err := t.compilePattern(xs[i], d, vars)
		if err != nil {
			return nil, err
		}
		p.elements = append(p.elements, patternElement{child, rep})
		p.vars = append(p.vars, child.vars...)
		if rep {
			i++
		}
	}
	if p.tail != nil {
		p.vars = append(p.vars, p.tail.vars...)
	}
	return p, nil
}
func (t *transformer) compileTail(n *reader.Node, depth int, vars *[]patternVariable) (*pattern, error) {
	if n.Datum() == scheme.Null {
		return &pattern{kind: pConstant, datum: scheme.Null}, nil
	}
	return t.compilePattern(n, depth, vars)
}

type capture struct {
	node     *reader.Node
	children []*capture
}
type captures map[int]*capture

func merge(a, b captures) captures {
	r := make(captures, len(a)+len(b))
	for k, v := range a {
		r[k] = v
	}
	for k, v := range b {
		r[k] = v
	}
	return r
}

type matcher struct {
	use         *builder
	transformer *transformer
	err         error
}

func (m *matcher) match(p *pattern, n *reader.Node) (captures, bool) {
	if err := m.use.ctx.Err(); err != nil {
		m.err = err
		return nil, false
	}
	switch p.kind {
	case pVariable:
		return captures{p.variable: &capture{node: n}}, true
	case pWildcard:
		return captures{}, true
	case pLiteral:
		ident, ok := n.Syntax.Identifier()
		if !ok {
			return nil, false
		}
		t := m.transformer
		a := t.context.engine.find(t.context.env, t.context.globals, p.literal)
		c, _ := m.use.resolve(ident)
		if a != nil && !a.declared {
			a = nil
		}
		if c != nil && !c.declared {
			c = nil
		}
		return captures{}, a == c && (a != nil || ident.Symbol().Name() == p.literal.Symbol().Name())
	case pConstant:
		return captures{}, datumEqual(p.datum, n.Datum())
	case pList:
		if n.Datum().Kind() != scheme.KindPair && n.Datum() != scheme.Null {
			return nil, false
		}
		xs, tails := split(n)
		for len(xs) < len(p.elements) {
			tail := tails[len(tails)-1]
			if tail.Car == nil || tail.Cdr == nil {
				break
			}
			xs = append(xs, tail.Car)
			tails = append(tails, tail.Cdr)
		}
		return m.sequence(p, xs, tails, 0, 0)
	case pVector:
		if n.Datum().Kind() != scheme.KindVector {
			return nil, false
		}
		xs := n.Elements
		tails := make([]*reader.Node, len(xs)+1)
		for i := range tails {
			tails[i] = atom(scheme.Null, n.Source())
		}
		return m.sequence(p, xs, tails, 0, 0)
	}
	return nil, false
}
func (m *matcher) sequence(p *pattern, xs, tails []*reader.Node, element, index int) (captures, bool) {
	if element == len(p.elements) {
		if p.kind == pVector {
			if index != len(xs) {
				return nil, false
			}
			return captures{}, true
		}
		return m.match(p.tail, tails[index])
	}
	seg := p.elements[element]
	if !seg.repeat {
		if index >= len(xs) {
			return nil, false
		}
		left, ok := m.match(seg.pattern, xs[index])
		if !ok {
			return nil, false
		}
		right, ok := m.sequence(p, xs, tails, element+1, index+1)
		if !ok {
			return nil, false
		}
		return merge(left, right), true
	}
	for count := len(xs) - index; count >= 0; count-- {
		if m.err != nil {
			return nil, false
		}
		repeated := captures{}
		for _, v := range seg.pattern.vars {
			repeated[v] = &capture{children: make([]*capture, 0, count)}
		}
		good := true
		for j := 0; j < count; j++ {
			one, ok := m.match(seg.pattern, xs[index+j])
			if !ok {
				good = false
				break
			}
			for _, v := range seg.pattern.vars {
				repeated[v].children = append(repeated[v].children, one[v])
			}
		}
		if !good {
			continue
		}
		rest, ok := m.sequence(p, xs, tails, element+1, index+count)
		if ok {
			return merge(repeated, rest), true
		}
	}
	return nil, false
}
func datumEqual(a, b scheme.Value) bool {
	if a.Kind() != b.Kind() {
		return false
	}
	switch x := a.(type) {
	case scheme.Number:
		return numeric.Equivalent(x, b.(scheme.Number))
	case *scheme.String:
		return x.Text() == b.(*scheme.String).Text()
	case *scheme.Bytevector:
		return bytes.Equal(x.Bytes(), b.(*scheme.Bytevector).Bytes())
	case scheme.Character:
		return x.Rune() == b.(scheme.Character).Rune()
	default:
		return a == b
	}
}
