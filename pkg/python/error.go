package python

func PyErr_Print() {
	pythonModule.Call_V_PTRS("PyErr_Print")
}
