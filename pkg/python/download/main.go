package main

import (
	"fmt"
	"github.com/klauspost/compress/zstd"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

const (
	pythonVersionBase       = "3.10"
	pythonVersionFull       = "3.10.3"
	pythonStandaloneVersion = "20220318"
)

var pythonDists = map[string]string{
	"linux": "unknown-linux-gnu-lto-full",
	"darwin": "apple-darwin-lto-full",
	"windows": "pc-windows-msvc-shared-pgo-full",
}

var archMapping = map[string]string{
	"amd64": "x86_64",
	"386": "i686",
	"arm64": "aarch64",
}

var removeLibs = []string{
	"site-packages",
	"venv",
	"ensurepip",
	"idlelib",
	"distutils",
	"pydoc_data",
	"asyncio",
	"email",
	"tkinter",
	"lib2to3",
	"xml",
	"multiprocessing",
	"unittest",
}

func main() {
	osName := os.Args[1]
	arch := os.Args[2]
	extractPath := os.Args[3]

	dist, ok := pythonDists[osName]
	if !ok {
		log.Panicf("no dist for %s", osName)
	}

	downloadPath := download(osName, arch, dist)

	os.RemoveAll(extractPath)
	decompress(downloadPath, extractPath)

	for _, lib := range removeLibs {
		_ = os.RemoveAll(filepath.Join(extractPath, "python", "install", "lib", fmt.Sprintf("python%s", pythonVersionBase), lib))
		_ = os.RemoveAll(filepath.Join(extractPath, "python", "install", "Lib", lib))
	}
}

func download(osName, arch, dist string) string {
	pythonArch, ok := archMapping[arch]
	if !ok {
		log.Errorf("arch %s not supported", arch)
		os.Exit(1)
	}
	fname := fmt.Sprintf("cpython-%s+%s-%s-%s.tar.zst", pythonVersionFull, pythonStandaloneVersion, pythonArch, dist)
	downloadPath := filepath.Join(os.TempDir(), fname)
	downloadUrl := fmt.Sprintf("https://github.com/indygreg/python-build-standalone/releases/download/%s/%s", pythonStandaloneVersion, fname)

	if _, err := os.Stat(downloadPath); err == nil {
		log.Infof("skipping download of %s", downloadUrl)
		return downloadPath
	}

	log.Infof("downloading %s", downloadUrl)

	r, err := http.Get(downloadUrl)
	if err != nil {
		log.Errorf("download failed: %v", err)
		os.Exit(1)
	}
	if r.StatusCode == http.StatusNotFound {
		log.Errorf("404 not found")
		os.Exit(1)
	}
	defer r.Body.Close()

	fileData, err := ioutil.ReadAll(r.Body)

	err = ioutil.WriteFile(downloadPath, fileData, 0o640)
	if err != nil {
		log.Errorf("writing file failed: %v", err)
		os.Remove(downloadPath)
		os.Exit(1)
	}

	return downloadPath
}

func decompress(archivePath string, targetPath string) string {
	f, err := os.Open(archivePath)
	if err != nil {
		log.Errorf("opening file failed: %v", err)
		os.Exit(1)
	}
	defer f.Close()

	z, err := zstd.NewReader(f)
	if err != nil {
		log.Errorf("decompression failed: %v", err)
		os.Exit(1)
	}
	defer z.Close()

	log.Infof("decompressing %s", archivePath)
	err = utils.ExtractTarStream(z, targetPath)
	if err != nil {
		log.Errorf("decompression failed: %v", err)
		os.Exit(1)
	}

	return targetPath
}