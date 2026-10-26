package ports

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var testConsoleDLL = windows.NewLazySystemDLL("kernel32.dll")

type testConsoleRecord struct {
	kind     uint16
	_        uint16
	down     int32
	repeat   uint16
	key      uint16
	scan     uint16
	char     uint16
	controls uint32
}

// AllocConsole is isolated in a child: no test may detach the runner's console
// or leave a borrowed standard handle/mode changed in the parent process.
func TestWindowsConsoleReadiness(t *testing.T) {
	if os.Getenv("GURI_TEST_OWN_CONSOLE") != "1" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, exe, "-test.run=^TestWindowsConsoleReadiness$", "-test.v")
		command.Env = append(os.Environ(), "GURI_TEST_OWN_CONSOLE=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("console child: %v\n%s", err, output)
		}
		t.Logf("isolated native console:\n%s", output)
		return
	}
	free := testConsoleDLL.NewProc("FreeConsole")
	if ok, _, err := free.Call(); ok == 0 && !errors.Is(err, windows.ERROR_INVALID_HANDLE) && !errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		t.Fatal(err)
	}
	if ok, _, err := testConsoleDLL.NewProc("AllocConsole").Call(); ok == 0 {
		t.Fatal("AllocConsole", err)
	}
	defer free.Call()
	name, err := windows.UTF16PtrFromString("CONIN$")
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	f := os.NewFile(uintptr(handle), "CONIN$")
	defer f.Close()
	setMode := func(t *testing.T, mode uint32) {
		t.Helper()
		if err := windows.SetConsoleMode(handle, mode); err != nil {
			t.Fatal(err)
		}
		if err := windows.FlushConsoleInputBuffer(handle); err != nil {
			t.Fatal(err)
		}
	}
	unchanged := func(t *testing.T, mode uint32) {
		t.Helper()
		var actual uint32
		if err := windows.GetConsoleMode(handle, &actual); err != nil || actual != mode {
			t.Fatal("borrowed console mode changed", actual, mode, err)
		}
	}
	writeRecords := func(t *testing.T, events []testConsoleRecord) {
		t.Helper()
		var n uint32
		ok, _, err := testConsoleDLL.NewProc("WriteConsoleInputW").Call(uintptr(handle), uintptr(unsafe.Pointer(&events[0])), uintptr(len(events)), uintptr(unsafe.Pointer(&n)))
		if ok == 0 || n != uint32(len(events)) {
			t.Fatal("WriteConsoleInputW", n, err)
		}
	}
	writeUnits := func(t *testing.T, units []uint16) {
		t.Helper()
		events := make([]testConsoleRecord, len(units))
		for i, ch := range units {
			events[i] = testConsoleRecord{kind: 1, down: 1, repeat: 1, char: ch}
			if ch == '\r' {
				events[i].key = 0x0D
			}
		}
		writeRecords(t, events)
	}
	writeText := func(t *testing.T, s string) { t.Helper(); writeUnits(t, utf16.Encode([]rune(s))) }
	check := func(t *testing.T, p *Resource, want bool) {
		t.Helper()
		got, err := p.Ready()
		if err != nil || got != want {
			t.Fatalf("ready=%t want=%t error=%v", got, want, err)
		}
	}
	read := func(t *testing.T, p *Resource, want rune) {
		t.Helper()
		check(t, p, true)
		got, err := p.ReadChar(false)
		if err != nil || got != want {
			t.Fatalf("char=%q want=%q error=%v", got, want, err)
		}
	}
	t.Run("cooked-line-and-events", func(t *testing.T) {
		mode := uint32(windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT)
		setMode(t, mode)
		p := FileInput(f, false, false)
		check(t, p, false)
		writeRecords(t, []testConsoleRecord{{kind: 0x10, down: 1}, {kind: 1, down: 0, repeat: 1, char: 'z'}})
		var before, after uint32
		if err := windows.GetNumberOfConsoleInputEvents(handle, &before); err != nil {
			t.Fatal(err)
		}
		check(t, p, false)
		check(t, p, false)
		if err := windows.GetNumberOfConsoleInputEvents(handle, &after); err != nil || before != after {
			t.Fatal("probe consumed events", before, after, err)
		}
		writeText(t, "a")
		check(t, p, false)
		writeText(t, "\r")
		read(t, p, 'a')
		read(t, p, '\r')
		read(t, p, '\n')
		check(t, p, false)
		unchanged(t, mode)
	})
	t.Run("raw-probe-and-unicode", func(t *testing.T) {
		setMode(t, 0)
		writeText(t, "A")
		for range 2 {
			if ready, err := fileReady(f); !ready || err != nil {
				t.Fatal(ready, err)
			}
		}
		var native [4]uint16
		var n uint32
		if err := windows.ReadConsole(handle, &native[0], uint32(len(native)), &n, nil); err != nil || n != 1 || native[0] != 'A' {
			t.Fatal("native input consumed", native, n, err)
		}
		p := FileInput(f, false, false)
		check(t, p, false)
		writeText(t, "猫")
		read(t, p, '猫')
		writeUnits(t, []uint16{0xD83D})
		check(t, p, false)
		writeUnits(t, []uint16{0xDE00})
		read(t, p, '😀')
		check(t, p, false)
		unchanged(t, 0)
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		unchanged(t, 0) // Closing the Scheme port must not close the console handle.
	})
	t.Run("long-cooked-line", func(t *testing.T) {
		mode := uint32(windows.ENABLE_LINE_INPUT)
		setMode(t, mode)
		p := FileInput(f, false, false)
		writeText(t, strings.Repeat("猫", 1500)+"\r")
		for range 1500 {
			read(t, p, '猫')
		}
		read(t, p, '\r')
		read(t, p, '\n')
		check(t, p, false)
		unchanged(t, mode)
	})
	t.Run("binary-console", func(t *testing.T) {
		setMode(t, 0)
		p := FileInput(f, true, false)
		writeText(t, "猫")
		for _, want := range []byte("猫") {
			check(t, p, true)
			got, err := p.ReadU8(false)
			if err != nil || got != want {
				t.Fatal(got, want, err)
			}
		}
		check(t, p, false)
		unchanged(t, 0)
	})
	t.Run("console-eof", func(t *testing.T) {
		mode := uint32(windows.ENABLE_LINE_INPUT)
		setMode(t, mode)
		p := FileInput(f, false, false)
		writeText(t, "\x1a")
		check(t, p, false)
		writeText(t, "\r")
		check(t, p, true)
		if _, err := p.ReadChar(false); !errors.Is(err, io.EOF) {
			t.Fatal("Ctrl+Z EOF", err)
		}
		unchanged(t, mode)
	})
}
