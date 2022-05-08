package status

import (
	"context"
	"fmt"
)

// StatusContext is used to report user-facing status/progress
type StatusContext struct {
	ctx      context.Context
	sh       StatusHandler
	sl       StatusLine
	finished bool
	failed   bool
}

type StatusLine interface {
	Update(message string)
	End(success bool)
}

type StatusHandler interface {
	StartStatus(message string) StatusLine

	Info(message string)
	Warning(message string)
	Error(message string)
	Trace(message string)

	PlainText(text string)
}

type contextKey struct{}

func NewContext(ctx context.Context, slh StatusHandler) context.Context {
	return context.WithValue(ctx, contextKey{}, slh)
}

func FromContext(ctx context.Context) StatusHandler {
	v := ctx.Value(contextKey{})
	if v == nil {
		return nil
	}
	return v.(StatusHandler)
}

func Start(ctx context.Context, status string, args ...any) *StatusContext {
	sh := FromContext(ctx)
	s := &StatusContext{
		ctx: ctx,
		sh:  sh,
	}

	s.sl = sh.StartStatus(fmt.Sprintf(status, args...))

	return s
}

func (s *StatusContext) Update(message string, args ...any) {
	s.sl.Update(fmt.Sprintf(message, args...))
}

func (s *StatusContext) Failed() {
	if s.finished {
		return
	}
	s.sl.End(false)
	s.failed = true
}

func (s *StatusContext) FailedWithMessage(msg string, args ...any) {
	if s.finished {
		return
	}
	s.Update(msg, args...)
	s.Failed()
}

func (s *StatusContext) Success() {
	s.sl.End(true)
	s.finished = true
}

func PlainText(ctx context.Context, text string) {
	slh := FromContext(ctx)
	slh.PlainText(text)
}

func Info(ctx context.Context, status string, args ...any) {
	slh := FromContext(ctx)
	slh.Info(fmt.Sprintf(status, args...))
}

func Warning(ctx context.Context, status string, args ...any) {
	slh := FromContext(ctx)
	slh.Warning(fmt.Sprintf(status, args...))
}

func Trace(ctx context.Context, status string, args ...any) {
	slh := FromContext(ctx)
	slh.Trace(fmt.Sprintf(status, args...))
}

func Error(ctx context.Context, status string, args ...any) {
	slh := FromContext(ctx)
	slh.Error(fmt.Sprintf(status, args...))
}
