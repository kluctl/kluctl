package python

import "C"
import (
	"github.com/codablock/kluctl/pkg/utils/lib_wrapper"
)

func PyUnicode_FromString(u string) *PyObject {
	cu := lib_wrapper.NewCString(u)
	defer cu.Free()

	return togo(pythonModule.Call_VP_PTRS("PyUnicode_FromString", cu.P))
}

func PyUnicode_AsUTF8(unicode *PyObject) string {
	cutf8 := pythonModule.Call_VP_PTRS("PyUnicode_AsUTF8", unicode.p)
	return C.GoString((*C.char)(cutf8))
}
