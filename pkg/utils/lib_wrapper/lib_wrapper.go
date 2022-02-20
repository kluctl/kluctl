package lib_wrapper

/*
#include "call.h"
*/
import "C"
import (
	"log"
	"sync"
	"unsafe"
)

const MaxVariadicLength = 5

type FunctionPtr unsafe.Pointer

type LibWrapper struct {
	module  unsafe.Pointer
	funcMap sync.Map
}

func (lw *LibWrapper) getFunc(funcName string) FunctionPtr {
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

func (lw *LibWrapper) Call_V_PTRS(funcName string, args ...unsafe.Pointer) {
	f := lw.getFunc(funcName)

	if len(args) > MaxVariadicLength {
		panic("Call_V_PTRS: too many arguments")
	}

	var cargs *unsafe.Pointer
	if len(args) == 0 {
		cargs = nil
	} else {
		cargs = &args[0]
	}
	C.call_v_ptrs(unsafe.Pointer(f), C.int(len(args)), cargs)
}

func (lw *LibWrapper) Call_VP_PTRS(funcName string, args ...unsafe.Pointer) unsafe.Pointer {
	f := lw.getFunc(funcName)

	if len(args) > MaxVariadicLength {
		panic("Call_V_PTRS: too many arguments")
	}

	var cargs *unsafe.Pointer
	if len(args) == 0 {
		cargs = nil
	} else {
		cargs = &args[0]
	}
	return C.call_vp_ptrs(unsafe.Pointer(f), C.int(len(args)), cargs)
}

func (lw *LibWrapper) Call_VP_PTRS_VARGS(funcName string, a1 []unsafe.Pointer, vargs ...unsafe.Pointer) unsafe.Pointer {
	f := lw.getFunc(funcName)

	if len(vargs) > MaxVariadicLength {
		panic("Call_V_PTRS: too many arguments")
	}

	var ca1 *unsafe.Pointer
	if len(a1) == 0 {
		ca1 = nil
	} else {
		ca1 = &a1[0]
	}

	var cvargs *unsafe.Pointer
	if len(vargs) == 0 {
		cvargs = nil
	} else {
		cvargs = &vargs[0]
	}
	return C.call_vp_ptrs_vargs(unsafe.Pointer(f), C.int(len(a1)), ca1, C.int(len(vargs)), cvargs)
}

func (lw *LibWrapper) Call_I_PTRS(funcName string, args ...unsafe.Pointer) int {
	f := lw.getFunc(funcName)

	if len(args) > MaxVariadicLength {
		panic("Call_V_PTRS: too many arguments")
	}

	var cargs *unsafe.Pointer
	if len(args) == 0 {
		cargs = nil
	} else {
		cargs = &args[0]
	}
	return int(C.call_i_ptrs(unsafe.Pointer(f), C.int(len(args)), cargs))
}

func (lw *LibWrapper) Call_VP_SS(funcName string, a1 int) unsafe.Pointer {
	f := lw.getFunc(funcName)
	return C.call_vp_ss(unsafe.Pointer(f), C.ssize_t(a1))
}

func (lw *LibWrapper) Call_VP_VP_SS(funcName string, a1 unsafe.Pointer, a2 int) unsafe.Pointer {
	f := lw.getFunc(funcName)
	return C.call_vp_vp_ss(unsafe.Pointer(f), a1, C.ssize_t(a2))
}
