package guri

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestReadAtoms(t *testing.T) {
	tests := []struct {
		source string
		kind   Kind
		value  any
	}{
		{"nil", Nil, nil}, {"true", Boolean, true}, {"false", Boolean, false},
		{"42", Integer, int64(42)}, {"-3", Integer, int64(-3)},
		{"9223372036854775807", Integer, int64(9223372036854775807)},
		{"-9223372036854775808", Integer, int64(-9223372036854775808)},
		{"3.14", Float, 3.14}, {"-0.5", Float, -0.5}, {"1e3", Float, float64(1000)},
		{`"hello 世界"`, String, "hello 世界"},
		{`"\"\\\n\r\t"`, String, "\"\\\n\r\t"},
		{"name", Symbol, "name"}, {"名前", Symbol, "名前"},
		{"+", Symbol, "+"}, {"-", Symbol, "-"}, {"set!", Symbol, "set!"},
		{":name", Keyword, "name"}, {":名前", Keyword, "名前"},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			forms, err := Read("atoms.guri", tt.source)
			if err != nil {
				t.Fatal(err)
			}
			if len(forms) != 1 {
				t.Fatalf("got %d forms, want 1", len(forms))
			}
			if forms[0].Kind != tt.kind || !reflect.DeepEqual(forms[0].Value, tt.value) {
				t.Fatalf("got %#v, want %s(%#v)", forms[0], tt.kind, tt.value)
			}
		})
	}
}

func TestReadCollections(t *testing.T) {
	forms, err := Read("collections.guri", `(f [1 (g)] {:x 2 :y 3}) () [] {}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 4 {
		t.Fatalf("got %d forms, want 4", len(forms))
	}
	call := forms[0]
	if call.Kind != List || len(call.Elements) != 3 {
		t.Fatalf("unexpected call: %#v", call)
	}
	if call.Elements[0].Kind != Symbol || call.Elements[0].Value != "f" {
		t.Fatalf("unexpected callee: %#v", call.Elements[0])
	}
	v := call.Elements[1]
	if v.Kind != Vector || len(v.Elements) != 2 || v.Elements[1].Kind != List {
		t.Fatalf("unexpected vector: %#v", v)
	}
	m := call.Elements[2]
	if m.Kind != Map || len(m.Elements) != 4 || m.Elements[0].Value != "x" || m.Elements[2].Value != "y" {
		t.Fatalf("map source order not preserved: %#v", m)
	}
	for i, kind := range []Kind{List, Vector, Map} {
		if forms[i+1].Kind != kind || len(forms[i+1].Elements) != 0 {
			t.Fatalf("unexpected empty %s: %#v", kind, forms[i+1])
		}
	}
}

func TestReadQuote(t *testing.T) {
	forms, err := Read("quote.guri", `'(a b) ''x '[a] '{:a b}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 4 {
		t.Fatalf("got %d forms, want 4", len(forms))
	}
	for i, kind := range []Kind{List, List, Vector, Map} {
		f := forms[i]
		if f.Kind != List || len(f.Elements) != 2 || f.Elements[0].Kind != Symbol || f.Elements[0].Value != "quote" || f.Elements[1].Kind != kind {
			t.Fatalf("unexpected quote expansion: %#v", f)
		}
	}
	inner := forms[1].Elements[1]
	if len(inner.Elements) != 2 || inner.Elements[0].Value != "quote" || inner.Elements[1].Value != "x" {
		t.Fatalf("unexpected nested quote: %#v", inner)
	}
}

func TestReadCommentsAndPositions(t *testing.T) {
	forms, err := Read("位置.guri", "; comment\r\n (名前\t\";文字\") ; end\n :次")
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 2 || len(forms[0].Elements) != 2 {
		t.Fatalf("unexpected forms: %#v", forms)
	}
	for _, check := range []struct {
		expr Expr
		line int
		col  int
	}{
		{forms[0], 2, 2}, {forms[0].Elements[0], 2, 3},
		{forms[0].Elements[1], 2, 6}, {forms[1], 3, 2},
	} {
		want := Position{"位置.guri", check.line, check.col}
		if check.expr.Position != want {
			t.Errorf("got position %#v, want %#v", check.expr.Position, want)
		}
	}
	if forms[0].Elements[1].Value != ";文字" {
		t.Fatal("semicolon in string treated as comment")
	}
}

func TestReadEmpty(t *testing.T) {
	for _, source := range []string{"", " \t\r\n", "; comment", "; one\n; two"} {
		forms, err := Read("empty.guri", source)
		if err != nil || len(forms) != 0 {
			t.Fatalf("Read(%q) = %#v, %v", source, forms, err)
		}
	}
}

func TestReadErrors(t *testing.T) {
	tests := []struct {
		source string
		line   int
		col    int
	}{
		{")", 1, 1}, {"]", 1, 1}, {"}", 1, 1},
		{"(]", 1, 2}, {"[)", 1, 2}, {"{)", 1, 2},
		{"(", 1, 1}, {"[1", 1, 1}, {"{:a 1", 1, 1},
		{"{:a}", 1, 1}, {"{:a 1 :b}", 1, 1},
		{`"open`, 1, 1}, {"\"trailing\\", 1, 1},
		{`"\q"`, 1, 2}, {"'", 1, 1}, {"' ;comment", 1, 1},
		{"' )", 1, 3}, {":", 1, 1},
		{"9223372036854775808", 1, 1}, {"-9223372036854775809", 1, 1},
		{"1e309", 1, 1}, {"\n [1 }", 2, 5},
		{"\xff", 1, 1}, {"\"a\xff\"", 1, 3}, {";\xff", 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			_, err := Read("bad.guri", tt.source)
			var readerErr *ReaderError
			if !errors.As(err, &readerErr) {
				t.Fatalf("got %v, want ReaderError", err)
			}
			want := Position{"bad.guri", tt.line, tt.col}
			if readerErr.Position != want || readerErr.Cause == "" {
				t.Fatalf("got %#v, want position %#v and cause", readerErr, want)
			}
			if !strings.Contains(err.Error(), "bad.guri:") || !strings.Contains(err.Error(), "ReaderError") {
				t.Fatalf("error missing context: %v", err)
			}
		})
	}
}
