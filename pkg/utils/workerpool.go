package utils

import (
	"github.com/gammazero/workerpool"
	"os"
	"strconv"
	"sync"
)

type WorkerPoolWithErrors struct {
	maxWorkers int
	pool       *workerpool.WorkerPool
	errors     []error
	results    []interface{}
	mutex      sync.Mutex
}

func NewWorkerPoolWithErrors(maxWorkers int) *WorkerPoolWithErrors {
	return &WorkerPoolWithErrors{
		maxWorkers: maxWorkers,
		pool:       workerpool.New(maxWorkers),
	}
}

func NewDebuggerAwareWorkerPool(maxWorkers int) *WorkerPoolWithErrors {
	if IsLaunchedByDebugger() {
		ignoreDebuggerStr, _ := os.LookupEnv("KLUCTL_IGNORE_DEBUGGER")
		ignoreDebugger, _ := strconv.ParseBool(ignoreDebuggerStr)
		if !ignoreDebugger {
			maxWorkers = 1
		}
	}
	return NewWorkerPoolWithErrors(maxWorkers)
}

func (wp *WorkerPoolWithErrors) Submit(cb func() error) {
	wp.SubmitWithResult(func() (interface{}, error) {
		err := cb()
		return nil, err
	})
}

func (wp *WorkerPoolWithErrors) SubmitWithResult(cb func() (interface{}, error)) {
	wp.pool.Submit(func() {
		result, err := cb()
		wp.mutex.Lock()
		defer wp.mutex.Unlock()
		if err != nil {
			wp.errors = append(wp.errors, err)
		} else if result != nil {
			wp.results = append(wp.results, result)
		}
	})
}

func (wp *WorkerPoolWithErrors) StopWait(restart bool) error {
	if wp.pool == nil {
		return nil
	}
	wp.pool.StopWait()
	if restart {
		wp.pool = workerpool.New(wp.maxWorkers)
	} else {
		wp.pool = nil
	}

	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	if len(wp.errors) == 0 {
		return nil
	}
	err := NewErrorListOrNil(wp.errors)
	wp.errors = nil
	return err
}

func (wp *WorkerPoolWithErrors) Errors() []error {
	return wp.errors
}

func (wp *WorkerPoolWithErrors) Results() []interface{} {
	return wp.results
}
