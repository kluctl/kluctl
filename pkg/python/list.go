package python

import "unsafe"

type PyList = PyObject

func PyList_New(len int) *PyList {
	return PythonWrapper.PyList_New(len)
}

func PyList_FromPointer(p unsafe.Pointer) *PyList {
	if p == nil {
		return nil
	}
	return &PyList{p: p}
}

func PyList_FromObject(l *PyObject) *PyList {
	return l
}

func (l *PyList) GetItem(pos int) *PyObject {
	return PythonWrapper.PyList_GetItem(l, pos)
}

func (l *PyList) Append(item *PyObject) int {
	return PythonWrapper.PyList_Append(l, item)
}
