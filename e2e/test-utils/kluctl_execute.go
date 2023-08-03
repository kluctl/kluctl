package test_utils

import (
	"bytes"
	"context"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/commands"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"strings"
	"sync"
	"testing"
)

func KluctlExecute(t *testing.T, ctx context.Context, args ...string) (string, string, error) {
	t.Logf("Runnning kluctl: %s", strings.Join(args, " "))

	var m sync.Mutex
	stdoutBuf := bytes.NewBuffer(nil)
	stdout := status.NewLineRedirector(func(line string) {
		m.Lock()
		defer m.Unlock()
		t.Log(line)
		stdoutBuf.WriteString(line + "\n")
	})
	stderrBuf := bytes.NewBuffer(nil)

	ctx = utils.WithTmpBaseDir(ctx, t.TempDir())
	ctx = commands.WithStdStreams(ctx, stdout, stderrBuf)
	sh := status.NewSimpleStatusHandler(func(message string) {
		m.Lock()
		defer m.Unlock()
		t.Log(message)
		stderrBuf.WriteString(message + "\n")
	}, false, true)
	defer func() {
		if sh != nil {
			sh.Stop()
		}
	}()
	ctx = status.NewContext(ctx, sh)
	err := commands.Execute(ctx, args, nil)
	sh.Stop()
	sh = nil
	_ = stdout.Close()

	m.Lock()
	defer m.Unlock()
	return stdoutBuf.String(), stderrBuf.String(), err
}
