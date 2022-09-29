//go:build windows

package term

import (
	"golang.org/x/sys/windows"
)

// GetSize returns the visible dimensions of the given terminal.
//
// These dimensions don't include any scrollback buffer height.
func GetSize(fd int) (width, height int, err error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info); err != nil {
		return 0, 0, err
	}
	// terminal.GetSize from crypto/ssh adds "+ 1" to both width and height:
	// https://go.googlesource.com/crypto/+/refs/heads/release-branch.go1.14/ssh/terminal/util_windows.go#75
	// but looks like this is a root cause of issue #66, so removing both "+ 1" have fixed it.
	return int(info.Window.Right - info.Window.Left), int(info.Window.Bottom - info.Window.Top), nil
}
