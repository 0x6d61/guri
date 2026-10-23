package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6d61/guri/internal/conformance"
)

func TestWorker(t *testing.T) {
	for i, arg := range os.Args {
		if arg == "--checker-worker" {
			data, err := os.ReadFile(os.Args[i+1])
			if err != nil {
				os.Exit(2)
			}
			os.Stdout.Write(data)
			os.Exit(0)
		}
	}
}

func checkerFixture(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range map[string]string{"case.scm": "actual", "requirements.csv": "id,section\nR7RS-4.1,4.1\n", "suite.json": `{"name":"cli","cases":[{"id":"literal","requirement_id":"R7RS-4.1/literal","class":"required","origin":"report","file":"case.scm","match":"exact","stdout":"expected","exit_code":0}]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return []string{"-suite", filepath.Join(dir, "suite.json"), "-requirements", filepath.Join(dir, "requirements.csv"), "-program", exe, "-arg=-test.run=^TestWorker$", "-arg=--", "-arg=--checker-worker"}
}

func TestCheckerExitStatusAndJSON(t *testing.T) {
	for _, reportOnly := range []bool{false, true} {
		args := checkerFixture(t)
		want := 1
		if reportOnly {
			args = append(args, "-report-only")
			want = 0
		}
		var out, diagnostics bytes.Buffer
		if got := run(context.Background(), args, &out, &diagnostics); got != want {
			t.Fatalf("exit %d want %d: %s", got, want, diagnostics.String())
		}
		var report conformance.Report
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.SuitePassed || report.Counts["fail"] != 1 {
			t.Fatalf("failure hidden in report-only mode: %+v", report)
		}
		if !strings.Contains(diagnostics.String(), "not a conformance declaration") {
			t.Fatal("missing scope of result")
		}
	}
}

func TestCheckerConfigurationErrors(t *testing.T) {
	for _, extra := range [][]string{{"-timeout=0"}, {"-case=missing"}, {"-program=/does/not/exist"}, {"-unexpected"}, {"positional"}, {"-requirements=/missing"}} {
		var out, diagnostics bytes.Buffer
		args := append(checkerFixture(t), extra...)
		if code := run(context.Background(), args, &out, &diagnostics); code != 2 {
			t.Fatalf("%v exit=%d: %s", extra, code, diagnostics.String())
		}
	}
	var out, diagnostics bytes.Buffer
	if run(context.Background(), []string{"-help"}, &out, &diagnostics) != 0 {
		t.Fatal("help failed")
	}
}

func TestCheckedInSuiteLinksToRequirements(t *testing.T) {
	suite, err := conformance.LoadSuite("../../testdata/r7rs/suite.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := suite.LinkRequirements("../../docs/r7rs/requirements.csv"); err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) < 10 {
		t.Fatal("expected representative standard cases")
	}
	for _, c := range suite.Cases {
		if c.Disposition != "" && c.Disposition != "run" {
			t.Fatalf("initial corpus must run, not hide gaps: %+v", c)
		}
	}
}
