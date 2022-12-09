package utils

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"
)

type tmpBaseDirKey int
type tmpBaseDirValue struct {
	dir      string
	initOnce sync.Once
}

var tmpBaseDirKeyInst tmpBaseDirKey
var tmpBaseDirValueDefault = &tmpBaseDirValue{
	dir: getDefaultTmpBaseDir(),
}

func WithTmpBaseDir(ctx context.Context, tmpBaseDir string) context.Context {
	return context.WithValue(ctx, tmpBaseDirKeyInst, &tmpBaseDirValue{
		dir: tmpBaseDir,
	})
}

func GetTmpBaseDir(ctx context.Context) string {
	v := ctx.Value(tmpBaseDirKeyInst)
	if v == nil {
		v = tmpBaseDirValueDefault
	}
	v2 := v.(*tmpBaseDirValue)
	v2.initOnce.Do(func() {
		v2.dir = createTmpBaseDir(v2.dir)
	})
	return v2.dir
}

func getDefaultTmpBaseDir() string {
	dir := os.Getenv("KLUCTL_BASE_TMP_DIR")
	if dir != "" {
		return dir
	}

	return filepath.Join(os.TempDir(), "kluctl-workdir")
}

func createTmpBaseDir(dir string) string {
	ensureDir(dir, 0o777, true)

	// every user gets its own tmp dir
	if runtime.GOOS == "windows" {
		u, err := user.Current()
		if err != nil {
			panic(err)
		}
		dir = filepath.Join(dir, u.Uid)
	} else {
		uid := os.Getuid()
		dir = filepath.Join(dir, fmt.Sprintf("%d", uid))
	}

	ensureDir(dir, 0o700, true)
	return dir
}

func ensureDir(path string, perm fs.FileMode, allowCreate bool) {
	if Exists(path) {
		st, err := os.Lstat(path)
		if err != nil {
			panic(err)
		}
		if !st.IsDir() {
			panic(fmt.Sprintf("%s is not a directory", path))
		}
		if st.Mode().Perm() != perm {
			err = os.Chmod(path, perm)
			if err != nil {
				panic(err)
			}
		}
	} else if !allowCreate {
		panic(fmt.Sprintf("failed to ensure directory %s", path))
	} else {
		err := os.Mkdir(path, perm)
		if err != nil {
			if os.IsExist(err) {
				ensureDir(path, perm, false)
			} else {
				panic(err)
			}
		}
	}
}
