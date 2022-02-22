//go:build darwin || linux
// +build darwin linux

package lib_wrapper

/*
#cgo LDFLAGS: -ldl
#include <stdlib.h>
#include <dlfcn.h>
*/
import "C"
import (
	log "github.com/sirupsen/logrus"
	"unsafe"
)

func LoadModule(pth string) *LibWrapper {
	cPth := C.CString(pth)
	defer C.free(unsafe.Pointer(cPth))

	mod := C.dlopen(cPth, C.RTLD_LAZY)
	if mod == nil {
		log.Panicf("dlopen for %s failed", pth)
	}
	return &LibWrapper{
		module: unsafe.Pointer(mod),
	}
}

func (lw *LibWrapper) loadFunc(funcName string) FunctionPtr {
	cFuncName := C.CString(funcName)
	defer C.free(unsafe.Pointer(cFuncName))

	f := C.dlsym(lw.module, cFuncName)
	if f == nil {
		log.Panicf("dlsym for %s failed", funcName)
	}
	return FunctionPtr(f)
}
