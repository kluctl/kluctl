package utils

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
)

func ExtractTar(tarData []byte, destDir string) error {
	tr := tar.NewReader(bytes.NewReader(tarData))

	// First pass: extract everything as-is
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	// Check if Chart.yaml exists in root or subdirectory
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return err
	}

	// If Chart.yaml is not in root, but exists in a single subdirectory, move everything up
	if _, err := os.Stat(filepath.Join(destDir, "Chart.yaml")); os.IsNotExist(err) {
		for _, entry := range entries {
			if entry.IsDir() {
				subDir := filepath.Join(destDir, entry.Name())
				if _, err := os.Stat(filepath.Join(subDir, "Chart.yaml")); err == nil {
					// Found Chart.yaml in subdirectory, move contents up
					subEntries, err := os.ReadDir(subDir)
					if err != nil {
						return err
					}
					for _, subEntry := range subEntries {
						oldPath := filepath.Join(subDir, subEntry.Name())
						newPath := filepath.Join(destDir, subEntry.Name())
						if err := os.Rename(oldPath, newPath); err != nil {
							return err
						}
					}
					os.Remove(subDir) // Remove now-empty directory
					break
				}
			}
		}
	}

	return nil
}
