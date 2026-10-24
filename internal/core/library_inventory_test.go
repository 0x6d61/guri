package core

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestStandardLibraryCatalog(t *testing.T) {
	f, err := os.Open("../../docs/r7rs/exports.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	ledger := map[string]map[string]bool{}
	for _, row := range rows[1:] {
		if ledger[row[0]] == nil {
			ledger[row[0]] = map[string]bool{}
		}
		ledger[row[0]][row[1]] = true
	}
	if len(ledger) != 16 || len(standardLibraryNames) != 16 {
		t.Fatal("standard library count")
	}
	complete := map[string]bool{"(scheme base)": true, "(scheme case-lambda)": true, "(scheme char)": true, "(scheme complex)": true, "(scheme cxr)": true, "(scheme file)": true, "(scheme inexact)": true, "(scheme read)": true, "(scheme write)": true}
	r := NewRuntime()
	for library, names := range standardLibraryNames {
		seen := map[string]bool{}
		for _, name := range strings.Fields(names) {
			if !ledger[library][name] || seen[name] {
				t.Fatal("catalog mismatch", library, name)
			}
			seen[name] = true
		}
		if len(seen) != len(ledger[library]) {
			t.Fatal("catalog missing identifiers", library)
		}
		ns, err := reader.ReadAll("catalog.scm", library, nil)
		if err != nil {
			t.Fatal(err)
		}
		key, _, err := parseLibraryName(ns[0])
		if err != nil {
			t.Fatal(err)
		}
		l, present := r.modules[key]
		if present != complete[library] {
			t.Fatal("incorrect availability", library, present)
		}
		if present && len(l.exports) != len(seen) {
			t.Fatal("partial standard library advertised", library)
		}
	}
}
func TestLibraryFilenameEncoding(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range []libraryPart{{text: "1", integer: true}, {text: "1"}, {text: ".."}, {text: "%2E%2E"}, {text: "a/b"}, {text: "a%2Fb"}, {text: ""}, {text: "\x00"}, {text: "AUX"}, {text: "猫"}} {
		filename := encodedLibraryPart(p)
		if seen[filename] || strings.ContainsAny(filename, "/\\") {
			t.Fatal(p, filename)
		}
		seen[filename] = true
	}
	if got := encodedLibraryPart(libraryPart{text: "myapp"}); got != "myapp" {
		t.Fatal(got)
	}
}

func TestConfiguredInstalledLibraryDirectories(t *testing.T) {
	t.Setenv("GURI_LOAD_PATH", "")
	previousSite, previousStandard := siteLibraryDirectory, standardLibraryDirectory
	t.Cleanup(func() { siteLibraryDirectory = previousSite; standardLibraryDirectory = previousStandard })
	standardLibraryDirectory = t.TempDir()
	siteLibraryDirectory = t.TempDir()
	put := func(root string, n int) {
		t.Helper()
		source := fmt.Sprintf(`(define-library (installed) (export value) (import (scheme base)) (begin (define value %d)))`, n)
		if err := os.WriteFile(filepath.Join(root, "installed.sld"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want int, paths ...string) {
		t.Helper()
		r := NewRuntime(paths...)
		result, err := r.Run(context.Background(), filepath.Join(t.TempDir(), "unrelated-script.scm"), `(import (installed)) value`)
		if err != nil || result.Len() != 1 || result.Values()[0].(*scheme.Integer).BigInt().Int64() != int64(want) {
			t.Fatal(result, err)
		}
	}
	put(standardLibraryDirectory, 1)
	check(1)
	put(siteLibraryDirectory, 2)
	check(2)
	environmentRoot := t.TempDir()
	put(environmentRoot, 3)
	t.Setenv("GURI_LOAD_PATH", environmentRoot)
	check(3)
	explicitRoot := t.TempDir()
	put(explicitRoot, 4)
	check(4, explicitRoot)
}
