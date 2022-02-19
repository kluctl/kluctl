package python

/*
#include "Python.h"
*/
import "C"

type PyList = PyObject

func PyList_New(len int) *PyList {
	return (*PyList)(C.PyList_New(C.Py_ssize_t(len)))
}

func PyList_FromObject(l *PyObject) *PyList {
	return l
}

func (l *PyList) GetItem(pos int) *PyObject {
	return 	(*PyObject)(C.PyList_GetItem((*C.PyObject)(l), C.Py_ssize_t(pos)))
}

func (l *PyList) Append(item *PyObject) int {
	return int(C.PyList_Append((*C.PyObject)(l), (*C.PyObject)(item)))
}
