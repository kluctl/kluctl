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

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

const (
	IgnoreFile   = ".sourceignore"
	ExcludeVCS   = ".git/,.gitignore,.gitmodules,.gitattributes"
	ExcludeExt   = "*.jpg,*.jpeg,*.gif,*.png,*.wmv,*.flv,*.tar.gz,*.zip"
	ExcludeCI    = ".github/,.circleci/,.travis.yml,.gitlab-ci.yml,appveyor.yml,.drone.yml,cloudbuild.yaml,codeship-services.yml,codeship-steps.yml"
	ExcludeExtra = "**/.goreleaser.yml,**/.sops.yaml,**/.flux.yaml"
)

// NewMatcher returns a gitignore.Matcher for the given gitignore.Pattern
// slice. It mainly exists to compliment the API.
func NewMatcher(ps []gitignore.Pattern) gitignore.Matcher {
	return gitignore.NewMatcher(ps)
}

// NewDefaultMatcher returns a gitignore.Matcher with the DefaultPatterns
// as lowest priority patterns.
func NewDefaultMatcher(ps []gitignore.Pattern, domain []string) gitignore.Matcher {
	var defaultPs []gitignore.Pattern
	defaultPs = append(defaultPs, VCSPatterns(domain)...)
	defaultPs = append(defaultPs, DefaultPatterns(domain)...)
	ps = append(defaultPs, ps...)
	return gitignore.NewMatcher(ps)
}

// VCSPatterns returns a gitignore.Pattern slice with ExcludeVCS
// patterns.
func VCSPatterns(domain []string) []gitignore.Pattern {
	var ps []gitignore.Pattern
	for _, p := range strings.Split(ExcludeVCS, ",") {
		ps = append(ps, gitignore.ParsePattern(p, domain))
	}
	return ps
}

// DefaultPatterns returns a gitignore.Pattern slice with the default
// ExcludeExt, ExcludeCI, ExcludeExtra patterns.
func DefaultPatterns(domain []string) []gitignore.Pattern {
	all := strings.Join([]string{ExcludeExt, ExcludeCI, ExcludeExtra}, ",")
	var ps []gitignore.Pattern
	for _, p := range strings.Split(all, ",") {
		ps = append(ps, gitignore.ParsePattern(p, domain))
	}
	return ps
}

// ReadPatterns collects ignore patterns from the given reader and
// returns them as a gitignore.Pattern slice.
// If a domain is supplied, this is used as the scope of the read
// patterns.
func ReadPatterns(reader io.Reader, domain []string) []gitignore.Pattern {
	var ps []gitignore.Pattern
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		s := scanner.Text()
		if !strings.HasPrefix(s, "#") && len(strings.TrimSpace(s)) > 0 {
			ps = append(ps, gitignore.ParsePattern(s, domain))
		}
	}
	return ps
}

// ReadIgnoreFile attempts to read the file at the given path and
// returns the read patterns.
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

// LoadIgnorePatterns recursively loads the IgnoreFile patterns found
// in the directory.
func LoadIgnorePatterns(dir string, domain []string) ([]gitignore.Pattern, error) {
	// Make a copy of the domain so that the underlying string array of domain
	// in the gitignore patterns are unique without any side effects.
	dom := make([]string, len(domain))
	copy(dom, domain)

	ps, err := ReadIgnoreFile(filepath.Join(dir, IgnoreFile), dom)
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
			if subps, err = LoadIgnorePatterns(filepath.Join(dir, fi.Name()), append(dom, fi.Name())); err != nil {
				return nil, err
			}
			if len(subps) > 0 {
				ps = append(ps, subps...)
			}
		}
	}
	return ps, nil
}
