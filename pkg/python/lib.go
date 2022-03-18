package python

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/embed_util"
	"github.com/codablock/kluctl/pkg/utils/lib_wrapper"
	"log"
	"path/filepath"
	"runtime"
	"unsafe"
)

var pythonModule *lib_wrapper.LibWrapper
var PythonWrapper LibPythonWrapper

type PythonThreadState struct {
	P unsafe.Pointer
}

func PythonThreadState_FromPointer(p unsafe.Pointer) *PythonThreadState {
	if p == nil {
		return nil
	}
	return &PythonThreadState{P: p}
}

func (p *PythonThreadState) GetPointer() unsafe.Pointer {
	if p == nil {
		return nil
	}
	return p.P
}

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
	PythonWrapper = New_LibPythonWrapper(pythonModule)

	l := PythonWrapper.Py_DecodeLocale(libPath, nil)
	PythonWrapper.Py_SetPythonHome(l)

	PythonWrapper.Py_InitializeEx(0)
	mainThreadState = PythonWrapper.PyEval_SaveThread()
}

func decompressLib() string {
	tarName := fmt.Sprintf("python-lib-%s.tar.gz", runtime.GOOS)
	tgz, err := pythonLib.Open(tarName)
	if err != nil {
		log.Panic(err)
	}
	defer tgz.Close()

	fileList, err := pythonLib.Open(tarName + ".files")
	if err != nil {
		log.Panic(err)
	}
	defer fileList.Close()

	libPath := filepath.Join(utils.GetTmpBaseDir(), fmt.Sprintf("python-libs-%s", runtime.GOOS))
	err = embed_util.ExtractTarToTmp(tgz, fileList, libPath)
	if err != nil {
		log.Panic(err)
	}

	return libPath
}
