package status

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

// StatusContext is used to report user-facing status/progress
type StatusContext struct {
	ctx      context.Context
	sh       StatusHandler
	sl       StatusLine
	finished bool
	failed   bool

	prefix       string
	startMessage string
	startTotal   int
	disableLogs  bool
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
	IsTerminal() bool
	IsTraceEnabled() bool

	SetTrace(trace bool)
	Stop()
	Flush()
	StartStatus(total int, message string) StatusLine

	Info(message string)
	Warning(message string)
	Error(message string)
	Trace(message string)

	PlainText(text string)
	InfoFallback(message string)

	Prompt(password bool, message string) (string, error)
}

type contextKey struct{}
type contextValue struct {
	slh             StatusHandler
	warningOnce     utils.OnceByKey
	deprecationOnce utils.OnceByKey
}

var noopContextValue = contextValue{
	slh: &NoopStatusHandler{},
}

func NewContext(ctx context.Context, slh StatusHandler) context.Context {
	return context.WithValue(ctx, contextKey{}, &contextValue{
		slh: slh,
	})
}

func getContextValue(ctx context.Context) *contextValue {
	v := ctx.Value(contextKey{})
	if v == nil {
		return &noopContextValue
	}
	cv := v.(*contextValue)
	return cv
}

func FromContext(ctx context.Context) StatusHandler {
	v := getContextValue(ctx)
	return v.slh
}

type Option func(s *StatusContext)

func WithPrefix(prefix string) Option {
	return func(s *StatusContext) {
		s.prefix = prefix
	}
}

func WithStatus(message string) Option {
	return func(s *StatusContext) {
		s.startMessage = message
	}
}

func WithStatusf(message string, args ...any) Option {
	return WithStatusf(fmt.Sprintf(message, args...))
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

func Startf(ctx context.Context, status string, args ...any) *StatusContext {
	return Start(ctx, fmt.Sprintf(status, args...))
}

func Start(ctx context.Context, status string) *StatusContext {
	return StartWithOptions(ctx,
		WithTotal(1),
		WithStatus(status),
	)
}

func (s *StatusContext) buildMessage(message string) string {
	if message == "" {
		return ""
	}
	if s.prefix == "" {
		return message
	}
	return fmt.Sprintf("%s: %s", s.prefix, message)
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

func (s *StatusContext) Update(message string) {
	if s == nil {
		return
	}
	s.sl.Update(s.buildMessage(message))
}

func (s *StatusContext) Updatef(message string, args ...any) {
	s.Update(fmt.Sprintf(message, args...))
}

func (s *StatusContext) InfoFallback(message string) {
	if s == nil {
		return
	}
	InfoFallback(s.ctx, s.buildMessage(message))
}

func (s *StatusContext) InfoFallbackf(message string, args ...any) {
	s.InfoFallback(fmt.Sprintf(message, args...))
}

func (s *StatusContext) UpdateAndInfoFallback(message string) {
	if s == nil {
		return
	}
	s.Update(message)
	s.InfoFallback(message)
}

func (s *StatusContext) UpdateAndInfoFallbackf(message string, args ...any) {
	s.UpdateAndInfoFallback(fmt.Sprintf(message, args...))
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

func (s *StatusContext) FailedWithMessage(msg string) {
	if s == nil {
		return
	}
	if s.finished {
		return
	}
	s.UpdateAndInfoFallback(msg)
	s.Failed()
}

func (s *StatusContext) FailedWithMessagef(msg string, args ...any) {
	s.FailedWithMessage(fmt.Sprintf(msg, args...))
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

func Info(ctx context.Context, status string) {
	slh := FromContext(ctx)
	slh.Info(status)
}

func Infof(ctx context.Context, status string, args ...any) {
	Info(ctx, fmt.Sprintf(status, args...))
}

func InfoFallback(ctx context.Context, status string) {
	slh := FromContext(ctx)
	slh.InfoFallback(status)
}

func InfoFallbackf(ctx context.Context, status string, args ...any) {
	InfoFallback(ctx, fmt.Sprintf(status, args...))
}

func Warning(ctx context.Context, status string) {
	slh := FromContext(ctx)
	slh.Warning(status)
}

func Warningf(ctx context.Context, status string, args ...any) {
	Warning(ctx, fmt.Sprintf(status, args...))
}

func WarningOnce(ctx context.Context, key string, status string) {
	cv := getContextValue(ctx)
	cv.deprecationOnce.Do(key, func() {
		Warning(ctx, status)
	})
}

func WarningOncef(ctx context.Context, key string, status string, args ...any) {
	cv := getContextValue(ctx)
	cv.deprecationOnce.Do(key, func() {
		Warningf(ctx, status, args...)
	})
}

func Trace(ctx context.Context, status string) {
	slh := FromContext(ctx)
	slh.Trace(status)
}

func Tracef(ctx context.Context, status string, args ...any) {
	Trace(ctx, fmt.Sprintf(status, args...))
}

func IsTraceEnabled(ctx context.Context) bool {
	slh := FromContext(ctx)
	return slh.IsTraceEnabled()
}

func Error(ctx context.Context, status string) {
	slh := FromContext(ctx)
	slh.Error(status)
}

func Errorf(ctx context.Context, status string, args ...any) {
	Error(ctx, fmt.Sprintf(status, args...))
}

func Prompt(ctx context.Context, password bool, message string) (string, error) {
	slh := FromContext(ctx)
	return slh.Prompt(password, message)
}

func Promptf(ctx context.Context, password bool, message string, args ...any) (string, error) {
	return Prompt(ctx, password, fmt.Sprintf(message, args...))
}

func Deprecation(ctx context.Context, key string, message string) {
	cv := getContextValue(ctx)
	cv.deprecationOnce.Do(key, func() {
		cv.slh.Warning(message)
	})
}

func Flush(ctx context.Context) {
	slh := FromContext(ctx)
	slh.Flush()
}
