package test_project

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"
	"sync"
	"testing"
)

func runHelper(t *testing.T, cmd *exec.Cmd) (string, string, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdoutPipe.Close()
		return "", "", err
	}

	var wg sync.WaitGroup
	stdReader := func(testLogPrefix string, buf io.StringWriter, pipe io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			l := scanner.Text()
			t.Log(testLogPrefix + l)
			_, _ = buf.WriteString(l + "\n")
		}
	}

	stdoutBuf := bytes.NewBuffer(nil)
	stderrBuf := bytes.NewBuffer(nil)

	wg.Add(2)
	go stdReader("stdout: ", stdoutBuf, stdoutPipe)
	go stdReader("stderr: ", stderrBuf, stderrPipe)

	err = cmd.Start()
	if err != nil {
		return "", "", err
	}
	wg.Wait()
	err = cmd.Wait()
	return stdoutBuf.String(), stderrBuf.String(), err
}
