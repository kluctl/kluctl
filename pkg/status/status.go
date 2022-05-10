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

	prefix        string
	startMessage  string
	startPriority int
	startTotal    int
	disableLogs   bool
}

type EndResult int

const (
	EndSuccess EndResult = iota
	EndWarning
	EndError
)

type StatusLine interface {
	SetTotal(total int)
	Increment()

	Update(message string)
	End(result EndResult)
}

type StatusHandler interface {
	SetTrace(trace bool)
	Stop()
	StartStatus(total int, message string) StatusLine

	Info(message string)
	Warning(message string)
	Error(message string)
	Trace(message string)

	PlainText(text string)
	InfoFallback(message string)
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

type Option func(s *StatusContext)

func WithPrefix(prefix string) Option {
	return func(s *StatusContext) {
		s.prefix = prefix
	}
}

func WithStatus(message string, args ...any) Option {
	return func(s *StatusContext) {
		s.startMessage = fmt.Sprintf(message, args...)
	}
}

func WithPriority(p int) Option {
	return func(s *StatusContext) {
		s.startPriority = p
	}
}

func WithTotal(t int) Option {
	return func(s *StatusContext) {
		s.startTotal = t
	}
}

func WithDisableLogs() Option {
	return func(s *StatusContext) {
		s.disableLogs = true
	}
}

func StartWithOptions(ctx context.Context, opts ...Option) *StatusContext {
	sh := FromContext(ctx)
	s := &StatusContext{
		ctx: ctx,
		sh:  sh,
	}

	for _, o := range opts {
		o(s)
	}

	s.sl = sh.StartStatus(s.startTotal, s.buildMessage(s.startMessage))

	return s
}

func Start(ctx context.Context, status string, args ...any) *StatusContext {
	return StartWithOptions(ctx,
		WithTotal(1),
		WithStatus(status, args...),
	)
}

func (s *StatusContext) buildMessage(message string, args ...any) string {
	if message == "" {
		return ""
	}
	m := fmt.Sprintf(message, args...)
	if s.prefix == "" {
		return m
	}
	return fmt.Sprintf("%s: %s", s.prefix, m)
}

func (s *StatusContext) SetTotal(total int) {
	if s == nil {
		return
	}
	s.sl.SetTotal(total)
}

func (s *StatusContext) Increment() {
	if s == nil {
		return
	}
	s.sl.Increment()
}

func (s *StatusContext) Update(message string, args ...any) {
	if s == nil {
		return
	}
	s.sl.Update(s.buildMessage(message, args...))
}

func (s *StatusContext) InfoFallback(message string, args ...any) {
	if s == nil {
		return
	}
	InfoFallback(s.ctx, s.buildMessage(message, args...))
}

func (s *StatusContext) UpdateAndInfoFallback(message string, args ...any) {
	if s == nil {
		return
	}
	s.Update(message, args...)
	s.InfoFallback(message, args...)
}

func (s *StatusContext) Failed() {
	if s == nil {
		return
	}
	if s.finished {
		return
	}
	s.sl.End(EndError)
	s.failed = true
}

func (s *StatusContext) FailedWithMessage(msg string, args ...any) {
	if s == nil {
		return
	}
	if s.finished {
		return
	}
	s.Update(msg, args...)
	s.Failed()
}

func (s *StatusContext) Success() {
	if s == nil {
		return
	}
	s.sl.End(EndSuccess)
	s.finished = true
}

func (s *StatusContext) Warning() {
	if s == nil {
		return
	}
	s.sl.End(EndWarning)
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

func InfoFallback(ctx context.Context, status string, args ...any) {
	slh := FromContext(ctx)
	slh.InfoFallback(fmt.Sprintf(status, args...))
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
