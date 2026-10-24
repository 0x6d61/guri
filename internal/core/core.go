// Package core resolves Scheme core bindings and compiles them to VM bytecode.
// NewCore is an internal bootstrap environment, not an imported Scheme library.
package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

var ErrSyntax = errors.New("invalid core syntax")

type Error struct {
	Source scheme.Source
	Cause  error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: SyntaxError: %v", e.Source.Filename, e.Source.Line, e.Source.Column, e.Cause)
}
func (e *Error) Unwrap() error { return e.Cause }

type form uint8

const (
	variable form = iota
	quote
	lambda
	conditional
	assignment
	definition
	begin
	let
	letStar
	letRec
	letRecStar
	defineSyntax
	letSyntax
	letRecSyntax
	syntaxRules
	syntaxError
	auxiliary
	macroCall
	derived
	quasiquote
	unquote
	splice
	include
	includeCI
	condExpand
	defineValues
	letValues
	letStarValues
	parameterize
	guard
	defineRecordType
)

type binding struct {
	transformer *transformer
	declared    bool
	id          scheme.Identifier
	form        form
	cell        *scheme.Cell
	slot        int
}
type scope struct {
	parent   *scope
	bindings []*binding
	slots    int
}

func (s *scope) add(id scheme.Identifier) (*binding, error) {
	for _, b := range s.bindings {
		if b.id.SameContext(id) {
			return nil, ErrSyntax
		}
	}
	b := &binding{id: id, slot: s.slots, declared: true}
	s.slots++
	s.bindings = append(s.bindings, b)
	return b, nil
}

type Engine struct {
	symbols    *scheme.SymbolTable
	globals    []*binding
	controls   map[string]*scheme.Procedure
	primitives []*binding
	loader     SourceLoader
	features   map[string]bool
	libraries  func([]scheme.Value) bool
	machine    *vm.Machine
}

func NewCore() *Engine {
	e := &Engine{symbols: scheme.NewSymbolTable(), machine: vm.New()}
	for name, f := range map[string]form{
		"parameterize": parameterize, "guard": guard,
		"define-values": defineValues, "let-values": letValues, "let*-values": letStarValues,
		"quote":         quote,
		"lambda":        lambda,
		"if":            conditional,
		"set!":          assignment,
		"define":        definition,
		"begin":         begin,
		"let":           let,
		"let*":          letStar,
		"letrec":        letRec,
		"letrec*":       letRecStar,
		"define-syntax": defineSyntax,
		"let-syntax":    letSyntax,
		"letrec-syntax": letRecSyntax,
		"syntax-rules":  syntaxRules,
		"syntax-error":  syntaxError,
		"_":             auxiliary,
		"...":           auxiliary,
		"else":          auxiliary, "=>": auxiliary,
		"cond": derived, "case": derived, "and": derived, "or": derived, "when": derived, "unless": derived, "do": derived, "case-lambda": derived,
		"quasiquote": quasiquote, "unquote": unquote, "unquote-splicing": splice,
		"include": include, "include-ci": includeCI, "cond-expand": condExpand,
		"define-record-type": defineRecordType,
	} {
		e.globals = append(e.globals, &binding{id: scheme.NewIdentifier(e.symbols.Intern(name)), form: f, declared: true})
	}
	e.installControl()
	e.installNumbers()
	e.installPairs()
	e.installText()
	e.installVectors()
	e.installPorts()
	e.primitives = append([]*binding(nil), e.globals...)
	return e
}
func (e *Engine) global(id scheme.Identifier) *binding {
	for _, b := range e.globals {
		if b.id.SameContext(id) {
			return b
		}
	}
	b := &binding{id: id, cell: scheme.NewUninitializedCell()}
	e.globals = append(e.globals, b)
	return b
}

// Bind supplies or replaces a value from the embedding/library layer.
func (e *Engine) Bind(name string, v scheme.Value) {
	b := e.variableBinding(scheme.NewIdentifier(e.symbols.Intern(name)))
	b.cell.Set(v)
}
func (e *Engine) Stats() vm.Stats { return e.machine.Stats() }
func (e *Engine) Eval(ctx context.Context, filename, source string) (scheme.Results, error) {
	nodes, err := reader.ReadAll(filename, source, e.symbols)
	if err != nil {
		return scheme.Results{}, err
	}

	b := &builder{engine: e, env: &scope{}, ctx: ctx}
	pos := scheme.Source{Filename: filename, Line: 1, Column: 1}
	if err := b.sequence(nodes, true, true, pos); err != nil {
		return scheme.Results{}, err
	}
	b.emit(vm.ReturnOp, 0, 0, pos)
	code, err := b.finish()
	if err != nil {
		return scheme.Results{}, err
	}
	return e.machine.Run(ctx, vm.Closure(code))
}

// Compile expands and compiles a position-bearing datum. The VM Code
// contract remains independent of reader Nodes and macro syntax.
func (e *Engine) Compile(n *reader.Node) (*vm.Code, error) {
	b := &builder{engine: e, env: &scope{}, ctx: context.Background()}
	if err := b.expr(n, true, true); err != nil {
		return nil, err
	}
	b.emit(vm.ReturnOp, 0, 0, n.Source())
	return b.finish()
}

type builder struct {
	ctx    context.Context
	engine *Engine
	env    *scope
	spec   vm.Spec
}

func (b *builder) emit(op vm.Op, a, c int, pos scheme.Source) int {
	index := len(b.spec.Instructions)
	b.spec.Instructions = append(b.spec.Instructions, vm.Instruction{Op: op, A: a, B: c, Source: pos})
	return index
}
func (b *builder) constant(v scheme.Value, pos scheme.Source) {
	index := len(b.spec.Constants)
	b.spec.Constants = append(b.spec.Constants, v)
	b.emit(vm.Const, index, 0, pos)
}
func (b *builder) finish() (*vm.Code, error) { b.spec.Slots = b.env.slots; return vm.NewCode(b.spec) }
func (b *builder) resolve(id scheme.Identifier) (*binding, int) {
	target := b.engine.find(b.env, b.engine.globals, id)
	if target == nil {
		target = b.engine.global(id)
	}
	depth := 0
	for s := b.env; s != nil; s = s.parent {
		for _, bind := range s.bindings {
			if bind == target {
				return target, depth
			}
		}
		depth++
	}
	return target, -1
}

func (b *builder) globalIndex(cell *scheme.Cell) int {
	for i, c := range b.spec.Globals {
		if c == cell {
			return i
		}
	}
	b.spec.Globals = append(b.spec.Globals, cell)
	return len(b.spec.Globals) - 1
}
func (b *builder) access(bind *binding, depth int, local, global vm.Op, pos scheme.Source) {
	if depth < 0 {
		b.emit(global, b.globalIndex(bind.cell), 0, pos)
	} else {
		b.emit(local, depth, bind.slot, pos)
	}
}
func syntax(n *reader.Node, message string) error {
	return &Error{n.Source(), fmt.Errorf("%w: %s", ErrSyntax, message)}
}
func id(n *reader.Node) (scheme.Identifier, error) {
	if n == nil {
		return scheme.Identifier{}, ErrSyntax
	}
	x, ok := n.Syntax.Identifier()
	if !ok {
		return x, syntax(n, "expected identifier")
	}
	return x, nil
}
func nodes(n *reader.Node) ([]*reader.Node, error) {
	var result []*reader.Node
	seen := map[scheme.Value]bool{}
	for n.Datum() != scheme.Null {
		if seen[n.Datum()] {
			return nil, syntax(n, "circular syntax list")
		}
		seen[n.Datum()] = true
		if _, ok := n.Datum().(*scheme.Pair); !ok || n.Car == nil || n.Cdr == nil {
			return nil, syntax(n, "expected proper list")
		}
		result = append(result, n.Car)
		n = n.Cdr
	}
	return result, nil
}
func (b *builder) headForm(n *reader.Node) form {
	if _, ok := n.Datum().(*scheme.Pair); !ok || n.Car == nil {
		return variable
	}
	i, ok := n.Car.Syntax.Identifier()
	if !ok {
		return variable
	}
	bind, _ := b.resolve(i)
	return bind.form
}
func (b *builder) expr(n *reader.Node, tail, top bool) error {
	var err error
	n, err = b.expandHead(n)
	if err != nil {
		return err
	}
	pos := n.Source()
	if ident, ok := n.Syntax.Identifier(); ok {
		bind, depth := b.resolve(ident)
		if bind.form != variable {
			return syntax(n, "syntax keyword used as a value")
		}
		b.access(bind, depth, vm.Local, vm.Global, pos)
		return nil
	}
	if n.Datum() == scheme.Null {
		return syntax(n, "empty list is not an expression")
	}
	if _, ok := n.Datum().(*scheme.Pair); !ok {
		scheme.FreezeLiteral(n.Datum())
		b.constant(n.Datum(), pos)
		return nil
	}
	parts, err := nodes(n)
	if err != nil {
		return err
	}
	args := parts[1:]
	arity := func(min, max int) error {
		if len(args) < min || max >= 0 && len(args) > max {
			return syntax(n, "incorrect syntax argument count")
		}
		return nil
	}
	switch b.headForm(n) {
	case parameterize:
		node, err := b.lowerParameterize(n, args)
		if err != nil {
			return err
		}
		return b.expr(node, tail, top)
	case guard:
		node, err := b.lowerGuard(n, args)
		if err != nil {
			return err
		}
		return b.expr(node, tail, top)
	case letValues, letStarValues:
		node, err := b.lowerValueBindings(n, args, b.headForm(n) == letStarValues)
		if err != nil {
			return err
		}
		return b.expr(node, tail, top)
	case derived:
		expanded, err := b.lowerDerived(n, args)
		if err != nil {
			return err
		}
		return b.expr(expanded, tail, top)
	case quasiquote:
		if err := arity(1, 1); err != nil {
			return err
		}
		if err := b.checkGraph(args[0], false); err != nil {
			return err
		}
		expanded, _, err := b.lowerQuasiquote(args[0], 1, false)
		if err != nil {
			return err
		}
		return b.expr(expanded, tail, false)
	case unquote, splice:
		return syntax(n, "unquote outside quasiquote")
	case quote:
		if err := arity(1, 1); err != nil {
			return err
		}
		scheme.FreezeLiteral(args[0].Datum())
		b.constant(args[0].Datum(), pos)
	case lambda:
		if err := arity(2, -1); err != nil {
			return err
		}
		return b.function(args[0], args[1:], pos)
	case conditional:
		if err := arity(2, 3); err != nil {
			return err
		}
		if err := b.expr(args[0], false, false); err != nil {
			return err
		}
		no := b.emit(vm.JumpFalse, 0, 0, pos)
		if err := b.expr(args[1], tail, false); err != nil {
			return err
		}
		done := b.emit(vm.Jump, 0, 0, pos)
		b.spec.Instructions[no].A = len(b.spec.Instructions)
		if len(args) == 3 {
			if err := b.expr(args[2], tail, false); err != nil {
				return err
			}
		} else {
			b.constant(scheme.Unspecified, pos)
		}
		b.spec.Instructions[done].A = len(b.spec.Instructions)
	case assignment:
		if err := arity(2, 2); err != nil {
			return err
		}
		ident, err := id(args[0])
		if err != nil {
			return err
		}
		bind, depth := b.resolve(ident)
		if bind.form != variable {
			return syntax(args[0], "cannot assign a syntax binding")
		}
		if err := b.expr(args[1], false, false); err != nil {
			return err
		}
		b.access(bind, depth, vm.SetLocal, vm.SetGlobal, pos)
	case definition:
		if !top {
			return syntax(n, "definition outside a definition region")
		}
		d, err := parseDefinition(n)
		if err != nil {
			return err
		}
		ident, err := id(d.name)
		if err != nil {
			return err
		}
		bind := b.engine.variableBinding(ident)
		if err := b.definitionValue(d); err != nil {
			return err
		}
		b.access(bind, -1, vm.InitLocal, vm.InitGlobal, pos)
	case defineSyntax:
		if !top {
			return syntax(n, "syntax definition outside a definition region")
		}
		if err := b.defineTransformer(n, nil); err != nil {
			return err
		}
		b.constant(scheme.Unspecified, pos)
	case letSyntax, letRecSyntax:
		return b.localTransformers(n, args, b.headForm(n) == letRecSyntax, tail)
	case syntaxRules, auxiliary:
		return syntax(n, "transformer/auxiliary syntax used as an expression")
	case syntaxError:
		if len(args) < 1 {
			return syntax(n, "syntax-error requires a message")
		}
		message, ok := args[0].Datum().(*scheme.String)
		if !ok {
			return syntax(args[0], "syntax-error message must be a string literal")
		}
		return syntaxFailure(n, args, message.Text())
	case begin:
		if len(args) == 0 && !top {
			return syntax(n, "expression begin requires an expression")
		}
		return b.sequence(args, tail, top, pos)
	case let, letStar, letRec, letRecStar:
		return b.bindingForm(n, args, b.headForm(n), tail)
	default:
		for _, p := range parts {
			if err := b.expr(p, false, false); err != nil {
				return err
			}
			b.emit(vm.Push, 0, 0, p.Source())
		}
		op := vm.Call
		if tail {
			op = vm.TailCall
		}
		b.emit(op, len(args), 0, pos)
	}
	return nil
}
func (b *builder) sequence(xs []*reader.Node, tail, top bool, pos scheme.Source) error {
	if len(xs) == 0 {
		b.constant(scheme.Unspecified, pos)
		return nil
	}
	for i, n := range xs {
		if err := b.expr(n, tail && i == len(xs)-1, top); err != nil {
			return err
		}
	}
	return nil
}
func (b *builder) child(env *scope, required int, rest bool) *builder {
	return &builder{engine: b.engine, env: env, ctx: b.ctx, spec: vm.Spec{Required: required, Rest: rest}}
}
func (b *builder) closure(child *builder, pos scheme.Source) error {
	child.emit(vm.ReturnOp, 0, 0, pos)
	c, err := child.finish()
	if err != nil {
		return err
	}
	index := len(b.spec.Functions)
	b.spec.Functions = append(b.spec.Functions, c)
	b.emit(vm.MakeClosure, index, 0, pos)
	return nil
}
func (b *builder) function(formals *reader.Node, body []*reader.Node, pos scheme.Source) error {
	env := &scope{parent: b.env}
	required := 0
	rest := false
	n := formals
	for n.Datum() != scheme.Null {
		if _, ok := n.Datum().(*scheme.Symbol); ok {
			ident, _ := id(n)
			if _, err := env.add(ident); err != nil {
				return syntax(n, "duplicate formal")
			}
			rest = true
			break
		}
		if _, ok := n.Datum().(*scheme.Pair); !ok || n.Car == nil || n.Cdr == nil {
			return syntax(n, "invalid formals")
		}
		ident, err := id(n.Car)
		if err != nil {
			return err
		}
		if _, err = env.add(ident); err != nil {
			return syntax(n.Car, "duplicate formal")
		}
		required++
		n = n.Cdr
	}
	child := b.child(env, required, rest)
	if err := child.body(body, pos); err != nil {
		return err
	}
	return b.closure(child, pos)
}

type def struct {
	name, expr, formals *reader.Node
	body                []*reader.Node
}

func parseDefinition(n *reader.Node) (def, error) {
	xs, err := nodes(n)
	if err != nil {
		return def{}, err
	}
	if len(xs) < 3 {
		return def{}, syntax(n, "incomplete definition")
	}
	target := xs[1]
	if _, ok := target.Datum().(*scheme.Pair); ok {
		if target.Car == nil || target.Cdr == nil {
			return def{}, syntax(n, "invalid function definition")
		}
		if _, err := id(target.Car); err != nil {
			return def{}, err
		}
		return def{name: target.Car, formals: target.Cdr, body: xs[2:]}, nil
	}
	if _, err := id(target); err != nil {
		return def{}, err
	}
	if len(xs) != 3 {
		return def{}, syntax(n, "definition requires one initializer")
	}
	return def{name: target, expr: xs[2]}, nil
}
func (b *builder) definitionValue(d def) error {
	if d.formals != nil {
		return b.function(d.formals, d.body, d.name.Source())
	}
	return b.expr(d.expr, false, false)
}

// body gives internal definitions their own letrec* region inside the formal
// parameter region. Only leading begins are spliced during definition discovery.
func (b *builder) body(body []*reader.Node, pos scheme.Source) error {
	inner := b.child(&scope{parent: b.env}, 0, false)
	pending := append([]*reader.Node(nil), body...)
	var defs []def
	var binds []*binding
	var consumed []scheme.Identifier
	hasSyntax := false
	for len(pending) > 0 {
		n, err := inner.expandHead(pending[0])
		if err != nil {
			return err
		}
		pending[0] = n
		switch inner.headForm(n) {
		case begin:
			key, _ := id(n.Car)
			consumed = append(consumed, key)
			xs, err := nodes(n)
			if err != nil {
				return err
			}
			pending = append(append([]*reader.Node(nil), xs[1:]...), pending[1:]...)
		case defineSyntax:
			if err := inner.defineTransformer(n, inner.env); err != nil {
				return err
			}
			hasSyntax = true
			pending = pending[1:]
		case definition:
			key, _ := id(n.Car)
			consumed = append(consumed, key)
			d, err := parseDefinition(n)
			if err != nil {
				return err
			}
			ident, _ := id(d.name)
			for _, key := range consumed {
				if ident.SameContext(key) {
					return syntax(d.name, "definition changes syntax used to discover its region")
				}
			}
			bind, err := inner.env.add(ident)
			if err != nil {
				return syntax(d.name, "duplicate internal definition")
			}
			defs = append(defs, d)
			binds = append(binds, bind)
			pending = pending[1:]
		default:
			if len(defs) == 0 && !hasSyntax {
				return b.sequence(pending, true, false, pos)
			}
			for i, d := range defs {
				if err := inner.definitionValue(d); err != nil {
					return err
				}
				inner.emit(vm.InitLocal, 0, binds[i].slot, d.name.Source())
			}
			if err := inner.sequence(pending, true, false, pos); err != nil {
				return err
			}
			if err := b.closure(inner, pos); err != nil {
				return err
			}
			b.emit(vm.Push, 0, 0, pos)
			b.emit(vm.TailCall, 0, 0, pos)
			return nil
		}
	}
	return &Error{pos, fmt.Errorf("%w: body requires an expression", ErrSyntax)}
}
