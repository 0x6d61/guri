package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/ports"
)

var version = "0.0.3"

const usage = `Usage: guri [options] [script.scm [args...]]
  (no script)      Start the Scheme REPL
  -               Read a Scheme program from standard input
  -I DIR, -IDIR   Add a library search directory (repeatable)
  --              End interpreter options
  --help          Show this help
  --version       Show the version
`

func main() { os.Exit(runCommand(os.Args[0], os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	return runCommand("guri", args, input, stdout, stderr)
}
func runCommand(command string, args []string, input io.Reader, stdout, stderr io.Writer) int {
	var paths []string
	index := 0
	for index < len(args) {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if arg == "--help" {
			if _, err := io.WriteString(stdout, usage); err != nil {
				return 1
			}
			return 0
		}
		if arg == "--version" {
			if _, err := fmt.Fprintf(stdout, "guri %s\n", version); err != nil {
				return 1
			}
			return 0
		}
		if arg == "-I" {
			index++
			if index == len(args) || args[index] == "" {
				fmt.Fprintln(stderr, "-I requires a directory")
				return 2
			}
			paths = append(paths, args[index])
			index++
			continue
		}
		if strings.HasPrefix(arg, "-I") {
			paths = append(paths, arg[2:])
			index++
			continue
		}
		if arg != "-" && strings.HasPrefix(arg, "-") {
			fmt.Fprintf(stderr, "unknown option: %s\n%s", arg, usage)
			return 2
		}
		break
	}
	if index == len(args) {
		input, stdout, stderr = interactiveStreams(input, stdout, stderr)
	}
	r := core.NewRuntime(paths...)
	in := ports.Input(input, false, ports.Options{Filename: "<stdin>"})
	if file, ok := input.(*os.File); ok {
		in = ports.FileInput(file, false, false)
	}
	out := ports.Output(stdout, false, ports.Options{})
	diagnostic := ports.Output(stderr, false, ports.Options{})
	if err := r.SetStandardPorts(in, out, diagnostic); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var err error
	if index == len(args) {
		if err = r.SetCommandLine(command); err == nil {
			err = r.REPL(context.Background(), "<stdin>", input, stdout, stderr)
		}
	} else {
		filename := args[index]
		if err = r.SetCommandLine(args[index:]...); err == nil {
			var data []byte
			if filename == "-" {
				data, err = io.ReadAll(input)
			} else {
				data, err = os.ReadFile(filename)
			}
			if err == nil {
				_, err = r.Run(context.Background(), filename, string(data))
			}
		}
	}
	var exit *core.ExitRequest
	if errors.As(err, &exit) {
		// Emergency termination skips Scheme after procedures and frontend flushing.
		if !exit.Emergency {
			if e := flushOpen(out); e != nil {
				fmt.Fprintln(stderr, e)
			}
			if e := flushOpen(diagnostic); e != nil {
				fmt.Fprintln(stderr, e)
			}
		}
		return exit.Code
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := flushOpen(out); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := flushOpen(diagnostic); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func flushOpen(p *ports.Resource) error {
	if !p.Open() {
		return nil
	}
	return p.Flush()
}
