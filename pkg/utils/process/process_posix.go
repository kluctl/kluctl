// +build !windows

package process

import (
	"golang.org/x/sys/unix"
	"os"
)

var ProcAttrWithProcessGroup = &unix.SysProcAttr{Setpgid: true}

func TerminateProcess(pid int, signal os.Signal) error {
	if pid == 0 || pid == -1 {
		return nil
	}
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		return err
	}

	if pgid == pid {
		pid = -1 * pid
	}

	target, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return target.Signal(signal)
}

// kills the process with pid pid, as well as its children.
func KillProcess(process *os.Process) error {
	return unix.Kill(-1*process.Pid, unix.SIGKILL)
}
