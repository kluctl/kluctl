package jinja2

import (
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/rogpeppe/go-internal/lockedfile"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed python_src
var _pythonSrc embed.FS
var pythonSrc, _ = fs.Sub(_pythonSrc, "python_src")

var pythonSrcExtracted string

func init() {
	srcDir, err := extractSource()
	if err != nil {
		panic(err)
	}
	pythonSrcExtracted = srcDir
}

func extractSource() (string, error) {
	hash := calcEmbeddedHash(pythonSrc)

	targetPath := filepath.Join(utils.GetTmpBaseDir(), fmt.Sprintf("jinja2-%s", hash[:16]))

	lock, err := lockedfile.Create(targetPath + ".lock")
	if err != nil {
		return "", err
	}
	defer lock.Close()

	err = fs.WalkDir(pythonSrc, ".", func(path string, d fs.DirEntry, err error) error {
		if d == nil || d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(pythonSrc, path)
		if err != nil {
			return err
		}
		targetPath2 := filepath.Join(targetPath, path)
		err = os.MkdirAll(filepath.Dir(targetPath2), 0o755)
		if err != nil {
			return err
		}

		err = os.WriteFile(targetPath2+".tmp", data, 0o644)
		if err != nil {
			return err
		}
		err = os.Rename(targetPath2+".tmp", targetPath2)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return targetPath, nil
}

func calcEmbeddedHash(fs1 fs.FS) string {
	h := sha256.New()
	err := fs.WalkDir(fs1, ".", func(path string, d fs.DirEntry, err error) error {
		_ = binary.Write(h, binary.LittleEndian, path)
		if d.IsDir() {
			_ = binary.Write(h, binary.LittleEndian, "dir")
		} else {
			_ = binary.Write(h, binary.LittleEndian, "regular")
			data, err := fs.ReadFile(fs1, path)
			if err != nil {
				panic(err)
			}
			_ = binary.Write(h, binary.LittleEndian, len(data))
			_ = binary.Write(h, binary.LittleEndian, data)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
