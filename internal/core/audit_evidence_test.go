package core

import (
	"context"
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/equality"
	"github.com/0x6d61/guri/internal/reader"
)

func TestAuditEvidenceIndex(t *testing.T) {
	read := func(path string) [][]string {
		t.Helper()
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		rows, err := csv.NewReader(f).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	requirements := read("../../docs/r7rs/requirements.csv")
	headings := map[string]string{}
	for _, row := range requirements[1:] {
		if !strings.Contains(row[0], "/") {
			headings[row[1]] = row[2]
		}
	}
	definitions := map[string]bool{}
	functions := regexp.MustCompile(`(?m)^func ((?:Test|Example)[A-Za-z0-9_]+)\(`)
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range functions.FindAllSubmatch(data, -1) {
			definitions[string(match[1])] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := read("../../docs/r7rs/audit/sections.csv")
	seen := map[string]bool{}
	if len(rows)-1 != 96 || len(headings) != 96 {
		t.Fatal("heading count", len(rows), len(headings))
	}
	for _, row := range rows[1:] {
		if seen[row[0]] || headings[row[0]] != row[1] || row[2] == "" || row[3] == "" || row[7] == "" {
			t.Fatal(row)
		}
		seen[row[0]] = true
		for _, path := range append(strings.Split(row[4], ";"), row[6]) {
			if _, err := os.Stat(filepath.Join("../..", path)); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range strings.Split(row[5], ";") {
			if !definitions[name] {
				t.Fatal("missing test evidence", name, row[0])
			}
		}
	}
}
func TestAuditWikiExamples(t *testing.T) {
	doc, err := os.ReadFile("../../docs/wiki/R7RS-Audit.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```scheme\\n(.*?)\\n```").FindAllStringSubmatch(string(doc), -1)
	expected := []string{"100", "matched"}
	if len(blocks) != len(expected) {
		t.Fatal(len(blocks))
	}
	for i, block := range blocks {
		r := NewRuntime()
		result, err := r.Run(context.Background(), "audit-example.scm", block[1])
		if err != nil {
			t.Fatal(err)
		}
		value, err := result.Single()
		if err != nil {
			t.Fatal(err)
		}
		want, err := reader.ReadAll("expected.scm", expected[i], nil)
		if err != nil {
			t.Fatal(err)
		}
		if !equality.Equal(value, want[0].Datum()) {
			t.Fatal(i, value)
		}
	}
}
