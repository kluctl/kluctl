package utils

import (
	"context"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
	"sync"
)

func GoLimited(ctx context.Context, sem *semaphore.Weighted, fn func(), errCb func(err error)) {
	go func() {
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

func GoLimitedMultiError(ctx context.Context, sem *semaphore.Weighted, merr **multierror.Error, mutex *sync.Mutex, fn func() error) {
	errCb := func(err error) {
		mutex.Lock()
		defer mutex.Unlock()
		*merr = multierror.Append(*merr, err)
	}
	GoLimited(ctx, sem, func() {
		err := fn()
		if err != nil {
			errCb(err)
		}
	}, errCb)
}
