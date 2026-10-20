package guri_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6d61/guri"
)

func TestV0001Programs(t *testing.T) {
	paths, err := filepath.Glob("testdata/v0.0.1/*.guri")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no acceptance programs")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(strings.TrimSuffix(path, ".guri") + ".out")
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			i := guri.NewInterpreter()
			i.Output = &out
			if _, err := i.Eval(path, string(source)); err != nil {
				t.Fatal(err)
			}
			if out.String() != string(want) {
				t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
			}
		})
	}
}

func TestV0001ErrorClasses(t *testing.T) {
	for _, tt := range []struct {
		source string
		kind   guri.ErrorKind
	}{
		{"missing", guri.NameError}, {"(set! missing 1)", guri.DefinitionError},
		{"(inc)", guri.ArityError}, {"(+ 1 true)", guri.TypeError},
		{"(nth [] 0)", guri.IndexError}, {"(/ 1 0)", guri.ArithmeticError},
	} {
		t.Run(string(tt.kind), func(t *testing.T) {
			_, err := guri.NewInterpreter().Eval("errors.guri", "\n "+tt.source)
			var e *guri.RuntimeError
			if !errors.As(err, &e) || e.Kind != tt.kind || e.Position.Line != 2 || e.Position.Column != 2 || e.Position.Filename != "errors.guri" || e.Cause == "" {
				t.Fatalf("unexpected diagnostic: %v", err)
			}
		})
	}
	_, err := guri.NewInterpreter().Eval("errors.guri", "\n [")
	var e *guri.ReaderError
	if !errors.As(err, &e) || e.Position.Line != 2 || e.Position.Column != 2 || e.Cause == "" {
		t.Fatalf("unexpected reader diagnostic: %v", err)
	}
}

func TestReduceRejectsNonCallableOnEmptyVector(t *testing.T) {
	_, err := guri.NewInterpreter().Eval("reduce.guri", "(reduce 42 0 [])")
	var e *guri.RuntimeError
	if !errors.As(err, &e) || e.Kind != guri.TypeError {
		t.Fatalf("got %v, want TypeError", err)
	}
}
