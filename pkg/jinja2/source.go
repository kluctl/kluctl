package jinja2

import (
	"archive/zip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	log "github.com/sirupsen/logrus"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed python_src
var pythonSrc embed.FS
var pythonSrcExtracted string

func init() {
	srcDir, err := extractSource()
	if err != nil {
		log.Panic(err)
	}
	pythonSrcExtracted = srcDir
}

func extractSource() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(pythonSrc, ".", func(path string, d fs.DirEntry, err error) error {
		h.Write([]byte(path))
		if !d.IsDir() {
			b, err := pythonSrc.ReadFile(path)
			if err != nil {
				log.Panic(err)
			}
			h.Write(b)
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	hash := hex.EncodeToString(h.Sum(nil))

	srcDir := filepath.Join(utils.GetTmpBaseDir(), fmt.Sprintf("jinja2-src-%s", hash[:16]))
	if utils.Exists(srcDir) {
		return srcDir, nil
	}
	srcDirTmp := srcDir + ".tmp"
	defer os.RemoveAll(srcDirTmp)

	err = utils.FsCopyDir(pythonSrc, "python_src", srcDirTmp)
	if err != nil {
		return "", err
	}

	err = filepath.WalkDir(filepath.Join(srcDirTmp, "wheel"), func(p string, d fs.DirEntry, err error) error {
		if !strings.HasSuffix(p, ".whl") {
			return nil
		}
		return unzipFile(p, srcDirTmp)
	})
	if err != nil {
		return "", err
	}

	err = os.Rename(srcDirTmp, srcDir)
	if err != nil && !os.IsExist(err) {
		return "", err
	}

	return srcDir, nil
}

func unzipFile(src string, target string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		filePath := filepath.Join(target, f.Name)
		if f.FileInfo().IsDir() {
			err = os.MkdirAll(filePath, 0o700)
			if err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		err = func() error {
			dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer dstFile.Close()

			fileInArchive, err := f.Open()
			if err != nil {
				return err
			}
			defer fileInArchive.Close()

			if _, err := io.Copy(dstFile, fileInArchive); err != nil {
				return err
			}
			return nil
		}()
	}
	return nil
}
