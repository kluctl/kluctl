package e2e

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	tmpFile1, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	tmpFile2, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	tmpFile2.Close()
	defer func() {
		os.Remove(tmpFile2.Name())
	}()
	tmpDir1, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}
	defer func() {
		os.RemoveAll(tmpDir1)
	}()
	tmpDir2, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}
	defer func() {
		os.RemoveAll(tmpDir2)
	}()
	os.Setenv("HELM_REGISTRY_CONFIG", tmpFile1.Name())
	os.Setenv("HELM_REPOSITORY_CONFIG", tmpFile2.Name())
	os.Setenv("HELM_REPOSITORY_CACHE", tmpDir1)
	os.Setenv("HELM_PLUGINS", tmpDir2)
	os.Exit(m.Run())
}
