package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
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

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(path.Join(targetPath, header.Name), 0755); err != nil {
				return fmt.Errorf("ExtractTarGz: Mkdir() failed: %w", err)
			}
		case tar.TypeReg:
			outFile, err := os.Create(path.Join(targetPath, header.Name))
			if err != nil {
				return fmt.Errorf("ExtractTarGz: Create() failed: %w", err)
			}
			_, err = io.Copy(outFile, tarReader)
			_ = outFile.Close()
			if err != nil {
				return fmt.Errorf("ExtractTarGz: Copy() failed: %w", err)
			}
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, path.Join(targetPath, header.Name)); err != nil {
				return fmt.Errorf("ExtractTarGz: Symlink() failed: %w", err)
			}
		default:
			return fmt.Errorf("ExtractTarGz: uknown type %v in %v", header.Typeflag, header.Name)
		}
	}
	return nil
}
