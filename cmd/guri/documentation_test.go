package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCLIDocumentedExamples(t *testing.T) {
	examples := map[string][]string{
		"README.md":                           {"15\n"},
		"docs/wiki/Home.md":                   {"42\n"},
		"docs/wiki/Language-Specification.md": {"(1 true)\n", "(#(9 2 3) #(100 2 3))\n"},
		"docs/wiki/Special-Forms.md":          {"6\n", "positive\n012\n"},
		"docs/wiki/Functions.md":              {"((9 2 3) 6)\n"},
		"docs/wiki/CLI.md":                    {"Hello, Scheme!\n"},
	}
	pattern := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```")
	for path, expected := range examples {
		t.Run(path, func(t *testing.T) {
			doc, err := os.ReadFile(filepath.Join("../..", path))
			if err != nil {
				t.Fatal(err)
			}
			blocks := pattern.FindAllStringSubmatch(string(doc), -1)
			if len(blocks) != len(expected) {
				t.Fatalf("blocks=%d expected=%d", len(blocks), len(expected))
			}
			for i, block := range blocks {
				code, output, diagnostic := invoke([]string{script(t, block[1])}, "")
				if code != 0 || output != expected[i] || diagnostic != "" {
					t.Fatalf("block %d: exit=%d output=%q diagnostic=%s", i, code, output, diagnostic)
				}
			}
		})
	}
}

func TestPublicBindingReferenceIndex(t *testing.T) {
	f, err := os.Open("../../docs/r7rs/exports.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile("../../docs/wiki/Functions.md")
	if err != nil {
		t.Fatal(err)
	}
	libraries := map[string][]string{}
	for _, row := range rows[1:] {
		if row[5] != "公開CLIのimport実装済み" {
			t.Fatal(row)
		}
		libraries[row[0]] = append(libraries[row[0]], row[1])
	}
	if len(libraries) != 16 || len(rows)-1 != 557 {
		t.Fatalf("libraries=%d bindings=%d", len(libraries), len(rows)-1)
	}
	for library, names := range libraries {
		heading := "### `" + library + "`"
		_, section, ok := strings.Cut(string(doc), heading)
		if !ok {
			t.Fatal("missing library", library)
		}
		section, _, _ = strings.Cut(section, "\n##")
		for _, name := range names {
			if !strings.Contains(section, "`"+name+"`") {
				t.Fatal("missing binding", library, name)
			}
		}
	}
}

func TestLegacyTestMigrationEvidence(t *testing.T) {
	f, err := os.Open("../../docs/r7rs/legacy-tests.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows)-1 != 56 {
		t.Fatalf("migration entries=%d", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := row[1] + ":" + row[2]
		if seen[key] || row[0] != "412cf98cd9999dd48443da10deb591969907d183" || row[3] == "" || row[6] == "" {
			t.Fatal(row)
		}
		seen[key] = true
		source, err := os.ReadFile(filepath.Join("../..", row[4]))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "func "+row[5]+"(") {
			t.Fatal("missing replacement test", row)
		}
	}
}
