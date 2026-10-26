package ports

import (
	"errors"
	"io"
	"os"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/sys/windows"
)

func fileInputStream(f *os.File) (io.Reader, func() (bool, error)) {
	raw, err := f.SyscallConn()
	var mode uint32
	var modeErr error
	if err == nil {
		err = raw.Control(func(fd uintptr) { modeErr = windows.GetConsoleMode(windows.Handle(fd), &mode) })
	}
	if err == nil && modeErr == nil {
		input := &consoleFileInput{file: f}
		return input, input.Ready
	}
	return f, func() (bool, error) { return fileReady(f) }
}

// Keep UTF-16 conversion and completed-line buffering in this adapter rather
// than os.File's private console buffer. Ready must see every pending byte.
// Neither reading nor peeking changes the borrowed console's input mode.
type consoleFileInput struct {
	file        *os.File
	buffer      []byte
	high        uint16
	linePending bool
	err         error
}

func (in *consoleFileInput) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for len(in.buffer) == 0 && in.err == nil {
		in.fill()
	}
	if len(in.buffer) > 0 {
		n := copy(p, in.buffer)
		in.buffer = in.buffer[n:]
		return n, nil
	}
	return 0, in.err
}
func (in *consoleFileInput) fill() {
	raw, err := in.file.SyscallConn()
	if err != nil {
		in.err = err
		return
	}
	var mode uint32
	var data [1024]uint16
	var n uint32
	var readErr error
	err = raw.Control(func(fd uintptr) {
		h := windows.Handle(fd)
		readErr = windows.GetConsoleMode(h, &mode)
		if readErr == nil {
			readErr = windows.ReadConsole(h, &data[0], uint32(len(data)), &n, nil)
		}
	})
	in.err = errors.Join(err, readErr)
	if in.err != nil {
		return
	}
	if n == 0 {
		if in.high != 0 {
			in.buffer = utf8.AppendRune(in.buffer, utf8.RuneError)
			in.high = 0
		}
		in.err = io.EOF
		return
	}
	in.linePending = mode&windows.ENABLE_LINE_INPUT != 0
	for _, unit := range data[:n] {
		// Processed Enter expands to CRLF. A read ending at CR still has LF
		// pending in the console's cooked buffer, outside the event queue.
		if unit == '\n' || unit == '\r' && mode&windows.ENABLE_PROCESSED_INPUT == 0 {
			in.linePending = false
		}
		if unit == 0x1a { // Match os.File's Windows console Ctrl+Z convention.
			if in.high != 0 {
				in.buffer = utf8.AppendRune(in.buffer, utf8.RuneError)
				in.high = 0
			}
			in.err = io.EOF
			in.linePending = false
			break
		}
		if in.high != 0 {
			high := in.high
			in.high = 0
			if unit >= 0xDC00 && unit <= 0xDFFF {
				in.buffer = utf8.AppendRune(in.buffer, utf16.DecodeRune(rune(high), rune(unit)))
				continue
			}
			in.buffer = utf8.AppendRune(in.buffer, utf8.RuneError)
		}
		switch {
		case unit >= 0xD800 && unit <= 0xDBFF:
			in.high = unit
		case unit >= 0xDC00 && unit <= 0xDFFF:
			in.buffer = utf8.AppendRune(in.buffer, utf8.RuneError)
		default:
			in.buffer = utf8.AppendRune(in.buffer, rune(unit))
		}
	}
}
func (in *consoleFileInput) Ready() (bool, error) {
	if len(in.buffer) > 0 || in.err != nil || in.linePending {
		return true, nil
	}
	raw, err := in.file.SyscallConn()
	if err != nil {
		return false, err
	}
	var ready bool
	var probeErr error
	err = raw.Control(func(fd uintptr) {
		h := windows.Handle(fd)
		var mode uint32
		probeErr = windows.GetConsoleMode(h, &mode)
		if probeErr == nil {
			ready, probeErr = consoleReady(h, mode, in.high)
		}
	})
	return ready, errors.Join(err, probeErr)
}
