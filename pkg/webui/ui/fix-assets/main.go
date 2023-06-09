package main

import (
	"encoding/json"
	"io/fs"
	"k8s.io/apimachinery/pkg/util/rand"
	"os"
	"path"
	"path/filepath"
	"regexp"
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

	hashRegex := regexp.MustCompile(`([a-z-0-9-_]*)(\.[a-f0-9]*)(\.chunk)?\.(js|css)(\.map)?`)

	newEntries := map[string]string{}
	for p1, p2 := range manifest.Files {
		oldName := path.Base(p2)
		newName := path.Base(p1)
		oldPath := path.Join(path.Dir(p2), oldName)
		newPath := path.Join(path.Dir(p2), newName)

		m := hashRegex.FindSubmatch([]byte(newName))
		if m != nil {
			s := strings.Split(newName, ".")
			// get rid of hash
			s = append(s[0:1], s[2:]...)
			newName = strings.Join(s, ".")
			newPath = path.Join(path.Dir(p2), newName)

			delete(manifest.Files, p1)
			newEntries[newName] = newPath
		} else {
			manifest.Files[p1] = newPath
		}

		// we assume that .map files are implicitly replaced by the non-.map versions
		if !strings.HasSuffix(oldName, ".map") {
			err = replaceInFiles(".", oldName, newName+"?f="+oldName)
			doError(err)
		}

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
