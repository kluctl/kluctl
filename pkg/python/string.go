package python

/*
#include "Python.h"
*/
import "C"
import "unsafe"

func PyUnicode_FromString(u string) *PyObject {
	cu := C.CString(u)
	defer C.free(unsafe.Pointer(cu))

	return (*PyObject)(C.PyUnicode_FromString(cu))
}

func PyUnicode_AsUTF8(unicode *PyObject) string {
	cutf8 := C.PyUnicode_AsUTF8((*C.PyObject)(unicode))
	return C.GoString(cutf8)
}
