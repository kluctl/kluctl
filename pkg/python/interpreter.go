package python

import "C"

import (
	"fmt"
	"runtime"
)

type PythonInterpreter struct {
	threadState *PythonThreadState
}

func NewPythonInterpreter() (*PythonInterpreter, error) {
	p := &PythonInterpreter{}

	PythonWrapper.PyEval_AcquireThread(mainThreadState)
	p.threadState = PythonWrapper.Py_NewInterpreter()
	if p.threadState == nil {
		PythonWrapper.PyErr_Print()
		return nil, fmt.Errorf("failed to initialize sub-interpreter")
	}
	PythonWrapper.PyEval_ReleaseThread(p.threadState)

	return p, nil
}

func MainPythonInterpreter() (*PythonInterpreter, error) {
	p := &PythonInterpreter{
		threadState: mainThreadState,
	}
	return p, nil
}

func (p *PythonInterpreter) Stop() {
	if p.threadState != mainThreadState {
		PythonWrapper.PyEval_AcquireThread(p.threadState)
		PythonWrapper.Py_EndInterpreter(p.threadState)
	}
	p.threadState = nil
}

func (p *PythonInterpreter) Lock() {
	runtime.LockOSThread()
	if p.threadState != nil {
		PythonWrapper.PyEval_AcquireThread(p.threadState)
	}
}

func (p *PythonInterpreter) Unlock() {
	if p.threadState != nil {
		PythonWrapper.PyEval_ReleaseThread(p.threadState)
	}
	runtime.UnlockOSThread()
}

func (p *PythonInterpreter) Run(fun func() error) error {
	p.Lock()
	defer p.Unlock()

	return fun()
}

func (p *PythonInterpreter) AppendSysPath(pth string) error {
	return p.Run(func() error {
		sys := PythonWrapper.PyImport_ImportModule("sys")
		defer sys.DecRef()
		l := PyList_FromObject(sys.GetAttrString("path"))
		defer l.DecRef()

		l.Append(PythonWrapper.PyUnicode_FromString(pth))
		return nil
	})
}

var mainThreadState *PythonThreadState
