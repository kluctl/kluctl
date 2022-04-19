package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	out := os.Args[1]
	dir := os.Args[2]
	patterns := os.Args[3:]

	fileList, tgz := writeTar(dir, patterns)

	hash := sha256.Sum256(tgz)
	err := ioutil.WriteFile(out, tgz, 0o600)
	if err != nil {
		log.Panic(err)
	}

	fileListStr := strings.Join(fileList, "\n")
	fileListStr = hex.EncodeToString(hash[:]) + "\n" + fileListStr
	err = ioutil.WriteFile(out+".files", []byte(fileListStr), 0o600)
	if err != nil {
		log.Panic(err)
	}
}

func writeTar(dir string, patterns []string) ([]string, []byte) {
	b := bytes.NewBuffer(nil)
	gz := gzip.NewWriter(b)
	t := tar.NewWriter(gz)

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
			if p.Match(rel) {
				match = true
				break
			}
		}
		if match {
			rootNames = append(rootNames, rel)
		}
		return nil
	})
	sort.Strings(rootNames)

	excludes := []glob.Glob{
		glob.MustCompile("__pycache__"),
		glob.MustCompile("**/__pycache__"),
		glob.MustCompile("**.a"),
		glob.MustCompile("**.pdb"),
	}

	var fileList []string

	for _, d := range rootNames {
		err := utils.AddToTar(t, filepath.Join(dir, d), d, func(h *tar.Header, size int64) (*tar.Header, error) {
			for _, e := range excludes {
				if e.Match(h.Name) {
					return nil, nil
				}
			}

			hashStr, err := utils.HashTarEntry(dir, h.Name)
			if err != nil {
				return nil, err
			}

			fileList = append(fileList, fmt.Sprintf("%s: %d %s", h.Name, size, hashStr))
			return h, nil
		})
		if err != nil {
			log.Panic(err)
		}
	}

	err = t.Close()
	if err != nil {
		log.Panic(err)
	}
	err = gz.Close()
	if err != nil {
		log.Panic(err)
	}

	return fileList, b.Bytes()
}
