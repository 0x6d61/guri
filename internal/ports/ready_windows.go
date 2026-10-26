package ports

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	peekNamedPipe    = kernel32.NewProc("PeekNamedPipe")
	peekConsoleInput = kernel32.NewProc("PeekConsoleInputW")
)

// INPUT_RECORD's union occupies 16 bytes. KEY_EVENT_RECORD is the view used
// below; non-key events are inspected only through eventType.
type consoleInputRecord struct {
	eventType    uint16
	_            uint16
	keyDown      int32
	repeatCount  uint16
	virtualKey   uint16
	scanCode     uint16
	char         uint16
	controlState uint32
}

func fileReady(f *os.File) (bool, error) {
	raw, err := f.SyscallConn()
	if err != nil {
		return false, err
	}
	var ready bool
	var probeErr error
	err = raw.Control(func(fd uintptr) {
		h := windows.Handle(fd)
		var kind uint32
		kind, probeErr = windows.GetFileType(h)
		if probeErr != nil {
			return
		}
		switch kind {
		case windows.FILE_TYPE_DISK:
			ready = true
		case windows.FILE_TYPE_PIPE:
			var available uint32
			ok, _, e := peekNamedPipe.Call(fd, 0, 0, 0, uintptr(unsafe.Pointer(&available)), 0)
			if ok != 0 {
				ready = available > 0
				return
			}
			if errors.Is(e, windows.ERROR_BROKEN_PIPE) {
				ready = true
				return
			}
			probeErr = e
		case windows.FILE_TYPE_CHAR:
			var mode uint32
			probeErr = windows.GetConsoleMode(h, &mode)
			if probeErr == nil {
				ready, probeErr = consoleReady(h, mode, 0)
			}
		default:
			probeErr = ErrReadiness
		}
	})
	return ready, errors.Join(err, probeErr)
}

// https://learn.microsoft.com/en-us/windows/console/peekconsoleinput
// Peeking leaves both character and non-character events in the OS queue.
func consoleReady(h windows.Handle, mode uint32, high uint16) (bool, error) {
	var count uint32
	if err := windows.GetNumberOfConsoleInputEvents(h, &count); err != nil {
		return false, err
	}
	if count == 0 {
		return false, nil
	}
	events := make([]consoleInputRecord, count)
	var n uint32
	ok, _, err := peekConsoleInput.Call(uintptr(h), uintptr(unsafe.Pointer(&events[0])), uintptr(count), uintptr(unsafe.Pointer(&n)))
	if ok == 0 {
		return false, err
	}
	for _, event := range events[:n] {
		if event.eventType != 1 || event.keyDown == 0 || event.char == 0 {
			continue
		}
		ch := event.char
		if mode&windows.ENABLE_LINE_INPUT != 0 {
			if ch == '\r' {
				return true, nil
			}
			continue // Cooked ReadConsole waits for Enter, even if a key is queued.
		}
		if ch == 3 && mode&windows.ENABLE_PROCESSED_INPUT != 0 {
			continue
		}
		if high != 0 || ch < 0xD800 || ch > 0xDBFF {
			return true, nil
		}
		high = ch // A lone UTF-16 high surrogate is not a complete character.
	}
	return false, nil
}
