package python
import "C"
import (
	"github.com/codablock/kluctl/pkg/utils/lib_wrapper"
)

type PyDict = PyObject

func PyDict_New() *PyDict {
	return togo(pythonModule.Call_VP_PTRS("PyDict_New"))
}

func PyDict_FromObject(o *PyObject) *PyDict {
	return o
}

func (d *PyDict) GetItemString(key string) *PyObject {
	ckey := lib_wrapper.NewCString(key)
	defer ckey.Free()

	return togo(pythonModule.Call_VP_PTRS("PyDict_GetItemString", d.p, ckey.P))
}
