package python

import (
	"unsafe"
)

type PyObject struct {
	p unsafe.Pointer
}

func PyObject_FromPointer(p unsafe.Pointer) *PyObject {
	if p == nil {
		return nil
	}
	return &PyObject{p: p}
}

func (p *PyObject) GetPointer() unsafe.Pointer {
	if p == nil {
		return nil
	}
	return p.p
}

func (p *PyObject) IncRef() {
	PythonWrapper.Py_IncRef(p)
}

func (p *PyObject) DecRef() {
	PythonWrapper.Py_DecRef(p)
}

func (p *PyObject) Length() int {
	return PythonWrapper.PyObject_Length(p)
}

func (p *PyObject) GetAttrString(attr_name string) *PyObject {
	return PythonWrapper.PyObject_GetAttrString(p, attr_name)
}

func (p *PyObject) SetAttrString(attr_name string, v *PyObject) int {
	return PythonWrapper.PyObject_SetAttrString(p, attr_name, v)
}

func (p *PyObject) CallObject(args *PyObject) *PyObject {
	return PythonWrapper.PyObject_CallObject(p, args)
}

func (p *PyObject) CallMethodObjArgs(name *PyObject, args ...*PyObject) *PyObject {
	args = append([]*PyObject{}, args...)
	args = append(args, nil)
	return PythonWrapper.PyObject_CallMethodObjArgs(p, name, args)
}
