//go:build darwin || linux
// +build darwin linux

package lib_wrapper

/*
#include <stdlib.h>
#include <dlfcn.h>

*/
import "C"
import (
	log "github.com/sirupsen/logrus"
	"unsafe"
)

func LoadModule(pth string) *LibWrapper {
	cPth := NewCString(pth)
	defer cPth.Free()

	mod := C.dlopen((*C.char)(cPth.P), C.RTLD_LAZY)
	if mod == nil {
		log.Panicf("dlopen for %s failed", pth)
	}
	return &LibWrapper{
		module: unsafe.Pointer(mod),
	}
}

func (lw *LibWrapper) loadFunc(funcName string) FunctionPtr {
	cFuncName := NewCString(funcName)
	defer cFuncName.Free()

	f := C.dlsym(lw.module, (*C.char)(cFuncName.P))
	if f == nil {
		log.Panicf("dlsym for %s failed", funcName)
	}
	return FunctionPtr(f)
}
