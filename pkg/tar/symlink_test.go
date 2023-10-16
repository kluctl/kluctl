/*
Copyright 2023 The Flux authors

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
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestSkipSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	symlinkTarget := filepath.Join(tmpDir, "symlink.target")
	err := os.WriteFile(symlinkTarget, geRandomContent(256), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	symlink := filepath.Join(tmpDir, "symlink")
	err = os.Symlink(symlinkTarget, symlink)
	if err != nil {
		t.Fatal(err)
	}

	tgzFileName := filepath.Join(t.TempDir(), "test.tgz")
	var buf bytes.Buffer
	err = tgzWithSymlinks(tmpDir, &buf)
	if err != nil {
		t.Fatal(err)
	}

	tgzFile, err := os.OpenFile(tgzFileName, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(tgzFile, &buf); err != nil {
		t.Fatal(err)
	}
	if err = tgzFile.Close(); err != nil {
		t.Fatal(err)
	}

	targetDirOutput := filepath.Join(t.TempDir(), "output")
	f1, err := os.Open(tgzFileName)
	if err != nil {
		t.Fatal(err)
	}

	err = Untar(f1, targetDirOutput, WithMaxUntarSize(-1))
	if err == nil {
		t.Errorf("wanted error: unsupported symlink")
	}

	f2, err := os.Open(tgzFileName)
	if err != nil {
		t.Fatal(err)
	}

	err = Untar(f2, targetDirOutput, WithMaxUntarSize(-1), WithSkipSymlinks())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := os.Open(path.Join(targetDirOutput, "symlink.target")); err != nil {
		t.Errorf("regular file not found: %v", err)
	}
}

func tgzWithSymlinks(src string, buf io.Writer) error {
	absDir, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	zr := gzip.NewWriter(buf)
	tw := tar.NewWriter(zr)
	if err := filepath.Walk(absDir, func(file string, fi os.FileInfo, err error) error {
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if fi.Mode().IsRegular() {
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
			return f.Close()
		}

		return nil
	}); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := zr.Close(); err != nil {
		return err
	}
	return nil
}
