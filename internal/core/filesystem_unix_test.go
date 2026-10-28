//go:build linux || darwin

package core_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestFilesystemUnrepresentableFilename(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad\xff"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	src := `(import (scheme base) (guri filesystem)) (guard (e (else (file-error? e))) (directory-files ` + strconv.Quote(root) + ` #t) #f)`
	if got := runProgram(t, core.NewRuntime(), src); got != scheme.True {
		t.Fatal(got)
	}
}

func TestFilesystemPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses Unix mode permissions; run as an unprivileged user")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("secret"), 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(source, 0600); err != nil {
			t.Error(err)
		}
	})
	destination := filepath.Join(root, "copy")
	src := `(import (scheme base) (guri filesystem)) (guard (e (else (file-error? e))) (copy-file ` + strconv.Quote(source) + ` ` + strconv.Quote(destination) + `) #f)`
	if got := runProgram(t, core.NewRuntime(), src); got != scheme.True {
		t.Fatal(got)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
