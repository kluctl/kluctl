package sops

import (
	"bytes"
	"fmt"
	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"io"
	"os"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type decryptingFs struct {
	filesys.FileSystem
	decryptor *decryptor.Decryptor
}

type file struct {
	os.FileInfo
	data *bytes.Buffer
	size int64
}

func NewDecryptingFs(fs filesys.FileSystem, decryptor *decryptor.Decryptor) filesys.FileSystem {
	return &decryptingFs{
		FileSystem: fs,
		decryptor:  decryptor,
	}
}

// Open opens the named file for reading.
func (fs *decryptingFs) Open(path string) (filesys.File, error) {
	f, err := fs.FileSystem.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	format := formats.FormatForPath(path)
	b2, _, err := MaybeDecrypt(fs.decryptor, b, format, format)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt file %s: %w", path, err)
	}
	ret := &file{
		FileInfo: st,
		data:     bytes.NewBuffer(b2),
	}
	return ret, nil
}

// ReadFile returns the contents of the file at the given path.
func (fs *decryptingFs) ReadFile(path string) ([]byte, error) {
	f, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

func (f file) Read(p []byte) (n int, err error) {
	return f.data.Read(p)
}

func (f file) Write(p []byte) (n int, err error) {
	panic("write is not implemented")
}

func (f file) Close() error {
	return nil
}

func (f file) Stat() (os.FileInfo, error) {
	return f, nil
}

func (f file) Size() int64 {
	return int64(f.data.Len())
}
