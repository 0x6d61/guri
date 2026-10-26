//go:build linux

package ports

import (
	"errors"
	"golang.org/x/sys/unix"
)

func systemFileReady(fd int) (bool, error) {
	p := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		_, err := unix.Poll(p, 0)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return false, err
		}
		if p[0].Revents&unix.POLLNVAL != 0 {
			return false, unix.EBADF
		}
		return p[0].Revents&(unix.POLLIN|unix.POLLERR|unix.POLLHUP) != 0, nil
	}
}
