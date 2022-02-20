package lib_wrapper

/*
#include <stdlib.h>
*/
import "C"
import "unsafe"

type UnsafeStr struct {
	P unsafe.Pointer
}

func NewCString(s string) *UnsafeStr {
	us := C.CString(s)
	return &UnsafeStr{P: unsafe.Pointer(us)}
}

func (us *UnsafeStr) Free() {
	C.free(us.P)
}
