package utils

import (
	"context"
	"os"
	"runtime"
	"testing"
)

func TestCreateSharedTmpBaseDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix permission bits")
	}
	t.Setenv("TMPDIR", t.TempDir())
	base := sharedTmpBaseDir()
	userDir := createTmpBaseDir(base)

	checkMode := func(path string, want os.FileMode) {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s: mode %04o, want %04o", path, got, want)
		}
	}
	checkMode(base, 0o777)
	checkMode(userDir, 0o700)

	if err := os.Chmod(base, 0o755); err != nil {
		t.Fatal(err)
	}
	createTmpBaseDir(base)
	checkMode(base, 0o755)
}

func TestWithTmpBaseDir(t *testing.T) {
	dir := t.TempDir()
	ctx := WithTmpBaseDir(context.Background(), dir)
	if got := GetTmpBaseDirNoDefault(ctx); got != dir {
		t.Errorf("GetTmpBaseDirNoDefault() = %q, want %q", got, dir)
	}
	if got := GetTmpBaseDir(ctx); got != createTmpBaseDir(dir) {
		t.Errorf("GetTmpBaseDir() = %q, want a dir below %q", got, dir)
	}
	if got := GetTmpBaseDirNoDefault(context.Background()); got != "" {
		t.Errorf("GetTmpBaseDirNoDefault() without override = %q, want empty", got)
	}
}
