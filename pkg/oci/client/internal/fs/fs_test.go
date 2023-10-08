// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	mu sync.Mutex
)

func TestMain(m *testing.M) {
	symlinks := []struct {
		oldPath string
		newPath string
	}{
		{
			oldPath: "C:/Users/fluxcd/go/src/github.com/golang/dep/internal/fs/testdata/test.file",
			newPath: "testdata/symlinks/windows-file-symlink",
		},
		{
			oldPath: "/non/existing/file",
			newPath: "testdata/symlinks/invalid-symlink",
		},
		{
			oldPath: "../test.file",
			newPath: "testdata/symlinks/file-symlink",
		},
		{
			oldPath: "../../testdata",
			newPath: "testdata/symlinks/dir-symlink",
		},
	}

	os.MkdirAll("testdata/symlinks", 0o755)

	for _, sl := range symlinks {
		err := os.Symlink(sl.oldPath, sl.newPath)
		if err != nil {
			panic(fmt.Errorf("failed to create symlink: %v", err))
		}
	}

	code := m.Run()

	if err := os.RemoveAll("testdata/symlinks"); err != nil {
		panic(fmt.Errorf("failed to remove symlink directory: %v", err))
	}

	os.Exit(code)
}

func TestRenameWithFallback(t *testing.T) {
	dir := t.TempDir()

	if err := RenameWithFallback(filepath.Join(dir, "does_not_exists"), filepath.Join(dir, "dst")); err == nil {
		t.Fatal("expected an error for non existing file, but got nil")
	}

	srcpath := filepath.Join(dir, "src")

	if srcf, err := os.Create(srcpath); err != nil {
		t.Fatal(err)
	} else {
		srcf.Close()
	}

	if err := RenameWithFallback(srcpath, filepath.Join(dir, "dst")); err != nil {
		t.Fatal(err)
	}

	srcpath = filepath.Join(dir, "a")
	if err := os.MkdirAll(srcpath, 0o770); err != nil {
		t.Fatal(err)
	}

	dstpath := filepath.Join(dir, "b")
	if err := os.MkdirAll(dstpath, 0o770); err != nil {
		t.Fatal(err)
	}

	if err := RenameWithFallback(srcpath, dstpath); err == nil {
		t.Fatal("expected an error if dst is an existing directory, but got nil")
	}
}

func TestCopyDir(t *testing.T) {
	dir := t.TempDir()

	srcdir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcdir, 0o750); err != nil {
		t.Fatal(err)
	}

	files := []struct {
		path     string
		contents string
		fi       os.FileInfo
	}{
		{path: "myfile", contents: "hello world"},
		{path: filepath.Join("subdir", "file"), contents: "subdir file"},
	}

	// Create structure indicated in 'files'
	for i, file := range files {
		fn := filepath.Join(srcdir, file.path)
		dn := filepath.Dir(fn)
		if err := os.MkdirAll(dn, 0o750); err != nil {
			t.Fatal(err)
		}

		fh, err := os.Create(fn)
		if err != nil {
			t.Fatal(err)
		}

		if _, err = fh.Write([]byte(file.contents)); err != nil {
			t.Fatal(err)
		}
		fh.Close()

		files[i].fi, err = os.Stat(fn)
		if err != nil {
			t.Fatal(err)
		}
	}

	destdir := filepath.Join(dir, "dest")
	if err := CopyDir(srcdir, destdir); err != nil {
		t.Fatal(err)
	}

	// Compare copy against structure indicated in 'files'
	for _, file := range files {
		fn := filepath.Join(srcdir, file.path)
		dn := filepath.Dir(fn)
		dirOK, err := IsDir(dn)
		if err != nil {
			t.Fatal(err)
		}
		if !dirOK {
			t.Fatalf("expected %s to be a directory", dn)
		}

		got, err := os.ReadFile(fn)
		if err != nil {
			t.Fatal(err)
		}

		if file.contents != string(got) {
			t.Fatalf("expected: %s, got: %s", file.contents, string(got))
		}

		gotinfo, err := os.Stat(fn)
		if err != nil {
			t.Fatal(err)
		}

		if file.fi.Mode() != gotinfo.Mode() {
			t.Fatalf("expected %s: %#v\n to be the same mode as %s: %#v",
				file.path, file.fi.Mode(), fn, gotinfo.Mode())
		}
	}
}

func TestCopyDirFail_SrcInaccessible(t *testing.T) {
	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in
		// Microsoft Windows. Skipping this this until a
		// compatible implementation is provided.
		t.Skip("skipping on windows")
	}

	var srcdir, dstdir string

	setupInaccessibleDir(t, func(dir string) error {
		srcdir = filepath.Join(dir, "src")
		return os.MkdirAll(srcdir, 0o750)
	})

	dir := t.TempDir()

	dstdir = filepath.Join(dir, "dst")
	if err := CopyDir(srcdir, dstdir); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}
}

func TestCopyDirFail_DstInaccessible(t *testing.T) {
	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in
		// Microsoft Windows. Skipping this this until a
		// compatible implementation is provided.
		t.Skip("skipping on windows")
	}

	var srcdir, dstdir string

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	if err := os.MkdirAll(srcdir, 0o750); err != nil {
		t.Fatal(err)
	}

	setupInaccessibleDir(t, func(dir string) error {
		dstdir = filepath.Join(dir, "dst")
		return nil
	})

	if err := CopyDir(srcdir, dstdir); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}
}

func TestCopyDirFail_SrcIsNotDir(t *testing.T) {
	var srcdir, dstdir string

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	if _, err := os.Create(srcdir); err != nil {
		t.Fatal(err)
	}

	dstdir = filepath.Join(dir, "dst")

	err := CopyDir(srcdir, dstdir)
	if err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}

	if err != errSrcNotDir {
		t.Fatalf("expected %v error for CopyDir(%s, %s), got %s", errSrcNotDir, srcdir, dstdir, err)
	}

}

func TestCopyDirFail_DstExists(t *testing.T) {
	var srcdir, dstdir string

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	if err := os.MkdirAll(srcdir, 0o750); err != nil {
		t.Fatal(err)
	}

	dstdir = filepath.Join(dir, "dst")
	if err := os.MkdirAll(dstdir, 0o750); err != nil {
		t.Fatal(err)
	}

	err := CopyDir(srcdir, dstdir)
	if err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}

	if err != errDstExist {
		t.Fatalf("expected %v error for CopyDir(%s, %s), got %s", errDstExist, srcdir, dstdir, err)
	}
}

func TestCopyDirFailOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in
		// Microsoft Windows. os.Chmod(..., 0o222) below is not
		// enough for the file to be readonly, and os.Chmod(...,
		// 0000) returns an invalid argument error. Skipping
		// this this until a compatible implementation is
		// provided.
		t.Skip("skipping on windows")
	}

	var srcdir, dstdir string

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	if err := os.MkdirAll(srcdir, 0o750); err != nil {
		t.Fatal(err)
	}

	srcfn := filepath.Join(srcdir, "file")
	srcf, err := os.Create(srcfn)
	if err != nil {
		t.Fatal(err)
	}
	srcf.Close()

	// setup source file so that it cannot be read
	if err = os.Chmod(srcfn, 0o220); err != nil {
		t.Fatal(err)
	}

	dstdir = filepath.Join(dir, "dst")

	if err = CopyDir(srcdir, dstdir); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()

	srcf, err := os.Create(filepath.Join(dir, "srcfile"))
	if err != nil {
		t.Fatal(err)
	}

	want := "hello world"
	if _, err := srcf.Write([]byte(want)); err != nil {
		t.Fatal(err)
	}
	srcf.Close()

	destf := filepath.Join(dir, "destf")
	if err := copyFile(srcf.Name(), destf); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(destf)
	if err != nil {
		t.Fatal(err)
	}

	if want != string(got) {
		t.Fatalf("expected: %s, got: %s", want, string(got))
	}

	wantinfo, err := os.Stat(srcf.Name())
	if err != nil {
		t.Fatal(err)
	}

	gotinfo, err := os.Stat(destf)
	if err != nil {
		t.Fatal(err)
	}

	if wantinfo.Mode() != gotinfo.Mode() {
		t.Fatalf("expected %s: %#v\n to be the same mode as %s: %#v", srcf.Name(), wantinfo.Mode(), destf, gotinfo.Mode())
	}
}

func TestCopyFileSymlink(t *testing.T) {
	dir := t.TempDir()
	defer cleanUpDir(dir)

	testcases := map[string]string{
		filepath.Join("./testdata/symlinks/file-symlink"):         filepath.Join(dir, "dst-file"),
		filepath.Join("./testdata/symlinks/windows-file-symlink"): filepath.Join(dir, "windows-dst-file"),
		filepath.Join("./testdata/symlinks/invalid-symlink"):      filepath.Join(dir, "invalid-symlink"),
	}

	for symlink, dst := range testcases {
		t.Run(symlink, func(t *testing.T) {
			var err error
			if err = copyFile(symlink, dst); err != nil {
				t.Fatalf("failed to copy symlink: %s", err)
			}

			var want, got string

			if runtime.GOOS == "windows" {
				// Creating symlinks on Windows require an additional permission
				// regular users aren't granted usually. So we copy the file
				// content as a fall back instead of creating a real symlink.
				srcb, err := os.ReadFile(symlink)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				dstb, err := os.ReadFile(dst)
				if err != nil {
					t.Fatalf("%+v", err)
				}

				want = string(srcb)
				got = string(dstb)
			} else {
				want, err = os.Readlink(symlink)
				if err != nil {
					t.Fatalf("%+v", err)
				}

				got, err = os.Readlink(dst)
				if err != nil {
					t.Fatalf("could not resolve symlink: %s", err)
				}
			}

			if want != got {
				t.Fatalf("resolved path is incorrect. expected %s, got %s", want, got)
			}
		})
	}
}

func TestCopyFileLongFilePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		// We want to ensure the temporary fix actually fixes the issue with
		// os.Chmod and long file paths. This is only applicable on Windows.
		t.Skip("skipping on non-windows")
	}

	dir := t.TempDir()

	// Create a directory with a long-enough path name to cause the bug in #774.
	dirName := ""
	for len(dir+string(os.PathSeparator)+dirName) <= 300 {
		dirName += "directory"
	}

	fullPath := filepath.Join(dir, dirName, string(os.PathSeparator))
	if err := os.MkdirAll(fullPath, 0o750); err != nil && !os.IsExist(err) {
		t.Fatalf("%+v", fmt.Errorf("unable to create temp directory: %s", fullPath))
	}

	err := os.WriteFile(fullPath+"src", []byte(nil), 0o640)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	err = copyFile(fullPath+"src", fullPath+"dst")
	if err != nil {
		t.Fatalf("unexpected error while copying file: %v", err)
	}
}

// C:\Users\appveyor\AppData\Local\Temp\1\gotest639065787\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890

func TestCopyFileFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in
		// Microsoft Windows. Skipping this this until a
		// compatible implementation is provided.
		t.Skip("skipping on windows")
	}

	dir := t.TempDir()

	srcf, err := os.Create(filepath.Join(dir, "srcfile"))
	if err != nil {
		t.Fatal(err)
	}
	srcf.Close()

	var dstdir string

	setupInaccessibleDir(t, func(dir string) error {
		dstdir = filepath.Join(dir, "dir")
		return os.Mkdir(dstdir, 0o770)
	})

	fn := filepath.Join(dstdir, "file")
	if err := copyFile(srcf.Name(), fn); err == nil {
		t.Fatalf("expected error for %s, got none", fn)
	}
}

// setupInaccessibleDir creates a temporary location with a single
// directory in it, in such a way that that directory is not accessible
// after this function returns.
//
// op is called with the directory as argument, so that it can create
// files or other test artifacts.
//
// If setupInaccessibleDir fails in its preparation, or op fails, t.Fatal
// will be invoked.
func setupInaccessibleDir(t *testing.T, op func(dir string) error) {
	dir, err := os.MkdirTemp("", "dep")
	if err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(dir, "dir")

	t.Cleanup(func() {
		if err := os.Chmod(subdir, 0o770); err != nil {
			t.Error(err)
		}
	})

	if err := os.Mkdir(subdir, 0o770); err != nil {
		t.Fatal(err)
	}

	if err := op(subdir); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(subdir, 0o660); err != nil {
		t.Fatal(err)
	}
}

func TestIsDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	var dn string

	setupInaccessibleDir(t, func(dir string) error {
		dn = filepath.Join(dir, "dir")
		return os.Mkdir(dn, 0o770)
	})

	tests := map[string]struct {
		exists bool
		err    bool
	}{
		wd:                            {true, false},
		filepath.Join(wd, "testdata"): {true, false},
		filepath.Join(wd, "main.go"):  {false, true},
		filepath.Join(wd, "this_file_does_not_exist.thing"): {false, true},
		dn: {false, true},
	}

	if runtime.GOOS == "windows" {
		// This test doesn't work on Microsoft Windows because
		// of the differences in how file permissions are
		// implemented. For this to work, the directory where
		// the directory exists should be inaccessible.
		delete(tests, dn)
	}

	for f, want := range tests {
		got, err := IsDir(f)
		if err != nil && !want.err {
			t.Fatalf("expected no error, got %v", err)
		}

		if got != want.exists {
			t.Fatalf("expected %t for %s, got %t", want.exists, f, got)
		}
	}
}

func TestIsSymlink(t *testing.T) {
	dir := t.TempDir()

	dirPath := filepath.Join(dir, "directory")
	if err := os.MkdirAll(dirPath, 0o770); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(dir, "file")
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	dirSymlink := filepath.Join(dir, "dirSymlink")
	fileSymlink := filepath.Join(dir, "fileSymlink")

	if err = os.Symlink(dirPath, dirSymlink); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filePath, fileSymlink); err != nil {
		t.Fatal(err)
	}

	var (
		inaccessibleFile    string
		inaccessibleSymlink string
	)

	setupInaccessibleDir(t, func(dir string) error {
		inaccessibleFile = filepath.Join(dir, "file")
		if fh, err := os.Create(inaccessibleFile); err != nil {
			return err
		} else if err = fh.Close(); err != nil {
			return err
		}

		inaccessibleSymlink = filepath.Join(dir, "symlink")
		return os.Symlink(inaccessibleFile, inaccessibleSymlink)
	})

	tests := map[string]struct{ expected, err bool }{
		dirPath:             {false, false},
		filePath:            {false, false},
		dirSymlink:          {true, false},
		fileSymlink:         {true, false},
		inaccessibleFile:    {false, true},
		inaccessibleSymlink: {false, true},
	}

	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in Windows. Skipping
		// these cases until a compatible implementation is provided.
		delete(tests, inaccessibleFile)
		delete(tests, inaccessibleSymlink)
	}

	for path, want := range tests {
		got, err := IsSymlink(path)
		if err != nil {
			if !want.err {
				t.Errorf("expected no error, got %v", err)
			}
		}

		if got != want.expected {
			t.Errorf("expected %t for %s, got %t", want.expected, path, got)
		}
	}
}

func cleanUpDir(dir string) {
	if dir != "" {
		os.RemoveAll(dir)
	}
}
