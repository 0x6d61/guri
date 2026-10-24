package core

import (
	"context"
	"testing"
	"time"

	"github.com/0x6d61/guri/internal/conformance"
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/scheme"
)

// This runs the same sources through the internal VM, independently from the
// public-CLI conformance reports. It must never replace their measured results.
func TestImportedProgramsFromConformanceSuites(t *testing.T) {
	for _, path := range []string{"../../testdata/r7rs/suite.json", "../../testdata/r7rs/chibi/suite.json"} {
		suite, err := conformance.LoadSuite(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range suite.Cases {
			t.Run(test.ID, func(t *testing.T) {
				r := NewRuntime()
				output := ports.MemoryOutput(false)
				b := r.bootstrap.find(nil, r.bootstrap.globals, scheme.NewIdentifier(r.bootstrap.symbols.Intern("current-output-port")))
				proc, err := b.cell.Get()
				if err != nil {
					t.Fatal(err)
				}
				proc.(*scheme.Procedure).Code().(*parameter).initial = ports.Value(output)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_, err = r.Run(ctx, test.File, test.Source)
				actual, readErr := output.OutputBytes(false)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if test.ExitCode != 0 {
					t.Fatal("internal unit runner cannot check process exit statuses", test.ExitCode)
				}
				matches := string(actual) == test.Stdout
				if test.Match == "one-of" {
					matches = false
					for _, alternative := range test.Alternatives {
						if string(actual) == alternative {
							matches = true
						}
					}
				} else if test.Match != "exact" {
					t.Fatal("unsupported suite match", test.Match)
				}
				if err != nil || !matches {
					t.Fatalf("stdout %q wanted %q or %q; error %v", actual, test.Stdout, test.Alternatives, err)
				}
			})
		}
	}
}
