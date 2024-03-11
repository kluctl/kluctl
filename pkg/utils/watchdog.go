package utils

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"sync/atomic"
	"time"
)

func RunWithDeadlineAndPanic(ctx context.Context, extraDeadline time.Duration, f func() error) error {
	deadline, hasDeadline := ctx.Deadline()

	if !hasDeadline {
		return f()
	}

	var finished atomic.Bool

	wait := deadline.Sub(time.Now()) + extraDeadline
	if wait < 0 {
		return ctx.Err()
	}

	deadlineErr := errors.New(fmt.Errorf("deadline exceeded while calling function"))

	t := time.AfterFunc(wait, func() {
		if !finished.Load() {
			panic(deadlineErr.ErrorStack())
		}
	})
	defer t.Stop()

	err := f()
	finished.Store(true)

	return err
}
