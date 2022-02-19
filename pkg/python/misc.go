package python

/*
#cgo pkg-config: python3
#include "Python.h"
*/
import "C"

import (
	"unsafe"
)

type PyThreadState C.PyThreadState

func Py_Initialize() {
	C.Py_Initialize()
}

func Py_NewInterpreter() *PyThreadState {
	p := C.Py_NewInterpreter()
	return (*PyThreadState)(unsafe.Pointer(p))
}

func Py_EndInterpreter(o *PyThreadState) {
	C.Py_EndInterpreter((*C.PyThreadState)(unsafe.Pointer(o)))
}

func PyEval_AcquireThread(o *PyThreadState) {
	C.PyEval_AcquireThread((*C.PyThreadState)(unsafe.Pointer(o)))
}

func PyEval_ReleaseThread(o *PyThreadState) {
	C.PyEval_ReleaseThread((*C.PyThreadState)(unsafe.Pointer(o)))
}

func PyEval_SaveThread() *PyThreadState {
	return (*PyThreadState)(C.PyEval_SaveThread())
}

func PyThreadState_Get() *PyThreadState {
	return (*PyThreadState)(C.PyThreadState_Get())
}

func PyImport_ImportModule(name string) *PyObject {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	return (*PyObject)(C.PyImport_ImportModule(cname))
}
