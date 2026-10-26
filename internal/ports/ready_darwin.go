package ports

import (
	"errors"
	"golang.org/x/sys/unix"
)

func systemFileReady(fd int) (ready bool, err error) {
	// Unlike poll on macOS, EVFILT_READ supports terminals as well as pipes.
	queue, err := unix.Kqueue()
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, unix.Close(queue)) }()
	change := unix.Kevent_t{}
	unix.SetKevent(&change, fd, unix.EVFILT_READ, unix.EV_ADD|unix.EV_ONESHOT)
	events := make([]unix.Kevent_t, 1)
	for {
		n, e := unix.Kevent(queue, []unix.Kevent_t{change}, events, &unix.Timespec{})
		if errors.Is(e, unix.EINTR) {
			continue
		}
		if e != nil {
			return false, e
		}
		if n == 0 {
			return false, nil
		}
		if events[0].Flags&unix.EV_ERROR != 0 {
			return false, unix.Errno(events[0].Data)
		}
		return true, nil // Data, EOF and device errors all let read return immediately.
	}
}
