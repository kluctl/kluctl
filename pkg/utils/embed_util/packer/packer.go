package packer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/embed_util"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

func Pack(out string, dir string, patterns ...string) error {
	fileList, err := findFiles(dir, patterns)
	if err != nil {
		return err
	}

	if utils.Exists(out) && utils.Exists(out+".files") {
		existingFileListStr, err := ioutil.ReadFile(out + ".files")
		if err == nil {
			_, existingFileList, err := embed_util.ReadFileList(string(existingFileListStr))
			if err == nil {
				if reflect.DeepEqual(existingFileList, fileList) {
					log.Infof("Skipping packing of %s", out)
					return nil
				}
			}
		}
	}

	log.Infof("writing tar %s with %d files", out, len(fileList))
	tgz, err := writeTar(dir, fileList)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(tgz)
	err = ioutil.WriteFile(out, tgz, 0o600)
	if err != nil {
		return err
	}

	var fileList2 []string
	for f, l := range fileList {
		fileList2 = append(fileList2, fmt.Sprintf("%s: %d", f, l))
	}

	fileListStr := strings.Join(fileList2, "\n")
	fileListStr = hex.EncodeToString(hash[:]) + "\n" + fileListStr
	err = ioutil.WriteFile(out+".files", []byte(fileListStr), 0o600)
	return err
}

func findFiles(dir string, patterns []string) (map[string]int64, error) {
	var globs []glob.Glob
	for _, p := range patterns {
		globs = append(globs, glob.MustCompile(p, '/'))
	}

	var rootNames []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		match := false
		for _, p := range globs {
			if p.Match(strings.ReplaceAll(rel, string(os.PathSeparator), "/")) {
				match = true
				break
			}
		}
		if match {
			rootNames = append(rootNames, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	excludes := []glob.Glob{
		glob.MustCompile("__pycache__"),
		glob.MustCompile("**/__pycache__"),
		glob.MustCompile("**.a"),
		glob.MustCompile("**.pdb"),
		glob.MustCompile("**.pyc"),
	}

	fileList := make(map[string]int64)
	for _, d := range rootNames {
		err = filepath.Walk(filepath.Join(dir, d), func(path string, info fs.FileInfo, err error) error {
			if !info.Mode().IsRegular() && info.Mode().Type() != fs.ModeSymlink {
				return nil
			}

			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			for _, e := range excludes {
				if e.Match(relPath) {
					return nil
				}
			}
			fileList[relPath] = info.Size()
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return fileList, err
}

func writeTar(dir string, fileList map[string]int64) ([]byte, error) {
	b := bytes.NewBuffer(nil)
	gz := gzip.NewWriter(b)
	t := tar.NewWriter(gz)

	for f, _ := range fileList {
		err := utils.AddToTar(t, filepath.Join(dir, f), f, nil)
		if err != nil {
			return nil, err
		}
	}

	err := t.Close()
	if err != nil {
		return nil, err
	}
	err = gz.Close()
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
