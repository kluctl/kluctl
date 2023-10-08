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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"gotest.tools/assert"
)

func TestReadPatterns(t *testing.T) {
	tests := []struct {
		name       string
		ignore     string
		domain     []string
		matches    []string
		mismatches []string
	}{
		{
			name: "simple",
			ignore: `ignore-dir/*
!ignore-dir/include
`,
			matches:    []string{"ignore-dir/file.yaml"},
			mismatches: []string{"file.yaml", "ignore-dir/include"},
		},
		{
			name: "with comments",
			ignore: `ignore-dir/*
# !ignore-dir/include`,
			matches: []string{"ignore-dir/file.yaml", "ignore-dir/include"},
		},
		{
			name:       "domain scoped",
			domain:     []string{"domain", "scoped"},
			ignore:     "ignore-dir/*",
			matches:    []string{"domain/scoped/ignore-dir/file.yaml"},
			mismatches: []string{"ignore-dir/file.yaml"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.ignore)
			ps := ReadPatterns(reader, tt.domain)
			matcher := NewMatcher(ps)
			for _, m := range tt.matches {
				assert.Equal(t, matcher.Match(strings.Split(m, "/"), false), true, "expected %s to match", m)
			}
			for _, m := range tt.mismatches {
				assert.Equal(t, matcher.Match(strings.Split(m, "/"), false), false, "expected %s to not match", m)
			}
		})
	}
}

func TestReadIgnoreFile(t *testing.T) {
	f, err := os.CreateTemp("", IgnoreFile)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err = f.Write([]byte(`# .sourceignore
ignore-this.txt`)); err != nil {
		t.Fatal(err)
	}
	f.Close()

	tests := []struct {
		name   string
		path   string
		domain []string
		want   []gitignore.Pattern
	}{
		{
			name: IgnoreFile,
			path: f.Name(),
			want: []gitignore.Pattern{
				gitignore.ParsePattern("ignore-this.txt", nil),
			},
		},
		{
			name:   "with domain",
			path:   f.Name(),
			domain: strings.Split(filepath.Dir(f.Name()), string(filepath.Separator)),
			want: []gitignore.Pattern{
				gitignore.ParsePattern("ignore-this.txt", strings.Split(filepath.Dir(f.Name()), string(filepath.Separator))),
			},
		},
		{
			name: "non existing",
			path: "",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadIgnoreFile(tt.path, tt.domain)
			if err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadIgnoreFile() got = %d, want %#v", got, tt.want)
			}
		})
	}
}

func TestVCSPatterns(t *testing.T) {
	tests := []struct {
		name       string
		domain     []string
		patterns   []gitignore.Pattern
		matches    []string
		mismatches []string
	}{
		{
			name:       "simple matches",
			matches:    []string{".git/config", ".gitignore"},
			mismatches: []string{"workload.yaml", "workload.yml", "simple.txt"},
		},
		{
			name:       "domain scoped matches",
			domain:     []string{"directory"},
			matches:    []string{"directory/.git/config", "directory/.gitignore"},
			mismatches: []string{"other/.git/config"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewDefaultMatcher(tt.patterns, tt.domain)
			for _, m := range tt.matches {
				assert.Equal(t, matcher.Match(strings.Split(m, "/"), false), true, "expected %s to match", m)
			}
			for _, m := range tt.mismatches {
				assert.Equal(t, matcher.Match(strings.Split(m, "/"), false), false, "expected %s to not match", m)
			}
		})
	}
}

func TestDefaultPatterns(t *testing.T) {
	tests := []struct {
		name       string
		domain     []string
		patterns   []gitignore.Pattern
		matches    []string
		mismatches []string
	}{
		{
			name:       "simple matches",
			matches:    []string{"image.jpg", "archive.tar.gz", ".github/workflows/workflow.yaml", "subdir/.flux.yaml", "subdir2/.sops.yaml"},
			mismatches: []string{"workload.yaml", "workload.yml", "simple.txt"},
		},
		{
			name:       "domain scoped matches",
			domain:     []string{"directory"},
			matches:    []string{"directory/image.jpg", "directory/archive.tar.gz"},
			mismatches: []string{"other/image.jpg", "other/archive.tar.gz"},
		},
		{
			name:       "patterns",
			patterns:   []gitignore.Pattern{gitignore.ParsePattern("!*.jpg", nil)},
			mismatches: []string{"image.jpg"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewDefaultMatcher(tt.patterns, tt.domain)
			for _, m := range tt.matches {
				assert.Equal(t, matcher.Match(strings.Split(m, "/"), false), true, "expected %s to match", m)
			}
			for _, m := range tt.mismatches {
				assert.Equal(t, matcher.Match(strings.Split(m, "/"), false), false, "expected %s to not match", m)
			}
		})
	}
}

func TestLoadExcludePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	files := map[string]string{
		".sourceignore":     "root.txt",
		"d/.gitignore":      "ignored",
		"z/.sourceignore":   "last.txt",
		"a/b/.sourceignore": "subdir.txt",
		"e/last.txt":        "foo",
		"a/c/subdir.txt":    "bar",
	}
	for n, c := range files {
		if err := os.MkdirAll(filepath.Join(tmpDir, filepath.Dir(n)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, n), []byte(c), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name   string
		dir    string
		domain []string
		want   []gitignore.Pattern
	}{
		{
			name: "traverse loads",
			dir:  tmpDir,
			want: []gitignore.Pattern{
				gitignore.ParsePattern("root.txt", []string{}),
				gitignore.ParsePattern("subdir.txt", []string{"a", "b"}),
				gitignore.ParsePattern("last.txt", []string{"z"}),
			},
		},
		{
			name:   "domain",
			dir:    tmpDir,
			domain: strings.Split(tmpDir, string(filepath.Separator)),
			want: []gitignore.Pattern{
				gitignore.ParsePattern("root.txt", strings.Split(tmpDir, string(filepath.Separator))),
				gitignore.ParsePattern("subdir.txt", append(strings.Split(tmpDir, string(filepath.Separator)), "a", "b")),
				gitignore.ParsePattern("last.txt", append(strings.Split(tmpDir, string(filepath.Separator)), "z")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadIgnorePatterns(tt.dir, tt.domain)
			if err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadIgnorePatterns() got = %#v, want %#v", got, tt.want)
				for _, v := range got {
					t.Error(v)
				}
			}
		})
	}
}
