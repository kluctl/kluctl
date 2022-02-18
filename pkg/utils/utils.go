package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/jinzhu/copier"
	"log"
	"os"
	"path"
	"sync"
)

var createTmpBaseDirOnce sync.Once

func GetTmpBaseDir() string {
	dir := path.Join(os.TempDir(), "kluctl")
	createTmpBaseDirOnce.Do(func() {
		err := os.MkdirAll(dir, 0o700)
		if err != nil {
			log.Fatal(err)
		}
	})
	return dir
}

func Sha256String(data string) string {
	h := sha256.Sum256([]byte(data))
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
