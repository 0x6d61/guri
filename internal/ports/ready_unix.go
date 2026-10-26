//go:build linux || darwin

package ports

import (
	"errors"
	"os"
)

func fileReady(f *os.File) (bool, error) {
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	// macOS poll does not support ordinary files; those never wait for input.
	if info.Mode().IsRegular() {
		return true, nil
	}
	raw, err := f.SyscallConn()
	if err != nil {
		return false, err
	}
	var ready bool
	var probeErr error
	err = raw.Control(func(fd uintptr) { ready, probeErr = systemFileReady(int(fd)) })
	return ready, errors.Join(err, probeErr)
}
