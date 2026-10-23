package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/conformance"
)

const chibiRevision = "b2087f3a320a56f79ede5c546fd45fdde582c4ed"
const chibiRoot = "../../testdata/r7rs/upstream/chibi"
const chibiCases = "../../testdata/r7rs/chibi"

func readChibiFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestChibiUpstreamIntegrity(t *testing.T) {
	for name, want := range map[string]string{
		"r7rs-tests.scm": "004d215a38f990bbf2bfaa8b4a395aa59ef98d155096f8e31cc361b7812f7d6a",
		"COPYING":        "86805fe7af1bd20d5f5050d86c61d6fe038452cb08c6106a08761e010d37da43",
	} {
		t.Run(name, func(t *testing.T) {
			data := readChibiFile(t, filepath.Join(chibiRoot, name))
			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
				t.Fatalf("upstream %s changed: %s", name, got)
			}
		})
	}
}

func TestChibiCasesKeepLicenseAndOriginalAssertions(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join(chibiCases, "suite.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := suite.LinkRequirements("../../docs/r7rs/requirements.csv"); err != nil {
		t.Fatal(err)
	}
	var provenance struct {
		Revision string `json:"revision"`
		Cases    []struct {
			ID            string `json:"id"`
			StartLine     int    `json:"start_line"`
			EndLine       int    `json:"end_line"`
			Specification string `json:"specification"`
			Adaptation    string `json:"adaptation"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(readChibiFile(t, filepath.Join(chibiCases, "provenance.json")), &provenance); err != nil {
		t.Fatal(err)
	}
	if provenance.Revision != chibiRevision || len(provenance.Cases) != len(suite.Cases) || len(suite.Cases) == 0 {
		t.Fatal("missing upstream revision or case provenance")
	}
	original := strings.Split(string(readChibiFile(t, filepath.Join(chibiRoot, "r7rs-tests.scm"))), "\n")
	license := string(readChibiFile(t, filepath.Join(chibiRoot, "COPYING")))
	cases := map[string]conformance.Case{}
	for _, c := range suite.Cases {
		cases[c.ID] = c
	}
	seen := map[string]bool{}
	for _, p := range provenance.Cases {
		c, ok := cases[p.ID]
		if !ok || seen[p.ID] {
			t.Fatalf("missing or duplicate case %s", p.ID)
		}
		seen[p.ID] = true
		if p.StartLine < 1 || p.EndLine < p.StartLine || p.EndLine > len(original) {
			t.Fatalf("invalid source range: %+v", p)
		}
		assertion := strings.Join(original[p.StartLine-1:p.EndLine], "\n") + "\n"
		if !strings.HasSuffix(c.Source, assertion) {
			t.Fatalf("%s: original assertion was changed or removed", p.ID)
		}
		for _, line := range strings.Split(strings.TrimSuffix(license, "\n"), "\n") {
			if line != "" && !strings.Contains(c.Source, ";; "+line+"\n") {
				t.Fatalf("%s: missing license line %q", p.ID, line)
			}
		}
		origin := fmt.Sprintf("https://github.com/ashinn/chibi-scheme/blob/%s/tests/r7rs-tests.scm#L%d-L%d", chibiRevision, p.StartLine, p.EndLine)
		if c.Origin != origin || !strings.Contains(c.Source, origin) || p.Specification == "" || p.Adaptation == "" {
			t.Fatalf("%s: missing provenance", p.ID)
		}
		if c.Disposition != "" && c.Disposition != "run" {
			t.Fatalf("%s: upstream gap hidden behind %s", p.ID, c.Disposition)
		}
		if c.Match != "exact" || c.Stdout != "ok" || c.ExitCode != 0 {
			t.Fatalf("%s: assertion failures must be observable", p.ID)
		}
	}
	files, err := filepath.Glob(filepath.Join(chibiCases, "*.scm"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(suite.Cases) {
		t.Fatal("derived source files and registered cases differ")
	}
}
