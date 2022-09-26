//go:build windows

package multiline

import (
	"fmt"
	"github.com/mattn/go-isatty"
	"io"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleCursorPosition   = kernel32.NewProc("SetConsoleCursorPosition")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterW")
)

// clear the line and move the cursor up
var clear = fmt.Sprintf("%c[%dA%c[2K\r", ESC, 0, ESC)

type short int16
type dword uint32
type word uint16

type coord struct {
	x short
	y short
}

type smallRect struct {
	left   short
	top    short
	right  short
	bottom short
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        word
	window            smallRect
	maximumWindowSize coord
}

// FdWriter is a writer with a file descriptor.
type FdWriter interface {
	io.Writer
	Fd() uintptr
}

func (ml *MultiLinePrinter) clearLines(lines int) {
	f, ok := ml.w.(FdWriter)
	if ok && !isatty.IsTerminal(f.Fd()) {
		ok = false
	}
	if !ok {
		_, _ = fmt.Fprint(ml.w, strings.Repeat(clear, lines))
		return
	}
	fd := f.Fd()
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info); err != nil {
		return
	}

	info.CursorPosition.Y -= int16(lines)
	if info.CursorPosition.Y < 0 {
		info.CursorPosition.Y = 0
	}
	_, _, _ = procSetConsoleCursorPosition.Call(
		uintptr(fd),
		uintptr(uint32(uint16(info.CursorPosition.Y))<<16|uint32(uint16(info.CursorPosition.X))),
	)

	// clear the lines
	cursor := &windows.Coord{
		X: info.Window.Left,
		Y: info.CursorPosition.Y,
	}
	count := uint32(info.Size.X) * uint32(lines)
	_, _, _ = procFillConsoleOutputCharacter.Call(
		uintptr(fd),
		uintptr(' '),
		uintptr(count),
		*(*uintptr)(unsafe.Pointer(cursor)),
		uintptr(unsafe.Pointer(new(uint32))),
	)
}
