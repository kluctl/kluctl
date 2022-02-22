package python

type PyList = PyObject

func PyList_New(len int) *PyList {
	p := pythonModule.Call_VP_SS("PyList_New", len)
	return togo(p)
}

func PyList_FromObject(l *PyObject) *PyList {
	return l
}

func (l *PyList) GetItem(pos int) *PyObject {
	return togo(pythonModule.Call_VP_VP_SS("PyList_GetItem", l.p, pos))
}

func (l *PyList) Append(item *PyObject) int {
	return pythonModule.Call_I_PTRS("PyList_Append", l.p, item.p)
}
