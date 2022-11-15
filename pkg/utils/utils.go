package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/jinzhu/copier"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
)

var createTmpBaseDirOnce sync.Once
var tmpBaseDir string

func GetTmpBaseDir() string {
	createTmpBaseDirOnce.Do(func() {
		createTmpBaseDir()
	})
	return tmpBaseDir
}

func createTmpBaseDir() {
	envTmpDir := os.Getenv("KLUCTL_BASE_TMP_DIR")
	if envTmpDir != "" {
		tmpBaseDir = envTmpDir
		return
	}

	dir := filepath.Join(os.TempDir(), "kluctl-workdir")

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

	tmpBaseDir = dir
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

func Sha256String(data string) string {
	return Sha256Bytes([]byte(data))
}

func Sha256Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func DeepCopy(dst interface{}, src interface{}) error {
	return copier.CopyWithOption(dst, src, copier.Option{
		DeepCopy: true,
	})
}

func FindStrInSlice(a []string, s string) int {
	for i, v := range a {
		if v == s {
			return i
		}
	}
	return -1
}

func ParseBoolOrFalse(s *string) bool {
	if s == nil {
		return false
	}
	b, err := strconv.ParseBool(*s)
	if err != nil {
		return false
	}
	return b
}

func StrPtr(s string) *string {
	return &s
}
