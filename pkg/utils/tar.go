package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
)

func ExtractTarGzFile(tarGzPath string, targetPath string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("archive %v could not be opened: %w", tarGzPath, err)
	}
	defer f.Close()

	err = ExtractTarGzStream(f, targetPath)
	if err != nil {
		return fmt.Errorf("archive %v could not be extracted: %w", tarGzPath, err)
	}
	return nil
}

func ExtractTarGzStream(r io.Reader, targetPath string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	return ExtractTarStream(gz, targetPath)
}

func ExtractTarStream(r io.Reader, targetPath string) error {
	tarReader := tar.NewReader(r)
	for true {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("ExtractTarStream: Next() failed: %w", err)
		}

		header.Name = filepath.FromSlash(header.Name)

		p := filepath.Join(targetPath, header.Name)
		err = os.MkdirAll(filepath.Dir(p), 0755)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, 0755); err != nil {
				return fmt.Errorf("ExtractTarStream: Mkdir() failed: %w", err)
			}
		case tar.TypeReg:
			outFile, err := os.Create(p)
			if err != nil {
				return fmt.Errorf("ExtractTarStream: Create() failed: %w", err)
			}
			_, err = io.Copy(outFile, tarReader)
			_ = outFile.Close()
			if err != nil {
				return fmt.Errorf("ExtractTarStream: Copy() failed: %w", err)
			}
			err = os.Chmod(p, header.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("ExtractTarStream: Chmod() failed: %w", err)
			}
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, p); err != nil {
				return fmt.Errorf("ExtractTarStream: Symlink() failed: %w", err)
			}
		default:
			return fmt.Errorf("ExtractTarStream: uknown type %v in %v", header.Typeflag, header.Name)
		}
	}
	return nil
}

func AddToTar(tw *tar.Writer, pth string, name string, filter func(h *tar.Header, size int64) (*tar.Header, error)) error {
	fi, err := os.Lstat(pth)
	if err != nil {
		return err
	}

	var linkName string
	if fi.Mode().Type() == fs.ModeSymlink {
		x, err := os.Readlink(pth)
		if err != nil {
			return err
		}
		linkName = x
	}

	h, err := tar.FileInfoHeader(fi, linkName)
	if err != nil {
		return err
	}
	h.Name = filepath.ToSlash(name)

	if filter != nil {
		s := fi.Size()
		if fi.IsDir() {
			s = 0
		}
		h, err = filter(h, s)
		if err != nil {
			return err
		}
		if h == nil {
			return nil
		}
	}

	err = tw.WriteHeader(h)
	if err != nil {
		return err
	}

	if fi.Mode().Type() == fs.ModeSymlink {
		return nil
	}

	if fi.Mode().IsDir() {
		des, err := os.ReadDir(pth)
		if err != nil {
			return err
		}
		for _, d := range des {
			err = AddToTar(tw, filepath.Join(pth, d.Name()), filepath.Join(name, d.Name()), filter)
			if err != nil {
				return err
			}
		}
		return nil
	} else if fi.Mode().IsRegular() {
		f, err := os.Open(pth)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		if err != nil {
			return err
		}
		return nil
	} else {
		return fmt.Errorf("unsupported file type/mode %s", fi.Mode().String())
	}
}

func HashTarEntry(dir string, name string) (string, error) {
	p := filepath.Join(dir, filepath.FromSlash(name))
	st, err := os.Lstat(p)
	if err != nil {
		return "", err
	}
	var hashData []byte
	if st.Mode().Type() == fs.ModeDir {
		hashData = []byte(filepath.ToSlash(name))
	} else if st.Mode().Type() == fs.ModeSymlink {
		l, err := os.Readlink(p)
		if err != nil {
			return "", err
		}
		hashData = []byte(l)
	} else if st.Mode().IsRegular() {
		var err error
		hashData, err = ioutil.ReadFile(p)
		if err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("unknown type %s", st.Mode().Type())
	}
	hashStr := Sha256Bytes(hashData)
	return hashStr, nil
}
