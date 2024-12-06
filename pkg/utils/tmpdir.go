package utils

import (
	"context"
	"fmt"
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

func GetTmpBaseDirNoDefault(ctx context.Context) string {
	v := ctx.Value(tmpBaseDirKey{})
	if v == nil {
		return ""
	}
	v2 := v.(*dirValue)
	return v2.dir
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

func GetCacheDirNoDefault(ctx context.Context) string {
	v := ctx.Value(cacheDirKey{})
	if v == nil {
		return ""
	} else {
		v2 := v.(*dirValue)
		return v2.dir
	}
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
	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		panic(err)
	}
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
	// all users can access the parent dir
	err := os.MkdirAll(dir, 0o777)
	if err != nil {
		panic(err)
	}

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

	// only current user can access the actual tmp dir
	err = os.Mkdir(dir, 0o700)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	return dir
}
