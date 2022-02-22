package python

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/lib_wrapper"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"
)

var pythonModule *lib_wrapper.LibWrapper

func init() {
	libPath := decompressLib()

	var module string
	if runtime.GOOS == "windows" {
		module = "python310.dll"
	} else if runtime.GOOS == "darwin" {
		module = "lib/libpython3.10.dylib"
	} else {
		module = "lib/libpython3.10.so"
	}

	pythonModule = lib_wrapper.LoadModule(filepath.Join(libPath, module))

	err := Py_SetPythonHome(libPath)
	if err != nil {
		log.Panic(err)
	}

	Py_Initialize()
	mainThreadState = PyEval_SaveThread()
}

func togo(p unsafe.Pointer) *PyObject {
	if p == nil {
		return nil
	}
	return &PyObject{p: p}
}

func decompressLib() string {
	tgz, err := pythonLib.ReadFile(fmt.Sprintf("python-lib-%s.tar.gz", runtime.GOOS))
	if err != nil {
		log.Panic(err)
	}

	hash := sha256.Sum256(tgz)
	hashStr := hex.EncodeToString(hash[:])

	libPath := filepath.Join(utils.GetTmpBaseDir(), fmt.Sprintf("python-libs-%s-%s", runtime.GOOS, hashStr[:16]))
	if utils.Exists(libPath) {
		return libPath
	}

	g, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		log.Panic(err)
	}
	defer g.Close()

	tmpLibPath, err := ioutil.TempDir(utils.GetTmpBaseDir(), "python-libs-tmp-")
	if err != nil {
		log.Panic(err)
	}
	defer os.RemoveAll(tmpLibPath)

	err = utils.ExtractTarGzStream(g, tmpLibPath)
	if err != nil {
		log.Panic(err)
	}

	err = os.Rename(tmpLibPath, libPath)
	if err != nil && !os.IsExist(err) {
		log.Panic(err)
	}
	return libPath
}
