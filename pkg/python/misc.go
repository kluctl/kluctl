package python

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils/lib_wrapper"
	"unsafe"
)

type PyThreadState unsafe.Pointer

func Py_SetPythonHome(home string) error {
	chome := lib_wrapper.NewCString(home)
	defer chome.Free()

	newHome := togo(pythonModule.Call_VP_PTRS("Py_DecodeLocale", chome.P, nil))
	if newHome == nil {
		return fmt.Errorf("fail to call Py_DecodeLocale on '%s'", home)
	}
	pythonModule.Call_V_PTRS("Py_SetPythonHome", newHome.p)

	return nil
}

func Py_Initialize() {
	pythonModule.Call_V_PTRS("Py_Initialize")
}

func Py_NewInterpreter() PyThreadState {
	p := pythonModule.Call_VP_PTRS("Py_NewInterpreter")
	return (PyThreadState)(p)
}

func Py_EndInterpreter(o PyThreadState) {
	pythonModule.Call_V_PTRS("Py_EndInterpreter", unsafe.Pointer(o))
}

func PyEval_AcquireThread(o PyThreadState) {
	pythonModule.Call_V_PTRS("PyEval_AcquireThread", unsafe.Pointer(o))
}

func PyEval_ReleaseThread(o PyThreadState) {
	pythonModule.Call_V_PTRS("PyEval_ReleaseThread", unsafe.Pointer(o))
}

func PyEval_SaveThread() PyThreadState {
	return PyThreadState(pythonModule.Call_VP_PTRS("PyEval_SaveThread"))
}

func PyThreadState_Get() PyThreadState {
	return PyThreadState(pythonModule.Call_VP_PTRS("PyThreadState_Get"))
}

func PyImport_ImportModule(name string) *PyObject {
	cname := lib_wrapper.NewCString(name)
	defer cname.Free()

	return togo(pythonModule.Call_VP_PTRS("PyImport_ImportModule", cname.P))
}
