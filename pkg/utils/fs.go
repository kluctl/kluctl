package utils

import (
	"fmt"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"strings"
)

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return false
	}
	return true
}

func IsFile(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	return fileInfo.Mode().IsRegular()
}

func IsDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	return fileInfo.IsDir()
}

func CheckInDir(root string, path string) error {
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}

	if absRoot == absPath {
		return nil
	}

	if !strings.HasPrefix(absPath, absRoot+string(os.PathSeparator)) {
		return fmt.Errorf("path %s is not inside directory %s", path, root)
	}
	return nil
}

func CheckSubInDir(root string, subDir string) error {
	return CheckInDir(root, filepath.Join(root, subDir))
}

func Touch(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to touch %v: %w", path, err)
	}
	return f.Close()
}

func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		p = homedir.HomeDir() + p[1:]
	}
	return p
}
