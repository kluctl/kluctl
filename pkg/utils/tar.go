package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
)

func ExtractTarGz(tarGzPath string, targetPath string) error {
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

	tarReader := tar.NewReader(gz)
	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("ExtractTarGz: Next() for %v failed: %w", tarGzPath, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(path.Join(targetPath, header.Name), 0755); err != nil {
				return fmt.Errorf("ExtractTarGz: Mkdir() for %v failed: %w", tarGzPath, err)
			}
		case tar.TypeReg:
			outFile, err := os.Create(path.Join(targetPath, header.Name))
			if err != nil {
				return fmt.Errorf("ExtractTarGz: Create() for %v failed: %w", tarGzPath, err)
			}
			_, err = io.Copy(outFile, tarReader)
			_ = outFile.Close()
			if err != nil {
				return fmt.Errorf("ExtractTarGz: Copy() for %v failed: %w", tarGzPath, err)
			}
		default:
			return fmt.Errorf("ExtractTarGz: uknown type for %v: %v in %v", tarGzPath, header.Typeflag, header.Name)
		}
	}
	return nil
}
