package jinja2

import (
	"archive/zip"
	"embed"
	"github.com/codablock/kluctl/pkg/utils"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

//go:embed python_src
var pythonSrc embed.FS

func extractSource() (string, error) {
	tmpDir, err := ioutil.TempDir(utils.GetTmpBaseDir(), "jinja2-src-")
	if err != nil {
		return "", err
	}

	err = utils.FsCopyDir(pythonSrc, "python_src", tmpDir)
	if err != nil {
		return "", err
	}

	err = filepath.WalkDir(filepath.Join(tmpDir, "wheel"), func(p string, d fs.DirEntry, err error) error {
		if !strings.HasSuffix(p, ".whl") {
			return nil
		}
		r, err := zip.OpenReader(p)
		if err != nil {
			return err
		}
		defer r.Close()

		for _, f := range r.File {
			filePath := filepath.Join(tmpDir, f.Name)
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
	})
	if err != nil {
		return "", err
	}

	return tmpDir, nil
}
