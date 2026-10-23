package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--conformance-helper" {
			continue
		}
		data, err := os.ReadFile(os.Args[i+1])
		if err != nil {
			os.Exit(7)
		}
		switch string(data) {
		case "pass":
			fmt.Print("ok\n")
		case "fail":
			fmt.Fprint(os.Stderr, "diagnostic\n")
			os.Exit(1)
		case "slow":
			time.Sleep(10 * time.Second)
		case "flood":
			fmt.Print(strings.Repeat("x", 2*1024*1024))
		case "choice":
			fmt.Print("second\n")
		default:
			os.Exit(9)
		}
		os.Exit(0)
	}
}

func fixture(t *testing.T, cases ...Case) Suite {
	t.Helper()
	dir := t.TempDir()
	for i := range cases {
		if cases[i].ID == "" {
			cases[i].ID = fmt.Sprintf("case-%d", i)
		}
		if cases[i].RequirementID == "" {
			cases[i].RequirementID = "R7RS-4.1/literal"
		}
		if cases[i].Class == "" {
			cases[i].Class = "required"
		}
		if cases[i].Match == "" {
			cases[i].Match = "exact"
		}
		if cases[i].Origin == "" {
			cases[i].Origin = "https://standards.scheme.org/official/r7rs.pdf"
		}
		name := fmt.Sprintf("%d.scm", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(cases[i].Source), 0600); err != nil {
			t.Fatal(err)
		}
		cases[i].File = name
	}
	raw, err := json.Marshal(Suite{Name: "fixture", Cases: cases})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "suite.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatal(err)
	}
	return suite
}

func helper(t *testing.T) []string {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return []string{path, "-test.run=^TestProcess$", "--", "--conformance-helper"}
}

func TestRunReportsFailuresAndReproduction(t *testing.T) {
	suite := fixture(t, Case{Source: "pass", Stdout: "ok\n"}, Case{Source: "fail", Stdout: "ok\n"})
	report := Run(context.Background(), suite, helper(t), time.Second)
	if report.SuitePassed || report.Counts["pass"] != 1 || report.Counts["fail"] != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	failed := report.Results[1]
	if failed.ExitCode != 1 || failed.Stderr != "diagnostic\n" || failed.Source != "fail" || failed.RequirementID != "R7RS-4.1/literal" || len(failed.Command) == 0 {
		t.Fatalf("missing reproduction: %+v", failed)
	}
	if report.Sections["4.1"]["fail"] != 1 || report.Chapters["4"]["fail"] != 1 {
		t.Fatalf("missing section summary: %+v", report.Sections)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"suite_passed":false`) {
		t.Fatalf("missing machine-readable failure: %s", raw)
	}
}

func TestRunIncompleteAndExpectedFailuresNeverPassSuite(t *testing.T) {
	suite := fixture(t, Case{Source: "fail", Stdout: "ok\n", Disposition: "xfail", Reason: "not implemented", Issue: 48}, Case{Source: "pass", Stdout: "ok\n", Disposition: "xfail", Reason: "stale expectation", Issue: 48}, Case{Source: "slow", Disposition: "unsupported", Reason: "missing feature", Issue: 48}, Case{Source: "slow", Disposition: "skip", Reason: "not selected", Issue: 48}, Case{Source: "slow", Disposition: "restricted", Reason: "profile undecided", Issue: 54})
	report := Run(context.Background(), suite, helper(t), time.Second)
	for _, status := range []string{"xfail", "xpass", "unsupported", "skip", "restricted"} {
		if report.Counts[status] != 1 {
			t.Fatalf("missing %s: %+v", status, report)
		}
	}
	if report.SuitePassed {
		t.Fatal("incomplete suite reported success")
	}
}

func TestRunTimeoutAndInfrastructureErrors(t *testing.T) {
	suite := fixture(t, Case{Source: "slow"})
	report := Run(context.Background(), suite, helper(t), 100*time.Millisecond)
	if report.Results[0].Status != "timeout" || report.SuitePassed {
		t.Fatalf("timeout not reported: %+v", report)
	}
	report = Run(context.Background(), suite, []string{filepath.Join(t.TempDir(), "missing")}, time.Second)
	if report.Results[0].Status != "error" {
		t.Fatalf("launch error not reported: %+v", report)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report = Run(ctx, suite, helper(t), time.Second)
	if report.Results[0].Status != "error" {
		t.Fatalf("cancellation reported as a test result: %+v", report)
	}
}

func TestRunRejectsExcessiveOutput(t *testing.T) {
	report := Run(context.Background(), fixture(t, Case{Source: "flood"}), helper(t), time.Second)
	if report.Results[0].Status != "error" || !report.Results[0].OutputTruncated || len(report.Results[0].Stdout) > 1024*1024 {
		t.Fatalf("output limit missing: %+v", report.Counts)
	}
}

func TestUnspecifiedAlternativesAndObservation(t *testing.T) {
	suite := fixture(t, Case{Source: "choice", Class: "unspecified", Match: "one-of", Alternatives: []string{"first\n", "second\n"}}, Case{Source: "fail", Class: "error-optional", Match: "observe"})
	report := Run(context.Background(), suite, helper(t), time.Second)
	if report.Counts["pass"] != 1 || report.Counts["observed"] != 1 || report.SuitePassed {
		t.Fatalf("observation counted as conformance: %+v", report)
	}
}

func TestMatchingExitAndStrictOutput(t *testing.T) {
	report := Run(context.Background(), fixture(t, Case{Source: "fail", ExitCode: 1, Stdout: "", Class: "error-required"}, Case{Source: "pass", Stdout: "ok"}), helper(t), time.Second)
	if report.Results[0].Status != "pass" || report.Results[1].Status != "fail" {
		t.Fatalf("exit/stdout comparison: %+v", report)
	}
	report = Run(context.Background(), fixture(t, Case{Source: "pass", Stdout: "ok\n"}), helper(t), time.Second)
	if !report.SuitePassed {
		t.Fatal("passing suite reported failure")
	}
}

func TestLoadSuiteRejectsInvalidManifests(t *testing.T) {
	for _, raw := range []string{`{}`, `{"name":"x","cases":[]}`, `{"name":"x","unknown":true}`, `{"name":"x","cases":[{"id":"a"}]}`, `{"name":"x","cases":[]} {}`} {
		t.Run(raw, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "suite.json")
			os.WriteFile(path, []byte(raw), 0600)
			if _, err := LoadSuite(path); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
	base := Case{ID: "a", File: "source.scm", RequirementID: "R7RS-4.1/a", Class: "required", Origin: "report", Match: "exact"}
	for _, alter := range []func(*Case){func(c *Case) { c.File = "../outside.scm" }, func(c *Case) { c.RequirementID = "missing" }, func(c *Case) { c.Class = "maybe" }, func(c *Case) { c.Match = "observe" }, func(c *Case) { c.Disposition = "skip" }, func(c *Case) { c.Match = "one-of" }, func(c *Case) { c.Disposition = "unknown" }} {
		c := base
		alter(&c)
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "source.scm"), []byte("pass"), 0600)
		raw, _ := json.Marshal(Suite{Name: "bad", Cases: []Case{c}})
		path := filepath.Join(dir, "suite.json")
		os.WriteFile(path, raw, 0600)
		if _, err := LoadSuite(path); err == nil {
			t.Fatalf("invalid case accepted: %+v", c)
		}
	}
}

func TestRequirementCoverage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.csv")
	os.WriteFile(path, []byte("id,section\nR7RS-4.1,4.1\nR7RS-6.2,6.2\n"), 0600)
	suite := fixture(t, Case{Source: "pass", Stdout: "ok\n"})
	if err := suite.LinkRequirements(path); err != nil {
		t.Fatal(err)
	}
	report := Run(context.Background(), suite, helper(t), time.Second)
	if len(report.UncoveredSections) != 1 || report.UncoveredSections[0] != "6.2" {
		t.Fatalf("uncovered sections missing: %+v", report)
	}
	suite.Cases[0].RequirementID = "R7RS-9.9/unknown"
	if err := suite.LinkRequirements(path); err == nil {
		t.Fatal("unknown requirement accepted")
	}
}
