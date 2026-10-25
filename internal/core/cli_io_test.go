package core_test

import (
	"context"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/ports"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestConfiguredStandardPortsShareLanguageParameters(t *testing.T) {
	r := core.NewRuntime()
	input := ports.InputString("42")
	output := ports.MemoryOutput(false)
	diagnostic := ports.MemoryOutput(false)
	if err := r.SetStandardPorts(input, output, diagnostic); err != nil {
		t.Fatal(err)
	}
	values, err := r.Run(context.Background(), "stdio.scm", `(import (scheme base) (scheme read) (scheme write)) (write (read)) (display "diagnostic" (current-error-port))`)
	if err != nil || values.Len() != 1 || values.Values()[0] != scheme.Unspecified {
		t.Fatal(values, err)
	}
	got, err := output.OutputBytes(false)
	if err != nil || string(got) != "42" {
		t.Fatal(string(got), err)
	}
	got, err = diagnostic.OutputBytes(false)
	if err != nil || string(got) != "diagnostic" {
		t.Fatal(string(got), err)
	}
}
func TestConfigurePortsRejectsInvalidInputAtomically(t *testing.T) {
	r := core.NewRuntime()
	input := ports.InputString("42")
	output := ports.MemoryOutput(false)
	diagnostic := ports.MemoryOutput(false)
	if err := r.SetStandardPorts(input, output, diagnostic); err != nil {
		t.Fatal(err)
	}
	if err := r.SetStandardPorts(output, diagnostic, output); err == nil {
		t.Fatal("output accepted as input")
	}
	_, err := r.Run(context.Background(), "stdio.scm", `(import (scheme base) (scheme read) (scheme write)) (write (read))`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := output.OutputBytes(false)
	if err != nil || string(got) != "42" {
		t.Fatal(string(got), err)
	}
}
