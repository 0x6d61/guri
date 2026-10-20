package main

import (
	"fmt"
	"io"
	"os"

	"github.com/0x6d61/guri"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--help" {
		fmt.Fprintln(stdout, "Usage: guri <file.guri>")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: guri <file.guri>")
		return 2
	}
	source, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	i := guri.NewInterpreter()
	i.Output = stdout
	if _, err := i.Eval(args[0], string(source)); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
