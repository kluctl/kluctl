package utils

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// TODO get rid of all this when https://github.com/otiai10/copy/issues/71 gets implemented

func CopyFile(src string, dst string) error {
	if !IsFile(src) {
		return fmt.Errorf("%s is not a regular file", src)
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return CopyFileStream(f, dst)
}

func FsCopyFile(srcFs fs.FS, src, dst string) error {
	src = filepath.ToSlash(src)
	source, err := srcFs.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	sourceFileStat, err := source.Stat()
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	return CopyFileStream(source, dst)
}

func CopyFileStream(src io.Reader, dst string) error {
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, src)
	return err
}

func FsCopyDir(srcFs fs.FS, src string, dst string) error {
	var err error
	var fds []fs.DirEntry

	src = filepath.ToSlash(src)

	if fds, err = fs.ReadDir(srcFs, src); err != nil {
		return err
	}
	if err = os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := filepath.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = FsCopyDir(srcFs, srcfp, dstfp); err != nil {
				return err
			}
		} else {
			if err = FsCopyFile(srcFs, srcfp, dstfp); err != nil {
				return err
			}
		}
	}
	return nil
}
