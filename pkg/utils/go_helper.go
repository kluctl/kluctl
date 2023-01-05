package utils

import (
	"context"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
	"sync"
)

type goHelper struct {
	ctx   context.Context
	sem   *semaphore.Weighted
	wg    sync.WaitGroup
	mutex sync.Mutex
	errs  *multierror.Error
}

func NewGoHelper(ctx context.Context, max int) *goHelper {
	g := &goHelper{
		ctx: ctx,
	}
	if max > 0 {
		g.sem = semaphore.NewWeighted(int64(max))
	}
	return g
}

func (g *goHelper) addError(err error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.errs = multierror.Append(g.errs, err)
}

func (g *goHelper) RunE(fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if g.sem != nil {
			err := g.sem.Acquire(g.ctx, 1)
			if err != nil {
				g.addError(err)
				return
			}
		}
		if g.sem != nil {
			defer g.sem.Release(1)
		}
		err := fn()
		if err != nil {
			g.addError(err)
		}
	}()
}

func (g *goHelper) Run(fn func()) {
	g.RunE(func() error {
		fn()
		return nil
	})
}

func (g *goHelper) Wait() {
	g.wg.Wait()
}

func (g *goHelper) ErrorOrNil() error {
	return g.errs.ErrorOrNil()
}
