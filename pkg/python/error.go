package python

/*
#include "Python.h"
*/
import "C"

func PyErr_Print() {
	C.PyErr_PrintEx(1)
}
