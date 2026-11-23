package utils

import (
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
