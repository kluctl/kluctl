package python

/*
#include "Python.h"
#include "variadic.h"
*/
import "C"
import "unsafe"

const MaxVariadicLength = 20

type PyObject C.PyObject

func (p *PyObject) IncRef() {
	C.Py_IncRef((*C.PyObject)(p))
}

func (p *PyObject) DecRef() {
	C.Py_DecRef((*C.PyObject)(p))
}

func (p *PyObject) Length() int {
	return int(C.PyObject_Length((*C.PyObject)(p)))
}

func (p *PyObject) GetAttrString(attr_name string) *PyObject {
	cattr_name := C.CString(attr_name)
	defer C.free(unsafe.Pointer(cattr_name))

	return (*PyObject)(C.PyObject_GetAttrString((*C.PyObject)(p), cattr_name))
}

func (p *PyObject) SetAttrString(attr_name string, v *PyObject) int {
	cattr_name := C.CString(attr_name)
	defer C.free(unsafe.Pointer(cattr_name))

	return int(C.PyObject_SetAttrString((*C.PyObject)(p), cattr_name, (*C.PyObject)(v)))
}

func (p *PyObject) CallObject(args *PyObject) *PyObject {
	return (*PyObject)(C.PyObject_CallObject((*C.PyObject)(p), (*C.PyObject)(args)))
}

func (p *PyObject) CallMethodObjArgs(name *PyObject, args ...*PyObject) *PyObject {
	if len(args) > MaxVariadicLength {
		panic("CallMethodObjArgs: too many arguments")
	}
	if len(args) == 0 {
		return (*PyObject)(C._go_PyObject_CallMethodObjArgs((*C.PyObject)(p), (*C.PyObject)(name), 0, (**C.PyObject)(nil)))
	}

	cargs := make([]*C.PyObject, len(args), len(args))
	for i, arg := range args {
		cargs[i] = (*C.PyObject)(arg)
	}
	return (*PyObject)(C._go_PyObject_CallMethodObjArgs((*C.PyObject)(p), (*C.PyObject)(name), C.int(len(args)), (**C.PyObject)(unsafe.Pointer(&cargs[0]))))
}
