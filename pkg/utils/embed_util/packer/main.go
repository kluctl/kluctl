package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/gobwas/glob"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main()  {
	out := os.Args[1]
	dir := os.Args[2]
	pattern := os.Args[3]

	fileList, tgz := writeTar(dir, pattern)

	hash := sha256.Sum256(tgz)
	err := ioutil.WriteFile(out, tgz, 0o600)
	if err != nil {
		log.Panic(err)
	}

	fileListStr := strings.Join(fileList, "\n")
	fileListStr = hex.EncodeToString(hash[:]) + "\n" + fileListStr
	err = ioutil.WriteFile(out + ".files", []byte(fileListStr), 0o600)
	if err != nil {
		log.Panic(err)
	}
}

func writeTar(dir string, pattern string) ([]string, []byte) {
	b := bytes.NewBuffer(nil)
	gz := gzip.NewWriter(b)
	t := tar.NewWriter(gz)

	var rootNames []string
	dirs, err := os.ReadDir(dir)
	if err != nil {
		log.Panic(err)
	}
	p := glob.MustCompile(pattern)
	for _, d := range dirs {
		if p.Match(d.Name()) {
			rootNames = append(rootNames, d.Name())
		}
	}
	sort.Strings(rootNames)

	excludes := []glob.Glob{
		glob.MustCompile("__pycache__"),
		glob.MustCompile("**/__pycache__"),
		glob.MustCompile("**.a"),
	}

	var fileList []string

	for _, d := range rootNames {
		err := utils.AddToTar(t, filepath.Join(dir, d), d, func(h *tar.Header) (*tar.Header, error) {
			for _, e := range excludes {
				if e.Match(h.Name) {
					return nil, nil
				}
			}

			s := h.Size
			if h.FileInfo().IsDir() {
				s = 0
			}
			fileList = append(fileList, fmt.Sprintf("%s: %d", h.Name, s))
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
