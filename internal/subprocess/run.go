// Package subprocess runs synchronous process pipelines using owned OS pipes.
package subprocess

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

type Options struct {
	Input       []byte
	Directory   string
	Environment []string
}

type Result struct {
	Statuses    []int64 // -1 denotes termination by a signal on Unix.
	Output      []byte
	Diagnostics [][]byte
}

// Run waits for every direct child and every I/O worker before returning.
// Intermediate output flows directly through OS pipes, without Go buffering.
func Run(ctx context.Context, commands [][]string, options Options) (result Result, err error) {
	if len(commands) == 0 {
		return result, errors.New("empty pipeline")
	}
	for _, argv := range commands {
		if len(argv) == 0 || argv[0] == "" {
			return result, errors.New("empty command")
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var files []*os.File
	// Closing a pipe again is expected during cancellation and deferred cleanup.
	closeFiles := func() {
		for _, f := range files {
			_ = f.Close()
		}
	}
	defer closeFiles()
	pipe := func() (*os.File, *os.File, error) {
		r, w, err := os.Pipe()
		if err == nil {
			files = append(files, r, w)
		}
		return r, w, err
	}
	cmds := make([]*exec.Cmd, len(commands))
	diagnostics := make([]bytes.Buffer, len(commands))
	stdin, inputWriter, err := pipe()
	if err != nil {
		return result, err
	}
	var output bytes.Buffer
	type transfer struct {
		dst      io.Writer
		src      io.Reader
		endpoint *os.File
		input    bool
	}
	copies := []transfer{{inputWriter, bytes.NewReader(options.Input), inputWriter, true}}
	// Each child's pipe ends are closed in the parent immediately after Start.
	childEnds := make([][]*os.File, len(commands))
	for i, argv := range commands {
		cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
		cmd.Dir = options.Directory
		if options.Environment != nil {
			cmd.Env = options.Environment
		}
		cmd.Stdin = stdin
		childEnds[i] = append(childEnds[i], stdin)
		stdout, writer, e := pipe()
		if e != nil {
			return result, e
		}
		cmd.Stdout = writer
		childEnds[i] = append(childEnds[i], writer)
		if i+1 == len(commands) {
			copies = append(copies, transfer{&output, stdout, stdout, false})
		} else {
			stdin = stdout
		}
		stderr, writer, e := pipe()
		if e != nil {
			return result, e
		}
		cmd.Stderr = writer
		childEnds[i] = append(childEnds[i], writer)
		copies = append(copies, transfer{&diagnostics[i], stderr, stderr, false})
		cmds[i] = cmd
	}
	// The immutable file list is now safe to share with the cancellation watcher.
	watcherDone := make(chan struct{})
	go func() { defer close(watcherDone); <-runCtx.Done(); closeFiles() }()
	defer func() { cancel(); <-watcherDone }()
	copied := make(chan error, len(copies))
	for _, work := range copies {
		go func() {
			_, e := io.Copy(work.dst, work.src)
			// A child may finish without consuming all its supplied input.
			if work.input && closedChildInput(e) {
				e = nil
			}
			ce := work.endpoint.Close()
			if ce != nil && !errors.Is(ce, os.ErrClosed) {
				e = errors.Join(e, ce)
			}
			if e != nil {
				cancel()
			}
			copied <- e
		}()
	}
	started := 0
	for i, cmd := range cmds {
		if e := cmd.Start(); e != nil {
			err = fmt.Errorf("pipeline stage %d: %w", i+1, e)
			cancel()
			break
		}
		started++
		for _, f := range childEnds[i] {
			if e := f.Close(); e != nil && !errors.Is(e, os.ErrClosed) {
				err = errors.Join(err, e)
				cancel()
			}
		}
		if err != nil {
			break
		}
	}
	result.Statuses = make([]int64, started)
	for i, cmd := range cmds[:started] {
		e := cmd.Wait()
		var exit *exec.ExitError
		if e != nil && !errors.As(e, &exit) {
			err = errors.Join(err, fmt.Errorf("pipeline stage %d: %w", i+1, e))
			cancel()
		}
		if cmd.ProcessState != nil {
			code := cmd.ProcessState.ExitCode()
			if runtime.GOOS == "windows" {
				result.Statuses[i] = int64(uint32(code))
			} else {
				result.Statuses[i] = int64(code)
			}
		}
	}
	for range copies {
		if e := <-copied; e != nil {
			err = errors.Join(err, e)
		}
	}
	if e := ctx.Err(); e != nil {
		return Result{}, e
	}
	if err != nil {
		return Result{}, err
	}
	result.Output = output.Bytes()
	result.Diagnostics = make([][]byte, len(diagnostics))
	for i := range diagnostics {
		result.Diagnostics[i] = diagnostics[i].Bytes()
	}
	return result, nil
}
