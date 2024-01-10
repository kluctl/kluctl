package utils

import (
	"context"
	"fmt"
	"io/fs"
	"k8s.io/client-go/util/homedir"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"
)

type tmpBaseDirKey struct{}
type cacheDirKey struct{}

type dirValue struct {
	dir      string
	initOnce sync.Once
}

var tmpBaseDirValueDefault = &dirValue{
	dir: getDefaultTmpBaseDir(),
}

func WithTmpBaseDir(ctx context.Context, tmpBaseDir string) context.Context {
	return context.WithValue(ctx, &tmpBaseDirKey{}, &dirValue{
		dir: tmpBaseDir,
	})
}

func WithCacheDir(ctx context.Context, cacheDir string) context.Context {
	return context.WithValue(ctx, cacheDirKey{}, &dirValue{
		dir: cacheDir,
	})
}

func GetTmpBaseDir(ctx context.Context) string {
	v := ctx.Value(tmpBaseDirKey{})
	if v == nil {
		v = tmpBaseDirValueDefault
	}
	v2 := v.(*dirValue)
	v2.initOnce.Do(func() {
		v2.dir = createTmpBaseDir(v2.dir)
	})
	return v2.dir
}

func GetCacheDir(ctx context.Context) string {
	v := ctx.Value(cacheDirKey{})
	var dir string
	if v == nil {
		dir = getDefaultCacheDir(ctx)
	} else {
		v2 := v.(*dirValue)
		dir = v2.dir
	}
	ensureDir(dir, 0o700, true)
	return dir
}

func getDefaultTmpBaseDir() string {
	dir := os.Getenv("KLUCTL_BASE_TMP_DIR")
	if dir != "" {
		return dir
	}

	return filepath.Join(os.TempDir(), "kluctl-workdir")
}

func getDefaultCacheDir(ctx context.Context) string {
	p := os.Getenv("KLUCTL_CACHE_DIR")
	if p != "" {
		return p
	}
	p = os.Getenv("XDG_CACHE_HOME")
	if p != "" {
		return filepath.Join(p, "kluctl")
	}
	h := homedir.HomeDir()
	switch runtime.GOOS {
	case "linux":
		if h != "" {
			return filepath.Join(h, ".cache", "kluctl")
		}
	case "darwin":
		if h != "" {
			return filepath.Join(h, "Library", "Caches", "kluctl")
		}
	case "windows":
		break
	}
	return filepath.Join(GetTmpBaseDir(ctx), "cache")
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
