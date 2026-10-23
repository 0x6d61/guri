// r7rscheck runs source programs independently of the interpreter implementation.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/0x6d61/guri/internal/conformance"
)

type arguments []string

func (a *arguments) String() string         { return fmt.Sprint([]string(*a)) }
func (a *arguments) Set(value string) error { *a = append(*a, value); return nil }

func main() { os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) }

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("r7rscheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	suitePath := flags.String("suite", "testdata/r7rs/suite.json", "suite manifest")
	requirements := flags.String("requirements", "docs/r7rs/requirements.csv", "requirement section table")
	program := flags.String("program", "", "interpreter executable (source file is appended as final argument)")
	timeout := flags.Duration("timeout", 5*time.Second, "timeout per isolated case")
	reportOnly := flags.Bool("report-only", false, "report language failures without enforcing success; infrastructure errors still fail")
	caseID := flags.String("case", "", "run only this case ID")
	var prefix arguments
	flags.Var(&prefix, "arg", "interpreter prefix argument; repeat as needed")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *program == "" || *timeout <= 0 {
		fmt.Fprintln(stderr, "provide -program and a positive -timeout, with no positional arguments")
		return 2
	}
	suite, err := conformance.LoadSuite(*suitePath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := suite.LinkRequirements(*requirements); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *caseID != "" {
		found := false
		for _, c := range suite.Cases {
			if c.ID == *caseID {
				suite.Cases = []conformance.Case{c}
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(stderr, "unknown case %q\n", *caseID)
			return 2
		}
	}
	command := append([]string{*program}, prefix...)
	report := conformance.Run(ctx, suite, command, *timeout)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintln(stderr, report.Summary())
	if report.Counts["error"] > 0 {
		return 2
	}
	if !*reportOnly && !report.SuitePassed {
		return 1
	}
	return 0
}
