package core

import (
	"testing"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestImportSetAlgebra(t *testing.T) {
	a := &binding{cell: scheme.NewCell(scheme.NewInteger(1))}
	b := &binding{cell: scheme.NewCell(scheme.NewInteger(2))}
	source := libraryExports{"a": a, "b": b}
	resolve := func(n *reader.Node) (libraryExports, error) { return source, nil }
	for src, want := range map[string]libraryExports{
		`(example)`:                      {"a": a, "b": b},
		`(only (example) a)`:             {"a": a},
		`(except (example) a)`:           {"b": b},
		`(prefix (only (example) b) p:)`: {"p:b": b},
		`(rename (example) (a b) (b a))`: {"a": b, "b": a},
		`(only (example))`:               {},
	} {
		nodes, err := reader.ReadAll("imports.scm", src, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := resolveImportSet(nodes[0], resolve)
		if err != nil || len(got) != len(want) {
			t.Fatal(src, got, err)
		}
		for name, value := range want {
			if got[name] != value {
				t.Fatal(src, name)
			}
		}
	}
	if source["a"] != a || source["b"] != b {
		t.Fatal("import transformation mutated source exports")
	}
	for _, src := range []string{`(only (example) missing)`, `(except (example) missing)`, `(rename (example) (missing x))`, `(rename (example) (a b))`, `(prefix (example) 1)`, `(prefix (example))`, `(rename (example) (a))`, `(only (example) 1)`} {
		ns, err := reader.ReadAll("bad.scm", src, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := resolveImportSet(ns[0], resolve); err == nil {
			t.Error(src)
		}
	}
}
func TestLibraryNameIdentityAndValidation(t *testing.T) {
	seen := map[string]bool{}
	for _, src := range []string{`(a b)`, `(a 1)`, `(a |1|)`, `(|a/b|)`, `(a b 100000000000000000000000000000000000000)`, `(||)`} {
		ns, err := reader.ReadAll("name.scm", src, nil)
		if err != nil {
			t.Fatal(err)
		}
		key, _, err := parseLibraryName(ns[0])
		if err != nil || seen[key] {
			t.Fatal(src, key, err)
		}
		seen[key] = true
	}
	for _, src := range []string{`()`, `(a -1)`, `(a 1.0)`, `(a "x")`, `a`, `(a . b)`} {
		ns, err := reader.ReadAll("bad.scm", src, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := parseLibraryName(ns[0]); err == nil {
			t.Error(src)
		}
	}
}
