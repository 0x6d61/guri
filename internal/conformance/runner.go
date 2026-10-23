// Package conformance runs isolated source programs against explicit expectations.
// It does not depend on Guri's reader, value representation, or evaluator.
package conformance

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Case describes a standalone program. Expectations come from the cited source,
// rather than the interpreter's existing behavior.
type Case struct {
	ID            string   `json:"id"`
	RequirementID string   `json:"requirement_id"`
	Class         string   `json:"class"`
	Origin        string   `json:"origin"`
	File          string   `json:"file"`
	Match         string   `json:"match"`
	Stdout        string   `json:"stdout,omitempty"`
	Alternatives  []string `json:"alternatives,omitempty"`
	ExitCode      int      `json:"exit_code"`
	Disposition   string   `json:"disposition,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	Issue         int      `json:"issue,omitempty"`
	Source        string   `json:"-"`
	path          string
}

type Suite struct {
	Name     string `json:"name"`
	Cases    []Case `json:"cases"`
	sections []string
}

var requirementID = regexp.MustCompile(`^R7RS-([1-7](?:\.[0-9]+)*)(?:/[a-zA-Z0-9_./-]+)?$`)

func section(id string) string {
	match := requirementID.FindStringSubmatch(id)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func LoadSuite(path string) (Suite, error) {
	var suite Suite
	data, err := os.ReadFile(path)
	if err != nil {
		return suite, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&suite); err != nil {
		return suite, err
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return suite, fmt.Errorf("expected one JSON document")
	}
	if suite.Name == "" || len(suite.Cases) == 0 {
		return suite, fmt.Errorf("suite needs a name and at least one case")
	}
	root, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return suite, err
	}
	ids := make(map[string]bool)
	for i := range suite.Cases {
		c := &suite.Cases[i]
		if c.ID == "" || ids[c.ID] {
			return suite, fmt.Errorf("empty or duplicate case ID %q", c.ID)
		}
		ids[c.ID] = true
		if !requirementID.MatchString(c.RequirementID) || c.Origin == "" {
			return suite, fmt.Errorf("%s: requirement ID and origin are required", c.ID)
		}
		switch c.Class {
		case "required", "optional", "implementation-defined", "unspecified", "error-required", "error-optional":
		default:
			return suite, fmt.Errorf("%s: unknown class %q", c.ID, c.Class)
		}
		switch c.Match {
		case "exact":
			if len(c.Alternatives) > 0 {
				return suite, fmt.Errorf("%s: exact match cannot have alternatives", c.ID)
			}
		case "one-of":
			if len(c.Alternatives) < 2 || c.Stdout != "" {
				return suite, fmt.Errorf("%s: one-of needs at least two alternatives and no stdout", c.ID)
			}
		case "observe":
			if c.Class != "unspecified" && c.Class != "error-optional" && c.Class != "implementation-defined" {
				return suite, fmt.Errorf("%s: required result cannot be observation-only", c.ID)
			}
			if c.Stdout != "" || len(c.Alternatives) > 0 {
				return suite, fmt.Errorf("%s: observation cannot assert output", c.ID)
			}
		default:
			return suite, fmt.Errorf("%s: unknown match %q", c.ID, c.Match)
		}
		if c.Class == "unspecified" && c.Match == "exact" {
			return suite, fmt.Errorf("%s: unspecified result needs alternatives, observation, or a property-check program", c.ID)
		}
		switch c.Disposition {
		case "", "run":
		case "skip", "unsupported", "restricted", "xfail":
			if c.Reason == "" || c.Issue <= 0 {
				return suite, fmt.Errorf("%s: incomplete case needs reason and tracking issue", c.ID)
			}
		default:
			return suite, fmt.Errorf("%s: unknown disposition %q", c.ID, c.Disposition)
		}
		if c.ExitCode < 0 || c.ExitCode > 255 {
			return suite, fmt.Errorf("%s: invalid expected exit code", c.ID)
		}
		if c.File == "" || filepath.IsAbs(c.File) || !filepath.IsLocal(c.File) {
			return suite, fmt.Errorf("%s: source must be relative to the suite", c.ID)
		}
		c.path = filepath.Join(root, c.File)
		source, err := os.ReadFile(c.path)
		if err != nil {
			return suite, fmt.Errorf("%s: %w", c.ID, err)
		}
		c.Source = string(source)
	}
	return suite, nil
}

// LinkRequirements validates the exact parent section, and records which
// sections have no cases. Section counts are not a measure of conformance.
func (s *Suite) LinkRequirements(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return err
	}
	if len(rows) < 2 {
		return fmt.Errorf("empty requirements table")
	}
	idCol, sectionCol := -1, -1
	for i, h := range rows[0] {
		switch h {
		case "id":
			idCol = i
		case "section":
			sectionCol = i
		}
	}
	if idCol < 0 || sectionCol < 0 {
		return fmt.Errorf("requirements need id and section columns")
	}
	ids := make(map[string]bool)
	sections := make(map[string]bool)
	for _, row := range rows[1:] {
		id, sec := row[idCol], row[sectionCol]
		if !requirementID.MatchString(id) || section(id) != sec || ids[id] {
			return fmt.Errorf("invalid or duplicate requirement %q", id)
		}
		ids[id] = true
		sections[sec] = true
	}
	for _, c := range s.Cases {
		if !ids["R7RS-"+section(c.RequirementID)] {
			return fmt.Errorf("%s: unknown requirement %s", c.ID, c.RequirementID)
		}
	}
	s.sections = nil
	for sec := range sections {
		s.sections = append(s.sections, sec)
	}
	sort.Strings(s.sections)
	return nil
}

type Result struct {
	ID              string   `json:"id"`
	RequirementID   string   `json:"requirement_id"`
	Class           string   `json:"class"`
	Origin          string   `json:"origin"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason,omitempty"`
	Issue           int      `json:"issue,omitempty"`
	Command         []string `json:"command"`
	Source          string   `json:"source"`
	ExpectedExit    int      `json:"expected_exit"`
	Match           string   `json:"match"`
	ExpectedStdout  string   `json:"expected_stdout,omitempty"`
	Alternatives    []string `json:"alternatives,omitempty"`
	Stdout          string   `json:"stdout"`
	Stderr          string   `json:"stderr"`
	ExitCode        int      `json:"exit_code"`
	DurationMS      int64    `json:"duration_ms"`
	OutputTruncated bool     `json:"output_truncated"`
}

type Report struct {
	SchemaVersion     int                       `json:"schema_version"`
	Suite             string                    `json:"suite"`
	SuitePassed       bool                      `json:"suite_passed"`
	Counts            map[string]int            `json:"counts"`
	Sections          map[string]map[string]int `json:"sections"`
	Chapters          map[string]map[string]int `json:"chapters"`
	UncoveredSections []string                  `json:"uncovered_sections"`
	Results           []Result                  `json:"results"`
}

func Run(ctx context.Context, suite Suite, command []string, timeout time.Duration) Report {
	report := Report{
		SchemaVersion: 1, Suite: suite.Name, SuitePassed: len(suite.Cases) > 0,
		Counts: make(map[string]int), Sections: make(map[string]map[string]int),
		Chapters: make(map[string]map[string]int), UncoveredSections: []string{}, Results: []Result{},
	}
	for _, c := range suite.Cases {
		result := runCase(ctx, c, command, timeout)
		report.Results = append(report.Results, result)
		report.Counts[result.Status]++
		sec := section(c.RequirementID)
		if report.Sections[sec] == nil {
			report.Sections[sec] = make(map[string]int)
		}
		report.Sections[sec][result.Status]++
		chapter := strings.Split(sec, ".")[0]
		if report.Chapters[chapter] == nil {
			report.Chapters[chapter] = make(map[string]int)
		}
		report.Chapters[chapter][result.Status]++
		if result.Status != "pass" {
			report.SuitePassed = false
		}
	}
	for _, sec := range suite.sections {
		if report.Sections[sec] == nil {
			report.UncoveredSections = append(report.UncoveredSections, sec)
		}
	}
	return report
}

const outputLimit = 1024 * 1024

type limitedBuffer struct {
	buffer    bytes.Buffer
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := outputLimit - b.buffer.Len()
	if n > remaining {
		b.truncated = true
		p = p[:remaining]
	}
	_, err := b.buffer.Write(p)
	return n, err
}

func runCase(ctx context.Context, c Case, command []string, timeout time.Duration) Result {
	result := Result{
		ID: c.ID, RequirementID: c.RequirementID, Class: c.Class, Origin: c.Origin,
		Issue: c.Issue, Reason: c.Reason, Source: c.Source, ExpectedExit: c.ExitCode,
		Match: c.Match, ExpectedStdout: c.Stdout, Alternatives: c.Alternatives, ExitCode: -1,
		Command: append(append([]string{}, command...), c.path),
	}
	switch c.Disposition {
	case "skip", "unsupported", "restricted":
		result.Status = c.Disposition
		return result
	}
	if len(command) == 0 || timeout <= 0 {
		result.Status = "error"
		result.Reason = "command and positive timeout are required"
		return result
	}
	childCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(childCtx, command[0], result.Command[1:]...)
	cmd.WaitDelay = 100 * time.Millisecond
	var stdout, stderr limitedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	result.DurationMS = time.Since(start).Milliseconds()
	result.Stdout, result.Stderr = stdout.buffer.String(), stderr.buffer.String()
	result.OutputTruncated = stdout.truncated || stderr.truncated
	if ctx.Err() != nil {
		result.Status = "error"
		result.Reason = ctx.Err().Error()
		return result
	}
	if childCtx.Err() == context.DeadlineExceeded {
		result.Status = "timeout"
		result.Reason = "case exceeded timeout"
		return result
	}
	if result.OutputTruncated {
		result.Status = "error"
		result.Reason = "output exceeded 1 MiB per stream"
		return result
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	var exitErr *exec.ExitError
	if err != nil && (!errors.As(err, &exitErr) || result.ExitCode < 0) {
		result.Status = "error"
		result.Reason = err.Error()
		return result
	}
	if c.Match == "observe" {
		result.Status = "observed"
		return result
	}
	matched := result.Stdout == c.Stdout
	if c.Match == "one-of" {
		matched = false
		for _, candidate := range c.Alternatives {
			if result.Stdout == candidate {
				matched = true
				break
			}
		}
	}
	result.Status = "pass"
	if result.ExitCode != c.ExitCode || !matched {
		result.Status = "fail"
	}
	if c.Disposition == "xfail" {
		if result.Status == "pass" {
			result.Status = "xpass"
		} else {
			result.Status = "xfail"
		}
	}
	return result
}

// Summary is a human-readable supplement to the JSON report.
func (r Report) Summary() string {
	statuses := make([]string, 0, len(r.Counts))
	for status := range r.Counts {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	var parts []string
	for _, status := range statuses {
		parts = append(parts, fmt.Sprintf("%s=%d", status, r.Counts[status]))
	}
	return fmt.Sprintf("%s: %s; suite_passed=%t (selected cases only, not a conformance declaration)", r.Suite, strings.Join(parts, ", "), r.SuitePassed)
}
