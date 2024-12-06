package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/kluctl/kluctl/lib/git/sourceignore"
	"github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/lib/status"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type CheckoutInfo struct {
	CheckedOutRef    types.GitRef `json:"checkedOutRef"`
	CheckedOutCommit string       `json:"checkedOutCommit"`
}

func DetectGitRepositoryRoot(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for true {
		st, err := os.Stat(filepath.Join(path, ".git"))
		if err == nil && st.IsDir() {
			break
		}
		old := path
		path = filepath.Dir(path)
		if old == path {
			return "", fmt.Errorf("could not detect git repository root")
		}
	}
	return path, nil
}

// GetWorktreeStatus returns the worktree status of the given repo
// When modified files are found, it will invoke the git binary for "git status --porcelain" to check that things have
// really changed. This is required because go-git in not properly handling CRLF: https://github.com/go-git/go-git/issues/594
func GetWorktreeStatus(ctx context.Context, path string) (git.Status, error) {
	g, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	wt, err := g.Worktree()
	if err != nil {
		return nil, err
	}
	gitStatus, err := wt.Status()
	if err != nil {
		return nil, err
	}

	isModified := false
	for _, s := range gitStatus {
		if s.Worktree == git.Modified {
			isModified = true
			break
		}
	}
	if !isModified {
		return gitStatus, nil
	}

	status.Trace(ctx, "Running git status --porcelain to verify CRLF issues are not what cause the dirty status")

	commandPath, err := exec.LookPath("git")
	if err != nil {
		status.Tracef(ctx, "Failed to lookup git binary: %v", err)
		return nil, err
	}

	out := bytes.NewBuffer(nil)

	cmd := &exec.Cmd{Path: commandPath, Dir: path, Env: os.Environ(), Args: []string{"git", "status", "--porcelain"}, Stdout: out, Stderr: out}
	err = cmd.Run()
	if err != nil {
		status.Tracef(ctx, "Failed to run git status --porcelain: err=%v, out=%s", err, out.String())
		return gitStatus, nil
	}

	parsedStatus, err := parsePorcelainStatus(out.String())
	if err != nil {
		status.Tracef(ctx, "Failed to parse output of git status --porcelain: %v", err)
		return gitStatus, err
	}

	return parsedStatus, nil
}

func parsePorcelainStatus(out string) (git.Status, error) {
	s := bufio.NewScanner(strings.NewReader(out))

	ret := git.Status{}
	for s.Scan() {
		if len(s.Text()) < 1 {
			continue
		}
		line := s.Text()
		var fs git.FileStatus
		var path string
		_, err := fmt.Sscanf(line, "%c%c %s", &fs.Staging, &fs.Worktree, &path)
		if err != nil {
			return nil, err
		}
		ret[path] = &fs
	}
	return ret, nil
}

func LoadGitignore(p string) ([]gitignore.Pattern, error) {
	p = filepath.Clean(p)
	domain := strings.Split(p, string(filepath.Separator))
	ignorePatterns, err := sourceignore.LoadIgnorePatterns(p, domain, ".gitignore")
	if err != nil {
		return nil, err
	}
	ignorePatterns = append(ignorePatterns, sourceignore.ReadPatterns(strings.NewReader(".git"), domain)...)
	return ignorePatterns, nil
}

func RunWithDeadlineAndPanic(ctx context.Context, extraDeadline time.Duration, f func() error) error {
	deadline, hasDeadline := ctx.Deadline()

	if !hasDeadline {
		return f()
	}

	var finished atomic.Bool

	wait := deadline.Sub(time.Now()) + extraDeadline
	if wait < 0 {
		return ctx.Err()
	}

	deadlineErr := errors.New(fmt.Errorf("deadline exceeded while calling function"))

	t := time.AfterFunc(wait, func() {
		if !finished.Load() {
			panic(deadlineErr.ErrorStack())
		}
	})
	defer t.Stop()

	err := f()
	finished.Store(true)

	return err
}
