package utils

import (
	"context"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
	"sync"
	"sync/atomic"
)

type goHelper[T any] struct {
	ctx   context.Context
	sem   *semaphore.Weighted
	wg    sync.WaitGroup
	mutex sync.Mutex

	next atomic.Int32
	res  []goHelperResult[T]
	merr *multierror.Error
}

type goHelperResult[T any] struct {
	finished bool
	result   T
	err      error
}

func NewGoHelperR[T any](ctx context.Context, max int) *goHelper[T] {
	g := &goHelper[T]{
		ctx: ctx,
	}
	if max > 0 {
		g.sem = semaphore.NewWeighted(int64(max))
	}
	return g
}

func NewGoHelper(ctx context.Context, max int) *goHelper[any] {
	return NewGoHelperR[any](ctx, max)
}

func (g *goHelper[T]) addResult(idx int, r T, err error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	for len(g.res) <= idx {
		g.res = append(g.res, goHelperResult[T]{})
	}
	g.res[idx] = goHelperResult[T]{
		finished: true,
		result:   r,
		err:      err,
	}
	if err != nil {
		g.merr = multierror.Append(g.merr, err)
	}
}

func (g *goHelper[T]) RunE(fn func() error) {
	g.RunRE(func() (T, error) {
		err := fn()
		var x T
		return x, err
	})
}

func (g *goHelper[T]) RunRE(fn func() (T, error)) {
	idx := int(g.next.Add(1) - 1)

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if g.sem != nil {
			err := g.sem.Acquire(g.ctx, 1)
			if err != nil {
				var r T
				g.addResult(idx, r, err)
				return
			}
		}
		if g.sem != nil {
			defer g.sem.Release(1)
		}
		r, err := fn()
		g.addResult(idx, r, err)
	}()
}

func (g *goHelper[T]) Run(fn func()) {
	g.RunE(func() error {
		fn()
		return nil
	})
}

func (g *goHelper[T]) Wait() {
	g.wg.Wait()
}

func (g *goHelper[T]) ErrorOrNil() error {
	return g.merr.ErrorOrNil()
}

func (g *goHelper[T]) Results() []T {
	ret := make([]T, len(g.res))
	for i, r := range g.res {
		ret[i] = r.result
	}
	return ret
}

func RunParallelE(ctx context.Context, fs ...func() error) error {
	g := NewGoHelper(ctx, 0)
	for _, f := range fs {
		g.RunE(f)
	}
	g.Wait()
	return g.ErrorOrNil()
}
