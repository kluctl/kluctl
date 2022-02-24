package python

/*
#include <assert.h>
#include <stdlib.h>
#include <sys/types.h>
void _trampoline_Py_Initialize(void* f) {
	typedef void (*F)();
	((F)f)();
}
void _trampoline_Py_InitializeEx(void* f, int initsigs) {
	typedef void (*F)(int);
	((F)f)(initsigs);
}
void* _trampoline_Py_DecodeLocale(void* f, char* s) {
	typedef void* (*F)(char*);
	return ((F)f)(s);
}
void _trampoline_Py_SetPythonHome(void* f, void* l) {
	typedef void (*F)(void*);
	((F)f)(l);
}
void* _trampoline_Py_NewInterpreter(void* f) {
	typedef void* (*F)();
	return ((F)f)();
}
void _trampoline_Py_EndInterpreter(void* f, void* o) {
	typedef void (*F)(void*);
	((F)f)(o);
}
void _trampoline_PyEval_AcquireThread(void* f, void* o) {
	typedef void (*F)(void*);
	((F)f)(o);
}
void _trampoline_PyEval_ReleaseThread(void* f, void* o) {
	typedef void (*F)(void*);
	((F)f)(o);
}
void* _trampoline_PyEval_SaveThread(void* f) {
	typedef void* (*F)();
	return ((F)f)();
}
void* _trampoline_PyThreadState_Swap(void* f, void* o) {
	typedef void* (*F)(void*);
	return ((F)f)(o);
}
void _trampoline_PyEval_ReleaseLock(void* f) {
	typedef void (*F)();
	((F)f)();
}
void* _trampoline_PyImport_ImportModule(void* f, char* name) {
	typedef void* (*F)(char*);
	return ((F)f)(name);
}
void _trampoline_Py_IncRef(void* f, void* o) {
	typedef void (*F)(void*);
	((F)f)(o);
}
void _trampoline_Py_DecRef(void* f, void* o) {
	typedef void (*F)(void*);
	((F)f)(o);
}
int _trampoline_PyObject_Length(void* f, void* p) {
	typedef int (*F)(void*);
	return ((F)f)(p);
}
void* _trampoline_PyObject_GetAttrString(void* f, void* p, char* name) {
	typedef void* (*F)(void*, char*);
	return ((F)f)(p, name);
}
int _trampoline_PyObject_SetAttrString(void* f, void* p, char* name, void* p2) {
	typedef int (*F)(void*, char*, void*);
	return ((F)f)(p, name, p2);
}
void* _trampoline_PyObject_CallObject(void* f, void* p, void* args) {
	typedef void* (*F)(void*, void*);
	return ((F)f)(p, args);
}
void* _trampoline_PyObject_CallMethodObjArgs(void* f, void* p, void* name, int cargs_vargs_len, void** cargs_vargs) {
	typedef void* (*F)(void*, void*, ...);
	switch(cargs_vargs_len) {
	case 0: return ((F)f)(p, name);
	case 1: return ((F)f)(p, name, cargs_vargs[0]);
	case 2: return ((F)f)(p, name, cargs_vargs[0], cargs_vargs[1]);
	case 3: return ((F)f)(p, name, cargs_vargs[0], cargs_vargs[1], cargs_vargs[2]);
	case 4: return ((F)f)(p, name, cargs_vargs[0], cargs_vargs[1], cargs_vargs[2], cargs_vargs[3]);
	case 5: return ((F)f)(p, name, cargs_vargs[0], cargs_vargs[1], cargs_vargs[2], cargs_vargs[3], cargs_vargs[4]);
	case 6: return ((F)f)(p, name, cargs_vargs[0], cargs_vargs[1], cargs_vargs[2], cargs_vargs[3], cargs_vargs[4], cargs_vargs[5]);
	case 7: return ((F)f)(p, name, cargs_vargs[0], cargs_vargs[1], cargs_vargs[2], cargs_vargs[3], cargs_vargs[4], cargs_vargs[5], cargs_vargs[6]);
	case 8: return ((F)f)(p, name, cargs_vargs[0], cargs_vargs[1], cargs_vargs[2], cargs_vargs[3], cargs_vargs[4], cargs_vargs[5], cargs_vargs[6], cargs_vargs[7]);
	case 9: return ((F)f)(p, name, cargs_vargs[0], cargs_vargs[1], cargs_vargs[2], cargs_vargs[3], cargs_vargs[4], cargs_vargs[5], cargs_vargs[6], cargs_vargs[7], cargs_vargs[8]);
	default: assert(0);
	}
}
void* _trampoline_PyList_New(void* f, ssize_t len) {
	typedef void* (*F)(ssize_t);
	return ((F)f)(len);
}
void* _trampoline_PyList_GetItem(void* f, void* o, ssize_t i) {
	typedef void* (*F)(void*, ssize_t);
	return ((F)f)(o, i);
}
int _trampoline_PyList_SetItem(void* f, void* o, ssize_t i, void* e) {
	typedef int (*F)(void*, ssize_t, void*);
	return ((F)f)(o, i, e);
}
int _trampoline_PyList_Append(void* f, void* o, void* e) {
	typedef int (*F)(void*, void*);
	return ((F)f)(o, e);
}
void* _trampoline_PyDict_New(void* f) {
	typedef void* (*F)();
	return ((F)f)();
}
void* _trampoline_PyDict_GetItemString(void* f, void* p, char* key) {
	typedef void* (*F)(void*, char*);
	return ((F)f)(p, key);
}
void* _trampoline_PyUnicode_FromString(void* f, char* pth) {
	typedef void* (*F)(char*);
	return ((F)f)(pth);
}
char* _trampoline_PyUnicode_AsUTF8(void* f, void* r) {
	typedef char* (*F)(void*);
	return ((F)f)(r);
}
void _trampoline_PyErr_Print(void* f) {
	typedef void (*F)();
	((F)f)();
}
*/
import "C"

import "unsafe"
import "github.com/codablock/kluctl/pkg/utils/lib_wrapper"

type LibPythonWrapperImpl struct {
	module                           *lib_wrapper.LibWrapper
	_func_Py_Initialize              lib_wrapper.FunctionPtr
	_func_Py_InitializeEx            lib_wrapper.FunctionPtr
	_func_Py_DecodeLocale            lib_wrapper.FunctionPtr
	_func_Py_SetPythonHome           lib_wrapper.FunctionPtr
	_func_Py_NewInterpreter          lib_wrapper.FunctionPtr
	_func_Py_EndInterpreter          lib_wrapper.FunctionPtr
	_func_PyEval_AcquireThread       lib_wrapper.FunctionPtr
	_func_PyEval_ReleaseThread       lib_wrapper.FunctionPtr
	_func_PyEval_SaveThread          lib_wrapper.FunctionPtr
	_func_PyThreadState_Swap         lib_wrapper.FunctionPtr
	_func_PyEval_ReleaseLock         lib_wrapper.FunctionPtr
	_func_PyImport_ImportModule      lib_wrapper.FunctionPtr
	_func_Py_IncRef                  lib_wrapper.FunctionPtr
	_func_Py_DecRef                  lib_wrapper.FunctionPtr
	_func_PyObject_Length            lib_wrapper.FunctionPtr
	_func_PyObject_GetAttrString     lib_wrapper.FunctionPtr
	_func_PyObject_SetAttrString     lib_wrapper.FunctionPtr
	_func_PyObject_CallObject        lib_wrapper.FunctionPtr
	_func_PyObject_CallMethodObjArgs lib_wrapper.FunctionPtr
	_func_PyList_New                 lib_wrapper.FunctionPtr
	_func_PyList_GetItem             lib_wrapper.FunctionPtr
	_func_PyList_SetItem             lib_wrapper.FunctionPtr
	_func_PyList_Append              lib_wrapper.FunctionPtr
	_func_PyDict_New                 lib_wrapper.FunctionPtr
	_func_PyDict_GetItemString       lib_wrapper.FunctionPtr
	_func_PyUnicode_FromString       lib_wrapper.FunctionPtr
	_func_PyUnicode_AsUTF8           lib_wrapper.FunctionPtr
	_func_PyErr_Print                lib_wrapper.FunctionPtr
}

func New_LibPythonWrapper(module *lib_wrapper.LibWrapper) LibPythonWrapper {
	w := &LibPythonWrapperImpl{module: module}
	w._func_Py_Initialize = w.module.GetFunc("Py_Initialize")
	w._func_Py_InitializeEx = w.module.GetFunc("Py_InitializeEx")
	w._func_Py_DecodeLocale = w.module.GetFunc("Py_DecodeLocale")
	w._func_Py_SetPythonHome = w.module.GetFunc("Py_SetPythonHome")
	w._func_Py_NewInterpreter = w.module.GetFunc("Py_NewInterpreter")
	w._func_Py_EndInterpreter = w.module.GetFunc("Py_EndInterpreter")
	w._func_PyEval_AcquireThread = w.module.GetFunc("PyEval_AcquireThread")
	w._func_PyEval_ReleaseThread = w.module.GetFunc("PyEval_ReleaseThread")
	w._func_PyEval_SaveThread = w.module.GetFunc("PyEval_SaveThread")
	w._func_PyThreadState_Swap = w.module.GetFunc("PyThreadState_Swap")
	w._func_PyEval_ReleaseLock = w.module.GetFunc("PyEval_ReleaseLock")
	w._func_PyImport_ImportModule = w.module.GetFunc("PyImport_ImportModule")
	w._func_Py_IncRef = w.module.GetFunc("Py_IncRef")
	w._func_Py_DecRef = w.module.GetFunc("Py_DecRef")
	w._func_PyObject_Length = w.module.GetFunc("PyObject_Length")
	w._func_PyObject_GetAttrString = w.module.GetFunc("PyObject_GetAttrString")
	w._func_PyObject_SetAttrString = w.module.GetFunc("PyObject_SetAttrString")
	w._func_PyObject_CallObject = w.module.GetFunc("PyObject_CallObject")
	w._func_PyObject_CallMethodObjArgs = w.module.GetFunc("PyObject_CallMethodObjArgs")
	w._func_PyList_New = w.module.GetFunc("PyList_New")
	w._func_PyList_GetItem = w.module.GetFunc("PyList_GetItem")
	w._func_PyList_SetItem = w.module.GetFunc("PyList_SetItem")
	w._func_PyList_Append = w.module.GetFunc("PyList_Append")
	w._func_PyDict_New = w.module.GetFunc("PyDict_New")
	w._func_PyDict_GetItemString = w.module.GetFunc("PyDict_GetItemString")
	w._func_PyUnicode_FromString = w.module.GetFunc("PyUnicode_FromString")
	w._func_PyUnicode_AsUTF8 = w.module.GetFunc("PyUnicode_AsUTF8")
	w._func_PyErr_Print = w.module.GetFunc("PyErr_Print")
	return w
}
func (w *LibPythonWrapperImpl) Py_Initialize() {
	C._trampoline_Py_Initialize(unsafe.Pointer(w._func_Py_Initialize))
}
func (w *LibPythonWrapperImpl) Py_InitializeEx(initsigs int) {
	_c_initsigs := C.int(initsigs)
	C._trampoline_Py_InitializeEx(unsafe.Pointer(w._func_Py_InitializeEx), _c_initsigs)
}
func (w *LibPythonWrapperImpl) Py_DecodeLocale(s string) unsafe.Pointer {
	_c_s := C.CString(s)
	defer C.free(unsafe.Pointer(_c_s))
	_g_ret := unsafe.Pointer(C._trampoline_Py_DecodeLocale(unsafe.Pointer(w._func_Py_DecodeLocale), _c_s))
	return _g_ret
}
func (w *LibPythonWrapperImpl) Py_SetPythonHome(l unsafe.Pointer) {
	C._trampoline_Py_SetPythonHome(unsafe.Pointer(w._func_Py_SetPythonHome), l)
}
func (w *LibPythonWrapperImpl) Py_NewInterpreter() *PythonThreadState {
	_g_ret := PythonThreadState_FromPointer(C._trampoline_Py_NewInterpreter(unsafe.Pointer(w._func_Py_NewInterpreter)))
	return _g_ret
}
func (w *LibPythonWrapperImpl) Py_EndInterpreter(o *PythonThreadState) {
	_c_o := (o).GetPointer()
	C._trampoline_Py_EndInterpreter(unsafe.Pointer(w._func_Py_EndInterpreter), _c_o)
}
func (w *LibPythonWrapperImpl) PyEval_AcquireThread(o *PythonThreadState) {
	_c_o := (o).GetPointer()
	C._trampoline_PyEval_AcquireThread(unsafe.Pointer(w._func_PyEval_AcquireThread), _c_o)
}
func (w *LibPythonWrapperImpl) PyEval_ReleaseThread(o *PythonThreadState) {
	_c_o := (o).GetPointer()
	C._trampoline_PyEval_ReleaseThread(unsafe.Pointer(w._func_PyEval_ReleaseThread), _c_o)
}
func (w *LibPythonWrapperImpl) PyEval_SaveThread() *PythonThreadState {
	_g_ret := PythonThreadState_FromPointer(C._trampoline_PyEval_SaveThread(unsafe.Pointer(w._func_PyEval_SaveThread)))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyThreadState_Swap(o *PythonThreadState) *PythonThreadState {
	_c_o := (o).GetPointer()
	_g_ret := PythonThreadState_FromPointer(C._trampoline_PyThreadState_Swap(unsafe.Pointer(w._func_PyThreadState_Swap), _c_o))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyEval_ReleaseLock() {
	C._trampoline_PyEval_ReleaseLock(unsafe.Pointer(w._func_PyEval_ReleaseLock))
}
func (w *LibPythonWrapperImpl) PyImport_ImportModule(name string) *PyObject {
	_c_name := C.CString(name)
	defer C.free(unsafe.Pointer(_c_name))
	_g_ret := PyObject_FromPointer(C._trampoline_PyImport_ImportModule(unsafe.Pointer(w._func_PyImport_ImportModule), _c_name))
	return _g_ret
}
func (w *LibPythonWrapperImpl) Py_IncRef(o *PyObject) {
	_c_o := (o).GetPointer()
	C._trampoline_Py_IncRef(unsafe.Pointer(w._func_Py_IncRef), _c_o)
}
func (w *LibPythonWrapperImpl) Py_DecRef(o *PyObject) {
	_c_o := (o).GetPointer()
	C._trampoline_Py_DecRef(unsafe.Pointer(w._func_Py_DecRef), _c_o)
}
func (w *LibPythonWrapperImpl) PyObject_Length(p *PyObject) int {
	_c_p := (p).GetPointer()
	_g_ret := int(C._trampoline_PyObject_Length(unsafe.Pointer(w._func_PyObject_Length), _c_p))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyObject_GetAttrString(p *PyObject, name string) *PyObject {
	_c_p := (p).GetPointer()
	_c_name := C.CString(name)
	defer C.free(unsafe.Pointer(_c_name))
	_g_ret := PyObject_FromPointer(C._trampoline_PyObject_GetAttrString(unsafe.Pointer(w._func_PyObject_GetAttrString), _c_p, _c_name))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyObject_SetAttrString(p *PyObject, name string, p2 *PyObject) int {
	_c_p := (p).GetPointer()
	_c_name := C.CString(name)
	defer C.free(unsafe.Pointer(_c_name))
	_c_p2 := (p2).GetPointer()
	_g_ret := int(C._trampoline_PyObject_SetAttrString(unsafe.Pointer(w._func_PyObject_SetAttrString), _c_p, _c_name, _c_p2))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyObject_CallObject(p *PyObject, args *PyObject) *PyObject {
	_c_p := (p).GetPointer()
	_c_args := (args).GetPointer()
	_g_ret := PyObject_FromPointer(C._trampoline_PyObject_CallObject(unsafe.Pointer(w._func_PyObject_CallObject), _c_p, _c_args))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyObject_CallMethodObjArgs(p *PyObject, name *PyObject, cargs_vargs []*PyObject) *PyObject {
	_c_p := (p).GetPointer()
	_c_name := (name).GetPointer()
	_c_cargs_vargs_ := cargs_vargs
	var _c_cargs_vargs *unsafe.Pointer
	_c_cargs_vargs__ := make([]unsafe.Pointer, len(_c_cargs_vargs_), len(_c_cargs_vargs_))
	for i, _ := range _c_cargs_vargs_ {
		_c_cargs_vargs__[i] = _c_cargs_vargs_[i].GetPointer()
	}
	if len(_c_cargs_vargs_) != 0 {
		_c_cargs_vargs = &_c_cargs_vargs__[0]
	}
	_c_cargs_vargs_len := len(_c_cargs_vargs_)
	_g_ret := PyObject_FromPointer(C._trampoline_PyObject_CallMethodObjArgs(unsafe.Pointer(w._func_PyObject_CallMethodObjArgs), _c_p, _c_name, C.int(_c_cargs_vargs_len), _c_cargs_vargs))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyList_New(len ssize_t) *PyList {
	_c_len := C.ssize_t(len)
	_g_ret := PyList_FromPointer(C._trampoline_PyList_New(unsafe.Pointer(w._func_PyList_New), _c_len))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyList_GetItem(o *PyList, i ssize_t) *PyObject {
	_c_o := (o).GetPointer()
	_c_i := C.ssize_t(i)
	_g_ret := PyObject_FromPointer(C._trampoline_PyList_GetItem(unsafe.Pointer(w._func_PyList_GetItem), _c_o, _c_i))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyList_SetItem(o *PyList, i ssize_t, e *PyObject) int {
	_c_o := (o).GetPointer()
	_c_i := C.ssize_t(i)
	_c_e := (e).GetPointer()
	_g_ret := int(C._trampoline_PyList_SetItem(unsafe.Pointer(w._func_PyList_SetItem), _c_o, _c_i, _c_e))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyList_Append(o *PyList, e *PyObject) int {
	_c_o := (o).GetPointer()
	_c_e := (e).GetPointer()
	_g_ret := int(C._trampoline_PyList_Append(unsafe.Pointer(w._func_PyList_Append), _c_o, _c_e))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyDict_New() *PyDict {
	_g_ret := PyDict_FromPointer(C._trampoline_PyDict_New(unsafe.Pointer(w._func_PyDict_New)))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyDict_GetItemString(p *PyDict, key string) *PyObject {
	_c_p := (p).GetPointer()
	_c_key := C.CString(key)
	defer C.free(unsafe.Pointer(_c_key))
	_g_ret := PyObject_FromPointer(C._trampoline_PyDict_GetItemString(unsafe.Pointer(w._func_PyDict_GetItemString), _c_p, _c_key))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyUnicode_FromString(pth string) *PyObject {
	_c_pth := C.CString(pth)
	defer C.free(unsafe.Pointer(_c_pth))
	_g_ret := PyObject_FromPointer(C._trampoline_PyUnicode_FromString(unsafe.Pointer(w._func_PyUnicode_FromString), _c_pth))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyUnicode_AsUTF8(r *PyObject) string {
	_c_r := (r).GetPointer()
	_g_ret := C.GoString(C._trampoline_PyUnicode_AsUTF8(unsafe.Pointer(w._func_PyUnicode_AsUTF8), _c_r))
	return _g_ret
}
func (w *LibPythonWrapperImpl) PyErr_Print() {
	C._trampoline_PyErr_Print(unsafe.Pointer(w._func_PyErr_Print))
}
