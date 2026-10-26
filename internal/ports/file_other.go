//go:build !windows

package ports

import (
	"io"
	"os"
)

func fileInputStream(f *os.File) (io.Reader, func() (bool, error)) {
	return f, func() (bool, error) { return fileReady(f) }
}
