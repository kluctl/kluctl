package e2e

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
)

func runWrappedCmd(testName string, cwd string, env []string, args []string) (string, string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", "", err
	}

	var args2 []string
	args2 = append(args2, "-test.run", testName)
	for _, a := range args {
		args2 = append(args2, "-karg", a)
	}

	cmd := exec.Command(executable, args2...)
	cmd.Env = env
	cmd.Dir = cwd

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdoutPipe.Close()
		return "", "", err
	}

	stdReader := func(std *os.File, buf io.StringWriter, pipe io.Reader) {
		scanner := bufio.NewScanner(pipe)
		inMarker := false
		for scanner.Scan() {
			l := scanner.Text()
			if !inMarker {
				if l == stdoutStartMarker {
					inMarker = true
					continue
				}
				_, _ = std.WriteString(l + "\n")
			} else {
				if l == stdoutEndMarker {
					inMarker = false
					continue
				}

				l += "\n"
				_, _ = std.WriteString(l)
				_, _ = buf.WriteString(l)
			}
		}
	}

	stdoutBuf := bytes.NewBuffer(nil)
	stderrBuf := bytes.NewBuffer(nil)

	go stdReader(os.Stdout, stdoutBuf, stdoutPipe)
	go stdReader(os.Stderr, stderrBuf, stderrPipe)

	err = cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}
