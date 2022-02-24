package lib_wrapper

import (
	"log"
	"sync"
	"unsafe"
)

type FunctionPtr unsafe.Pointer

type LibWrapper struct {
	module  unsafe.Pointer
	funcMap sync.Map
}

func (lw *LibWrapper) GetFunc(funcName string) FunctionPtr {
	f, ok := lw.funcMap.Load(funcName)
	if !ok {
		f = lw.loadFunc(funcName)
		f, _ = lw.funcMap.LoadOrStore(funcName, f)
	}
	f2, ok := f.(FunctionPtr)
	if !ok {
		log.Panicf("f2 != FunctionPtr")
	}
	return f2
}
