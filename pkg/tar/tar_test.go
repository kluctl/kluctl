/*
Copyright 2022 The Flux authors

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

package tar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type untarTestCase struct {
	name            string
	targetDir       string
	secureTargetDir string
	fileName        string
	content         []byte
	wantErr         string
	maxUntarSize    int
	allowSymlink    bool
}

func TestUntar(t *testing.T) {
	targetDirOutput := filepath.Join(t.TempDir(), "output")
	symlink := filepath.Join(t.TempDir(), "symlink")

	subdir := filepath.Join(targetDirOutput, "subdir")
	err := os.MkdirAll(subdir, 0o755)
	if err != nil {
		t.Fatalf("cannot create subdir: %v", err)
	}

	err = os.Symlink(subdir, symlink)
	if err != nil {
		t.Fatalf("cannot create symlink: %v", err)
	}

	cases := []untarTestCase{
		{
			name:            "file at root",
			fileName:        "file1",
			content:         geRandomContent(256),
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "file at subdir root",
			fileName:        "abc/fileX",
			content:         geRandomContent(256),
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "directory traversal parent",
			fileName:        "../abc/file",
			content:         geRandomContent(256),
			wantErr:         `tar contained invalid name error "../abc/file"`,
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "breach max size",
			fileName:        "big-file",
			content:         geRandomContent(256),
			maxUntarSize:    255,
			wantErr:         `tar "big-file" is bigger than max archive size of 255 bytes`,
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "breach default max untar size",
			fileName:        "another-big-file",
			content:         geRandomContent(DefaultMaxUntarSize + 1),
			wantErr:         `tar "another-big-file" is bigger than max archive size of 104857600 bytes`,
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "disable max size checks",
			fileName:        "another-big-file",
			content:         geRandomContent(DefaultMaxUntarSize + 1),
			maxUntarSize:    -1,
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "existing subdir",
			fileName:        "subdir/file1",
			content:         geRandomContent(256),
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "relative target dir",
			fileName:        "file1",
			content:         geRandomContent(256),
			targetDir:       "anydir",
			secureTargetDir: "./anydir",
		},
		{
			name:            "relative paths can't ascend",
			fileName:        "file1",
			content:         geRandomContent(256),
			targetDir:       "../../../../../../../../tmp/test",
			secureTargetDir: "./tmp/test",
		},
		{
			name:      "symlink",
			fileName:  "any-file1",
			content:   geRandomContent(256),
			targetDir: symlink,
			wantErr:   fmt.Sprintf(`dir '%s' must be a directory`, symlink),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f, err := createTestTar(tt)
			if err != nil {
				t.Fatalf("creating test tar: %v", err)
			}
			defer os.Remove(f.Name())
			defer os.RemoveAll(tt.targetDir)

			opts := make([]TarOption, 0)
			if tt.maxUntarSize != 0 {
				opts = append(opts, WithMaxUntarSize(tt.maxUntarSize))
			}

			err = Untar(f, tt.targetDir, opts...)
			var got string
			if err != nil {
				got = err.Error()
			}
			if tt.wantErr != got {
				t.Errorf("wanted error: '%s' got: '%v'", tt.wantErr, err)
			}

			if tt.wantErr == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// only assess file if no errors were expected
			if tt.wantErr == "" {
				abs := filepath.Join(tt.secureTargetDir, tt.fileName)
				fi, err := os.Stat(abs)
				if err != nil {
					t.Errorf("stat %q: %v", abs, err)
					return
				}

				if fi.Size() != int64(len(tt.content)) {
					t.Errorf("file size wanted: %d got: %d", len(tt.content), fi.Size())
				}
			}

			if tt.targetDir != tt.secureTargetDir {
				os.RemoveAll(tt.secureTargetDir)
			}
		})
	}
}

func Fuzz_Untar(f *testing.F) {
	tf, err := createTestTar(untarTestCase{
		name:     "file at root",
		fileName: "file1",
		content:  geRandomContent(256),
	})
	if err != nil {
		f.Fatalf("cannot create test tar: %v", err)
	}
	defer os.Remove(tf.Name())

	var content []byte
	_, err = tf.Read(content)
	if err != nil {
		f.Fatalf("cannot read test tar: %v", err)
	}

	f.Add(content)

	f.Fuzz(func(t *testing.T, data []byte) {
		_ = Untar(bytes.NewReader(data), t.TempDir())
	})
}

func createTestTar(tt untarTestCase) (*os.File, error) {
	f, err := os.CreateTemp("", "flux-untar-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	gzw := gzip.NewWriter(f)
	writer := tar.NewWriter(gzw)

	writer.WriteHeader(&tar.Header{
		Name: tt.fileName,
		Size: int64(len(tt.content)),
		Mode: 0o777,
	})

	writer.Write(tt.content)

	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %v", err)
	}
	if err = gzw.Close(); err != nil {
		return nil, fmt.Errorf("close gzip: %v", err)
	}

	name := f.Name()
	if err = f.Close(); err != nil {
		return nil, fmt.Errorf("close file: %v", err)
	}
	f, err = os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("reopen file: %v", err)
	}
	return f, nil
}

func geRandomContent(len int) []byte {
	content := make([]byte, len)
	rand.Read(content)
	return content
}
