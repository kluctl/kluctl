//go:build windows
// +build windows

package lib_wrapper

import "C"
import (
	log "github.com/sirupsen/logrus"
	"syscall"
	"unsafe"
)

func LoadModule(pth string) *LibWrapper {
	mod, err := syscall.LoadLibrary(pth)
	if err != nil {
		log.Panicf("LoadLibrary for %s failed: %w", pth, err)
	}
	return &LibWrapper{
		module: unsafe.Pointer(mod),
	}
}

func (lw *LibWrapper) loadFunc(funcName string) FunctionPtr {
	mod := syscall.Handle(lw.module)
	f, err := syscall.GetProcAddress(mod, funcName)
	if err != nil {
		log.Panicf("GetProcAddress for %s failed", funcName)
	}
	return FunctionPtr(f)
}
