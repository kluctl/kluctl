//go:build windows

package process

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

var ProcAttrWithProcessGroup = &windows.SysProcAttr{
	CreationFlags: windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NEW_PROCESS_GROUP,
}

func TerminateProcess(pid int, _ os.Signal) error {
	if pid == 0 || pid == -1 {
		return nil
	}
	dll, err := windows.LoadDLL("kernel32.dll")
	if err != nil {
		return err
	}
	defer dll.Release()

	f, err := dll.FindProc("AttachConsole")
	if err != nil {
		return err
	}
	r1, _, err := f.Call(uintptr(pid))
	if r1 == 0 && err != syscall.ERROR_ACCESS_DENIED {
		return err
	}

	f, err = dll.FindProc("SetConsoleCtrlHandler")
	if err != nil {
		return err
	}
	r1, _, err = f.Call(0, 1)
	if r1 == 0 {
		return err
	}
	f, err = dll.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return err
	}
	r1, _, err = f.Call(windows.CTRL_BREAK_EVENT, uintptr(pid))
	if r1 == 0 {
		return err
	}
	r1, _, err = f.Call(windows.CTRL_C_EVENT, uintptr(pid))
	if r1 == 0 {
		return err
	}
	return nil
}

func KillProcess(pid int) error {
	ps := os.Process{
		Pid: pid,
	}
	return ps.Kill()
}

func ListProcesses(ctx context.Context) ([]Process, error) {
	return nil, fmt.Errorf("not implemented on windows")
}
