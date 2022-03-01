package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ExtractTarGzFile(tarGzPath string, targetPath string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("archive %v could not be opened: %w", tarGzPath, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("archive %v could not be opened: %w", tarGzPath, err)
	}
	defer gz.Close()

	return ExtractTarGzStream(gz, targetPath)
}

func ExtractTarGzStream(r io.Reader, targetPath string) error {
	tarReader := tar.NewReader(r)
	for true {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("ExtractTarGz: Next() failed: %w", err)
		}

		header.Name = strings.ReplaceAll(header.Name, "/", string(os.PathSeparator))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(filepath.Join(targetPath, header.Name), 0755); err != nil {
				return fmt.Errorf("ExtractTarGz: Mkdir() failed: %w", err)
			}
		case tar.TypeReg:
			outFile, err := os.Create(filepath.Join(targetPath, header.Name))
			if err != nil {
				return fmt.Errorf("ExtractTarGz: Create() failed: %w", err)
			}
			_, err = io.Copy(outFile, tarReader)
			_ = outFile.Close()
			if err != nil {
				return fmt.Errorf("ExtractTarGz: Copy() failed: %w", err)
			}
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, filepath.Join(targetPath, header.Name)); err != nil {
				return fmt.Errorf("ExtractTarGz: Symlink() failed: %w", err)
			}
		default:
			return fmt.Errorf("ExtractTarGz: uknown type %v in %v", header.Typeflag, header.Name)
		}
	}
	return nil
}

func AddToTar(tw *tar.Writer, pth string, name string, filter func(h *tar.Header) (*tar.Header, error)) error {
	fi, err := os.Stat(pth)
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
	h.Name = strings.ReplaceAll(name, string(os.PathSeparator), "/")

	if filter != nil {
		h, err = filter(h)
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
