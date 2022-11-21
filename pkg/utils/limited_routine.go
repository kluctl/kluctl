package utils

import (
	"context"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
	"sync"
)

func GoLimited(ctx context.Context, sem *semaphore.Weighted, wg *sync.WaitGroup, fn func(), errCb func(err error)) {
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		err := sem.Acquire(ctx, 1)
		if err != nil {
			if errCb != nil {
				errCb(err)
			}
			return
		}
		defer sem.Release(1)
		fn()
	}()
}

func GoLimitedMultiError(ctx context.Context, sem *semaphore.Weighted, merr **multierror.Error, mutex *sync.Mutex, wg *sync.WaitGroup, fn func() error) {
	errCb := func(err error) {
		mutex.Lock()
		defer mutex.Unlock()
		*merr = multierror.Append(*merr, err)
	}
	GoLimited(ctx, sem, wg, func() {
		err := fn()
		if err != nil {
			errCb(err)
		}
	}, errCb)
}
