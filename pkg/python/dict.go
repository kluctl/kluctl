package python

import "C"
import "unsafe"

type PyDict = PyObject

func PyDict_New() *PyDict {
	return PythonWrapper.PyDict_New()
}

func PyDict_FromPointer(p unsafe.Pointer) *PyDict {
	if p == nil {
		return nil
	}
	return &PyDict{p: p}
}

func PyDict_FromObject(o *PyObject) *PyDict {
	return o
}

func (d *PyDict) GetItemString(key string) *PyObject {
	return PythonWrapper.PyDict_GetItemString(d, key)
}
