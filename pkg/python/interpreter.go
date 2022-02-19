package python


import (
	"log"
	"runtime"
)

type PythonInterpreter struct {
	calls    chan *pythonInterpreterCall
	init     chan error
	stopChan chan bool

	threadState *PyThreadState
}

type pythonInterpreterCall struct {
	fun func() error
	result chan error
}

func NewPythonInterpreter() (*PythonInterpreter, error) {
	p := &PythonInterpreter{
		calls: make(chan *pythonInterpreterCall),
		init: make(chan error),
		stopChan: make(chan bool),
	}

	go p.runThread()

	err := <-p.init
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *PythonInterpreter) Stop() {
	p.stopChan <- true
}

func (p *PythonInterpreter) Run(fun func() error) error {
	c := &pythonInterpreterCall{
		fun: fun,
		result: make(chan error),
	}
	p.calls <- c
	err := <- c.result
	return err
}

func (p *PythonInterpreter) AppendSysPath(pth string) error {
	return p.Run(func() error {
		sys := PyImport_ImportModule("sys")
		defer sys.DecRef()
		l := PyList_FromObject(sys.GetAttrString("path"))
		defer l.DecRef()

		l.Append(PyUnicode_FromString(pth))
		return nil
	})
}

func (p *PythonInterpreter) runThread() {
	runtime.LockOSThread()

	PyEval_AcquireThread(mainThreadState)
	p.threadState = Py_NewInterpreter()
	PyEval_ReleaseThread(p.threadState)

	defer func() {
		PyEval_AcquireThread(p.threadState)
		Py_EndInterpreter(p.threadState)
	}()

	p.init <- nil

	for true {
		select {
		case c := <-p.calls:
			PyEval_AcquireThread(p.threadState)
			p.processCall(c)
			PyEval_ReleaseThread(p.threadState)
		case <-p.stopChan:
			return
		}
	}
}

func (p *PythonInterpreter) processCall(c *pythonInterpreterCall) {
	ss := PyThreadState_Get()
	if ss != p.threadState {
		log.Panicf("ss != p.threadState")
	}

	err := c.fun()
	c.result<-err
}

var mainThreadState *PyThreadState

func init() {
	Py_Initialize()
	mainThreadState = PyEval_SaveThread()
}

