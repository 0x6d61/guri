//go:build !linux

package ports

import "os"

func fileReady(f *os.File) (bool, error) {
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if info.Mode().IsRegular() {
		return true, nil
	}
	return false, ErrReadiness
}
