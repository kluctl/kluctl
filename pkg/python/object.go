package python

import (
	"github.com/codablock/kluctl/pkg/utils/lib_wrapper"
	"unsafe"
)

type PyObject struct {
	p unsafe.Pointer
}

func (p *PyObject) IncRef() {
	pythonModule.Call_V_PTRS("Py_IncRef", p.p)
}

func (p *PyObject) DecRef() {
	pythonModule.Call_V_PTRS("Py_DecRef", p.p)
}

func (p *PyObject) Length() int {
	return pythonModule.Call_I_PTRS("PyObject_Length", p.p)
}

func (p *PyObject) GetAttrString(attr_name string) *PyObject {
	cattr_name := lib_wrapper.NewCString(attr_name)
	defer cattr_name.Free()

	return togo(pythonModule.Call_VP_PTRS("PyObject_GetAttrString", p.p, cattr_name.P))
}

func (p *PyObject) SetAttrString(attr_name string, v *PyObject) int {
	cattr_name := lib_wrapper.NewCString(attr_name)
	defer cattr_name.Free()

	return pythonModule.Call_I_PTRS("PyObject_SetAttrString", cattr_name.P, v.p)
}

func (p *PyObject) CallObject(args *PyObject) *PyObject {
	var a unsafe.Pointer
	if args != nil {
		a = args.p
	}
	return togo(pythonModule.Call_VP_PTRS("PyObject_CallObject", p.p, a))
}

func (p *PyObject) CallMethodObjArgs(name *PyObject, args ...*PyObject) *PyObject {
	cargs := make([]unsafe.Pointer, len(args), len(args)+1)
	for i, o := range args {
		cargs[i] = o.p
	}
	cargs = append(cargs, nil)
	return togo(pythonModule.Call_VP_PTRS_VARGS("PyObject_CallMethodObjArgs", []unsafe.Pointer{p.p, name.p}, cargs...))
}
