//go:build !windows

package process

import (
	"bytes"
	"context"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
func KillProcess(pid int) error {
	return unix.Kill(pid, unix.SIGKILL)
}

func ListProcesses(ctx context.Context) ([]Process, error) {
	bin, err := exec.LookPath("ps")
	if err != nil {
		return nil, err
	}

	args := []string{"-ax", "-o", "pid,command"}

	cmd := exec.CommandContext(ctx, bin, args...)

	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf
	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(buf.String(), "\n")
	ret := make([]Process, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		s := strings.SplitN(l, " ", 2)
		pid, err := strconv.ParseInt(s[0], 10, 32)
		if err != nil {
			continue
		}
		ret = append(ret, Process{
			Pid:     int(pid),
			Command: s[1],
		})
	}
	return ret, nil
}
