package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/0x6d61/guri/internal/reader"
	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// Extensions have their own export cells, separate from the bootstrap and the
// R7RS catalog. They are available only through explicit import declarations.
func (r *Runtime) installFilesystem() {
	exports := libraryExports{}
	for name, arity := range map[string]pairArity{
		"directory-files": {1, 2}, "create-directory": {1, 1}, "delete-directory": {1, 1},
		"rename-file": {2, 2}, "file-directory?": {1, 2}, "file-regular?": {1, 2},
		"file-size": {1, 1}, "copy-file": {2, 2},
	} {
		procedure := vm.Native(name, 0, -1, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			if len(a) < arity.min || len(a) > arity.max {
				return portFailure(c, name, a, ErrorOrdinary, vm.ErrArity)
			}
			paths := 1
			if name == "rename-file" || name == "copy-file" {
				paths = 2
			}
			args := make([]string, paths)
			for i := range paths {
				s, ok := a[i].(*scheme.String)
				if !ok {
					return portFailure(c, name, a, ErrorOrdinary, scheme.ErrType)
				}
				args[i] = s.Text()
			}
			flag := name != "directory-files"
			if paths == 1 && len(a) == 2 {
				b, ok := a[1].(scheme.Boolean)
				if !ok {
					return portFailure(c, name, a, ErrorOrdinary, scheme.ErrType)
				}
				flag = bool(b)
			}
			v, err := filesystemValue(c.Context(), name, args, flag)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return vm.Action{}, err
			}
			if err != nil {
				return portFailure(c, name, a, ErrorFile, err)
			}
			return vm.Return(v), nil
		})
		exports[name] = &binding{id: scheme.NewIdentifier(r.bootstrap.symbols.Intern(name)), form: variable, declared: true, cell: scheme.NewCell(procedure)}
	}
	ns, err := reader.ReadAll("<extension-libraries>", "(guri filesystem)", r.bootstrap.symbols)
	if err != nil {
		panic(err)
	}
	key, _, err := parseLibraryName(ns[0])
	if err != nil {
		panic(err)
	}
	r.modules[key] = &library{exports: exports, prepared: true, initialized: true}
}

func filesystemValue(ctx context.Context, name string, paths []string, flag bool) (scheme.Value, error) {
	path := paths[0]
	switch name {
	case "directory-files":
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		values := make([]scheme.Value, 0, len(entries))
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !flag && strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			s, err := scheme.NewString(entry.Name())
			if err != nil {
				return nil, fmt.Errorf("filename cannot be represented as a Scheme string: %w", err)
			}
			values = append(values, s)
		}
		return scheme.List(values...), nil
	case "create-directory":
		return scheme.Unspecified, os.Mkdir(path, 0775)
	case "delete-directory":
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("not a directory: %s", path)
		}
		// Unlike os.Remove, rmdir cannot delete a regular file if another process
		// replaces the checked directory before the removal operation.
		return scheme.Unspecified, os.NewSyscallError("rmdir", syscall.Rmdir(path))
	case "rename-file":
		return scheme.Unspecified, os.Rename(path, paths[1])
	case "file-directory?", "file-regular?":
		stat := os.Stat
		if !flag {
			stat = os.Lstat
		}
		info, err := stat(path)
		if os.IsNotExist(err) {
			return scheme.False, nil
		}
		if err != nil {
			return nil, err
		}
		if name == "file-directory?" {
			return scheme.Boolean(info.IsDir()), nil
		}
		return scheme.Boolean(info.Mode().IsRegular()), nil
	case "file-size":
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("not a regular file: %s", path)
		}
		return scheme.NewInteger(info.Size()), nil
	case "copy-file":
		return scheme.Unspecified, copyFilesystemFile(ctx, path, paths[1])
	default:
		return nil, fmt.Errorf("unknown filesystem operation: %s", name)
	}
}

func copyFilesystemFile(ctx context.Context, source, destination string) (err error) {
	// Check before opening so a named pipe or device is not opened as a source.
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", source)
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, in.Close()) }()
	info, err = in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", source)
	}
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, out.Close()) }()
	_, err = io.Copy(out, filesystemCopyReader{ctx, in})
	return err
}

type filesystemCopyReader struct {
	ctx  context.Context
	file *os.File
}

func (r filesystemCopyReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.file.Read(b)
}
