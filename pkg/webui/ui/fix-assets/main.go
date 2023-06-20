package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"k8s.io/apimachinery/pkg/util/rand"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type assetManifest struct {
	Files       map[string]string `json:"files"`
	Entrypoints []string          `json:"entrypoints"`
}

func main() {
	doError := func(err error) {
		if err == nil {
			return
		}
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}

	err := os.Chdir("build")
	doError(err)

	s, err := os.ReadFile("asset-manifest.json")
	doError(err)

	var manifest assetManifest
	err = json.Unmarshal(s, &manifest)
	doError(err)

	var sortedByLen []string
	for p1 := range manifest.Files {
		sortedByLen = append(sortedByLen, p1)
	}
	sort.Slice(sortedByLen, func(i, j int) bool {
		l1 := len(sortedByLen[i])
		l2 := len(sortedByLen[j])
		if l1 != l2 {
			return l2 < l1
		}
		return sortedByLen[i] < sortedByLen[j]
	})

	hashes := map[string]string{}
	for _, p1 := range sortedByLen {
		p2 := manifest.Files[p1]
		h := calcContentHash(p2)
		hashes[p1] = h
	}

	newEntries := map[string]string{}
	for _, p1 := range sortedByLen {
		p2 := manifest.Files[p1]
		oldName := path.Base(p2)
		newName := path.Base(p1)
		newName = strings.ReplaceAll(newName, ".dummy.", ".")
		oldPath := path.Join(path.Dir(p2), oldName)
		newPath := path.Join(path.Dir(p2), newName)

		if strings.Contains(p1, ".dummy.") {
			delete(manifest.Files, p1)
			newP1 := strings.ReplaceAll(p1, ".dummy.", ".")
			newEntries[newP1] = newPath
		} else {
			manifest.Files[p1] = newPath
		}

		contentHash := hashes[p1]
		err = replaceInFiles(".", oldName, newName+"?h="+contentHash)
		doError(err)

		err = os.Rename(oldPath, newPath)
		doError(err)

		for i, e := range manifest.Entrypoints {
			if path.Base(e) == oldName {
				manifest.Entrypoints[i] = path.Join(path.Dir(e), newName)
			}
		}
	}
	for k, v := range newEntries {
		manifest.Files[k] = v
	}

	s, err = json.MarshalIndent(&manifest, "", "  ")
	doError(err)
	err = os.WriteFile("asset-manifest.json", s, 0o600)
	doError(err)
}

func calcContentHash(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}

	if isText(p) {
		// replace CR LF \r\n (windows) with LF \n (unix)
		b = bytes.ReplaceAll(b, []byte{13, 10}, []byte{10})
		// replace CF \r (mac) with LF \n (unix)
		b = bytes.ReplaceAll(b, []byte{13}, []byte{10})
	}

	s := sha256.Sum256(b)
	ret := hex.EncodeToString(s[:])

	os.Stderr.WriteString(fmt.Sprintf("%s: %s\n", p, ret))

	return ret
}

func isText(p string) bool {
	mt := mime.TypeByExtension(path.Ext(p))
	if strings.HasPrefix(mt, "text/") {
		return true
	}
	if strings.Index(mt, "+xml") != -1 {
		return true
	}
	return false
}

func replaceInFiles(dir string, s string, r string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		f, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		dummy1 := rand.String(32)
		dummy2 := rand.String(32)

		f2 := string(f)

		f2 = strings.ReplaceAll(f2, "f="+s, dummy1)
		f2 = strings.ReplaceAll(f2, s+".map", dummy2)

		f2 = strings.ReplaceAll(f2, s, r)

		f2 = strings.ReplaceAll(f2, dummy1, "f="+s)
		f2 = strings.ReplaceAll(f2, dummy2, s+".map")

		if strings.Compare(string(f), f2) == 0 {
			return nil
		}

		err = os.WriteFile(path, []byte(f2), 0o600)
		if err != nil {
			return err
		}
		return nil
	})
}
