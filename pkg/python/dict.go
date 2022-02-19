package python

/*
#include "Python.h"
*/
import "C"
import "unsafe"

type PyDict = PyObject

func PyDict_New() *PyDict {
	return (*PyList)(C.PyDict_New())
}

func PyDict_FromObject(o *PyObject) *PyDict {
	return (*PyDict)(o)
}

func (d *PyDict) GetItemString(key string) *PyObject {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))

	return (*PyObject)(C.PyDict_GetItemString((*C.PyObject)(d), ckey))
}