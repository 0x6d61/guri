package reader_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/0x6d61/guri/internal/conformance"
	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"golang.org/x/text/cases"
)

func read(t *testing.T, source string) []*reader.Node {
	t.Helper()
	nodes, err := reader.ReadAll("test.scm", source, scheme.NewSymbolTable())
	if err != nil {
		t.Fatal(err)
	}
	return nodes
}
func symbol(t *testing.T, n *reader.Node) string {
	t.Helper()
	s, ok := n.Datum().(*scheme.Symbol)
	if !ok {
		t.Fatalf("not a symbol: %T", n.Datum())
	}
	return s.Name()
}

func TestStandardAtomsAndLegacyNames(t *testing.T) {
	nodes := read(t, `#t #f #TRUE #False nil true false :key b"abc"`)
	for i, v := range []scheme.Value{scheme.True, scheme.False, scheme.True, scheme.False} {
		if nodes[i].Datum() != v {
			t.Fatalf("boolean %d", i)
		}
	}
	for i, name := range []string{"nil", "true", "false", ":key", "b"} {
		if symbol(t, nodes[i+4]) != name {
			t.Fatal("legacy special treatment")
		}
	}
	if nodes[9].Datum().(*scheme.String).Text() != "abc" {
		t.Fatal("b string became Bytes")
	}
}

func TestListsVectorsAndAbbreviations(t *testing.T) {
	nodes := read(t, "() (a b . c) #(x #t) #u8(0 #xFF #e2.0) 'x `(a ,b ,@c)")
	if nodes[0].Datum() != scheme.Null {
		t.Fatal("empty list")
	}
	pair := nodes[1].Datum().(*scheme.Pair)
	if pair.Car().(*scheme.Symbol).Name() != "a" || pair.Cdr().(*scheme.Pair).Cdr().(*scheme.Symbol).Name() != "c" {
		t.Fatal("improper list structure")
	}
	if symbol(t, nodes[1].Car) != "a" || symbol(t, nodes[1].Cdr.Cdr) != "c" {
		t.Fatal("syntax tree disagrees with datum")
	}
	if nodes[2].Datum().(*scheme.Vector).Len() != 2 || len(nodes[2].Elements) != 2 {
		t.Fatal("vector")
	}
	bytes := nodes[3].Datum().(*scheme.Bytevector).Bytes()
	if string(bytes) != string([]byte{0, 255, 2}) {
		t.Fatalf("bytevector: %v", bytes)
	}
	if symbol(t, nodes[4].Car) != "quote" || symbol(t, nodes[4].Cdr.Car) != "x" {
		t.Fatal("quote expansion")
	}
	qq := nodes[5]
	if symbol(t, qq.Car) != "quasiquote" {
		t.Fatal("quasiquote")
	}
	contents := qq.Cdr.Car
	if symbol(t, contents.Cdr.Car.Car) != "unquote" || symbol(t, contents.Cdr.Cdr.Car.Car) != "unquote-splicing" {
		t.Fatal("unquote expansion")
	}
}

func TestCommentsDirectivesAndFullCaseFolding(t *testing.T) {
	nodes := read(t, "#| outer #| nested |# ignored |# #;#;a b c\n#!fold-case Straße Σς #\\SPACE #\\A #!no-fold-case Mixed")
	if len(nodes) != 6 || symbol(t, nodes[0]) != "c" || symbol(t, nodes[1]) != "strasse" || symbol(t, nodes[2]) != "σσ" || symbol(t, nodes[5]) != "Mixed" {
		t.Fatal("comment or fold-case semantics")
	}
	if nodes[3].Datum().(scheme.Character).Rune() != ' ' || nodes[4].Datum().(scheme.Character).Rune() != 'A' {
		t.Fatal("character name versus literal folding")
	}
	nodes = read(t, "#;#!fold-case ABC DEF")
	if len(nodes) != 1 || symbol(t, nodes[0]) != "def" {
		t.Fatal("directive state lost in datum comment")
	}
}

func TestStringsCharactersAndEscapedIdentifiers(t *testing.T) {
	nodes := read(t, "\"a\\a\\b\\t\\n\\r\\\"\\\\\\|\\x3bb;\" |H\\x65;llo| || |a\\|b| #\\x1f600 #\\null #\\)")
	if nodes[0].Datum().(*scheme.String).Text() != "a\a\b\t\n\r\"\\|λ" {
		t.Fatal("string escapes")
	}
	if symbol(t, nodes[1]) != "Hello" || symbol(t, nodes[2]) != "" || symbol(t, nodes[3]) != "a|b" {
		t.Fatal("identifier escapes")
	}
	if nodes[4].Datum().(scheme.Character).Rune() != '😀' || nodes[5].Datum().(scheme.Character).Rune() != 0 || nodes[6].Datum().(scheme.Character).Rune() != ')' {
		t.Fatal("character syntax")
	}
	nodes = read(t, "\"a\\  \r\n  b\r\nc\rd\"")
	if nodes[0].Datum().(*scheme.String).Text() != "ab\nc\nd" {
		t.Fatal("line continuation/ending normalization")
	}
}

func TestIncrementalReadingSourcesAndMutableData(t *testing.T) {
	r, err := reader.New("input.scm", "; hi\r\n(日本 . 2) #!fold-case NAME", scheme.NewSymbolTable())
	if err != nil {
		t.Fatal(err)
	}
	node, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	if node.Source() != (scheme.Source{Filename: "input.scm", Line: 2, Column: 1}) || node.Car.Source().Column != 2 || node.Cdr.Source().Column != 7 {
		t.Fatalf("source positions: %+v %+v", node.Source(), node.Cdr.Source())
	}
	next, err := r.Read()
	if err != nil || symbol(t, next) != "name" {
		t.Fatal("incremental directive state")
	}
	if _, err = r.Read(); !errors.Is(err, io.EOF) {
		t.Fatal("EOF")
	}
	if _, err = r.Read(); !errors.Is(err, io.EOF) {
		t.Fatal("repeat EOF")
	}
	nodes := read(t, "(a) #(1) \"a\"")
	if err := nodes[0].Datum().(*scheme.Pair).SetCar(scheme.False); err != nil {
		t.Fatal("read data prematurely frozen")
	}
	if err := nodes[1].Datum().(*scheme.Vector).Set(0, scheme.False); err != nil {
		t.Fatal(err)
	}
	c, _ := scheme.NewCharacter('b')
	if err := nodes[2].Datum().(*scheme.String).Set(0, c); err != nil {
		t.Fatal(err)
	}
}

func TestReaderDiagnostics(t *testing.T) {
	for _, source := range []string{"[1]", "{}", "(", "(a .)", "(. a)", "(a . b c)", "#(a . b)", "#u8(256)", "#u8(-1)", "#u8(1.0)", "#truex", "12foo", "1/0", "#e#e1", "\"unterminated", "\"\\xD800;\"", "|unterminated", "#;", "#|unterminated", ".", "#\\Space", "abc'foo", "#!fold-caseX a"} {
		t.Run(source, func(t *testing.T) {
			_, err := reader.ReadAll("bad.scm", source, nil)
			var diagnostic *reader.Error
			if !errors.As(err, &diagnostic) || diagnostic.Position.Filename != "bad.scm" || diagnostic.Position.Line < 1 || diagnostic.Position.Column < 1 {
				t.Fatalf("missing diagnostic for %q: %v", source, err)
			}
		})
	}
	_, err := reader.ReadAll("utf8.scm", "; comment\r\n"+string([]byte{0xff}), nil)
	var diagnostic *reader.Error
	if !errors.As(err, &diagnostic) || diagnostic.Position.Line != 2 || diagnostic.Position.Column != 1 {
		t.Fatalf("invalid UTF-8 location: %v", err)
	}
	if nodes, err := reader.ReadAll("empty.scm", "; empty", nil); err != nil || len(nodes) != 0 {
		t.Fatal("empty source")
	}
}

func TestReadAllCheckedInR7RSCases(t *testing.T) {
	for _, path := range []string{"../../testdata/r7rs/suite.json", "../../testdata/r7rs/chibi/suite.json"} {
		suite, err := conformance.LoadSuite(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range suite.Cases {
			t.Run(c.ID, func(t *testing.T) {
				nodes, err := reader.ReadAll(c.File, c.Source, nil)
				if err != nil || len(nodes) == 0 {
					t.Fatalf("cannot read imported program: %v", err)
				}
			})
		}
	}
}

func TestIdentifierGrammar(t *testing.T) {
	source := "+ - ... +soup+ ->string .foo +.foo a@b λ 日本 |123|"
	nodes := read(t, source)
	if len(nodes) != len(strings.Fields(source)) {
		t.Fatal("identifier count")
	}
	for _, n := range nodes {
		if n.Datum().Kind() != scheme.KindSymbol {
			t.Fatalf("not identifier: %T", n.Datum())
		}
	}
}

func TestCaseFoldingDataMatchesGoUnicode(t *testing.T) {
	if cases.UnicodeVersion != unicode.Version {
		t.Fatalf("case folding Unicode %s differs from identifiers Unicode %s", cases.UnicodeVersion, unicode.Version)
	}
}
