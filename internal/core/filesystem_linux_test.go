package core_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

// Linux filesystems can expose arbitrary non-NUL filename bytes. APFS rejects
// this fixture at creation, and Windows filenames use UTF-16 instead.
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
