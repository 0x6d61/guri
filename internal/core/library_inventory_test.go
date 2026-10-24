package core

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/reader"
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
