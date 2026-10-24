//go:build linux

package ports

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

func fileReady(f *os.File) (bool, error) {
	raw, err := f.SyscallConn()
	if err != nil {
		return false, err
	}
	var ready bool
	var probeErr error
	err = raw.Control(func(fd uintptr) {
		// ppoll(2) works across Linux architectures, supports descriptors beyond
		// FD_SETSIZE, and reports pipe EOF/HUP. A zero timeout never waits.
		p := struct {
			fd              int32
			events, revents int16
		}{fd: int32(fd), events: 1}
		for {
			timeout := syscall.Timespec{}
			_, _, errno := syscall.Syscall6(syscall.SYS_PPOLL, uintptr(unsafe.Pointer(&p)), 1, uintptr(unsafe.Pointer(&timeout)), 0, 0, 0)
			if errno == syscall.EINTR {
				continue
			}
			if errno != 0 {
				probeErr = errno
				return
			}
			if p.revents&0x20 != 0 {
				probeErr = syscall.EBADF
				return
			}
			ready = p.revents&(1|8|16) != 0
			return
		}
	})
	return ready, errors.Join(err, probeErr)
}
