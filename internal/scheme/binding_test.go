package scheme_test

import (
	"errors"
	"testing"

	"github.com/0x6d61/guri/internal/scheme"
)

func TestSymbolsScopesAndBindingCells(t *testing.T) {
	table := scheme.NewSymbolTable()
	x := table.Intern("x")
	if x != table.Intern("x") || x == table.Intern("X") || table.Intern("").Name() != "" {
		t.Fatal("symbol interning")
	}
	a, b := scheme.NewScope(), scheme.NewScope()
	original := scheme.NewIdentifier(x)
	scoped := original.WithScope(a).WithScope(b)
	if original.SameContext(scoped) || !scoped.SameContext(original.WithScope(b).WithScope(a)) || !scoped.SameContext(scoped.WithScope(a)) {
		t.Fatal("immutable scope sets")
	}
	scopes := scoped.Scopes()
	scopes[0] = scheme.NewScope()
	if !scoped.SameContext(original.WithScope(a).WithScope(b)) {
		t.Fatal("scope backing exposed")
	}
	cell := scheme.NewCell(scheme.NewInteger(1))
	binding := scheme.NewBinding(scoped, cell)
	alias := binding.Cell()
	alias.Set(scheme.True)
	value, err := cell.Get()
	if err != nil || value != scheme.True {
		t.Fatal("shared binding cell")
	}
	shadow := scheme.NewBinding(scoped, scheme.NewCell(scheme.False))
	if binding.ID() == shadow.ID() {
		t.Fatal("shadowed binding reused identity")
	}
	pending := scheme.NewUninitializedCell()
	if _, err := pending.Get(); !errors.Is(err, scheme.ErrUninitialized) {
		t.Fatalf("uninitialized binding: %v", err)
	}
	pending.Set(scheme.Null)
	value, err = pending.Get()
	if err != nil || value != scheme.Null {
		t.Fatal("null and uninitialized conflated")
	}
}

func TestSyntaxRetainsSourceAndIdentifierContext(t *testing.T) {
	table := scheme.NewSymbolTable()
	scope := scheme.NewScope()
	source := scheme.Source{Filename: "input.scm", Line: 3, Column: 7}
	scopes := []*scheme.Scope{scope}
	syntax := scheme.NewSyntax(table.Intern("x"), source, scopes...)
	scopes[0] = scheme.NewScope()
	id, ok := syntax.Identifier()
	if !ok || !id.SameContext(scheme.NewIdentifier(table.Intern("x")).WithScope(scope)) || syntax.Source() != source {
		t.Fatal("syntax source/context lost")
	}
	if _, ok := scheme.NewSyntax(scheme.NewInteger(1), source).Identifier(); ok {
		t.Fatal("number became identifier")
	}
}
