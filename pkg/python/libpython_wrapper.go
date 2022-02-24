package python

import "unsafe"

type ssize_t = int

//go:generate go run .../lib_wrapper/gen --input libpython_wrapper.go --output libpython_wrapper_impl.go
type LibPythonWrapper interface {
	Py_Initialize()
	Py_InitializeEx(initsigs int)

	Py_DecodeLocale(s string) unsafe.Pointer
	Py_SetPythonHome(l unsafe.Pointer)

	Py_NewInterpreter() *PythonThreadState
	Py_EndInterpreter(o *PythonThreadState)

	PyEval_AcquireThread(o *PythonThreadState)
	PyEval_ReleaseThread(o *PythonThreadState)
	PyEval_SaveThread() *PythonThreadState
	PyThreadState_Swap(o *PythonThreadState) *PythonThreadState
	PyEval_ReleaseLock()

	PyImport_ImportModule(name string) *PyObject

	Py_IncRef(o *PyObject)
	Py_DecRef(o *PyObject)
	PyObject_Length(p *PyObject) int
	PyObject_GetAttrString(p *PyObject, name string) *PyObject
	PyObject_SetAttrString(p *PyObject, name string, p2 *PyObject) int
	PyObject_CallObject(p *PyObject, args *PyObject) *PyObject
	PyObject_CallMethodObjArgs(p *PyObject, name *PyObject, cargs_vargs []*PyObject) *PyObject

	PyList_New(len ssize_t) *PyList
	PyList_GetItem(o *PyList, i ssize_t) *PyObject
	PyList_SetItem(o *PyList, i ssize_t, e *PyObject) int
	PyList_Append(o *PyList, e *PyObject) int

	PyDict_New() *PyDict
	PyDict_GetItemString(p *PyDict, key string) *PyObject

	PyUnicode_FromString(pth string) *PyObject
	PyUnicode_AsUTF8(r *PyObject) string

	PyErr_Print()
}
