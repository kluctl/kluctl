/*
Copyright 2021 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sourceignore

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
)

// LoadIgnorePatterns recursively loads the IgnoreFile patterns found
// in the directory.
func LoadIgnorePatterns(dir string, domain []string, ignoreFile string) ([]gitignore.Pattern, error) {
	// Make a copy of the domain so that the underlying string array of domain
	// in the gitignore patterns are unique without any side effects.
	dom := make([]string, len(domain))
	copy(dom, domain)

	ps, err := ReadIgnoreFile(filepath.Join(dir, ignoreFile), dom)
	if err != nil {
		return nil, err
	}
	fis, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, fi := range fis {
		if fi.IsDir() && fi.Name() != ".git" {
			var subps []gitignore.Pattern
			if subps, err = LoadIgnorePatterns(filepath.Join(dir, fi.Name()), append(dom, fi.Name()), ignoreFile); err != nil {
				return nil, err
			}
			if len(subps) > 0 {
				ps = append(ps, subps...)
			}
		}
	}
	return ps, nil
}

func ReadIgnoreFile(path string, domain []string) ([]gitignore.Pattern, error) {
	var ps []gitignore.Pattern
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		ps = append(ps, ReadPatterns(f, domain)...)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return ps, nil
}

func ReadPatterns(reader io.Reader, domain []string) []gitignore.Pattern {
	var ps []gitignore.Pattern
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		s := scanner.Text()
		if len(s) == 0 || strings.HasPrefix(s, "#") {
			continue
		}

		parsed := gitignore.ParsePattern(s, domain)
		ps = append(ps, parsed)
	}
	return ps
}
