package status

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/status/multiline"
	"io"
	"sync"
)

type MultiLineStatusHandler struct {
	ctx         context.Context
	out         io.Writer
	enableColor bool
	trace       bool

	ml *multiline.MultiLinePrinter
}

type statusLine struct {
	slh *MultiLineStatusHandler
	l   *multiline.Line

	current int
	total   int
	message string

	barOverride string

	mutex sync.Mutex
}

func NewMultiLineStatusHandler(ctx context.Context, out io.Writer, enableColor bool, trace bool) *MultiLineStatusHandler {
	sh := &MultiLineStatusHandler{
		ctx:         ctx,
		out:         out,
		enableColor: enableColor,
		trace:       trace,
	}

	sh.start()

	return sh
}

func (s *MultiLineStatusHandler) IsTraceEnabled() bool {
	return s.trace
}

func (s *MultiLineStatusHandler) Flush() {
	s.ml.Flush()
}

func (s *MultiLineStatusHandler) SetTrace(trace bool) {
	s.trace = trace
}

func (s *MultiLineStatusHandler) start() {
	s.ml = &multiline.MultiLinePrinter{}
	s.ml.Start(s.out)
}

func (s *MultiLineStatusHandler) Stop() {
	s.ml.Stop()
}

func (s *MultiLineStatusHandler) StartStatus(level Level, total int, message string) StatusLine {
	return s.startStatus(level, total, message, "")
}

func (s *MultiLineStatusHandler) startStatus(level Level, total int, message string, barOverride string) *statusLine {
	sl := &statusLine{
		slh:         s,
		total:       total,
		message:     message,
		barOverride: barOverride,
	}

	if level != LevelProgress {
		sl.barOverride = s.levelPrefix(level)
	}

	sl.l = sl.slh.ml.NewLine(sl.lineFunc)

	return sl
}

func (s *MultiLineStatusHandler) withColor(c string, txt string) string {
	if !s.enableColor {
		return txt
	}
	switch c {
	case "red":
		c = "\x1b[31m"
	case "green":
		c = "\x1b[32m"
	case "yellow":
		c = "\x1b[33m"
	}
	return fmt.Sprintf("%s%s\x1b[0m", c, txt)
}

func (s *MultiLineStatusHandler) printLine(message string, barOverride string, doFlush bool) {
	s.ml.NewTopLine(func() string {
		return fmt.Sprintf("%s %s", barOverride, message)
	})
	if doFlush {
		s.Flush()
	}
}

func (s *MultiLineStatusHandler) levelPrefix(level Level) string {
	var o string
	switch level {
	case LevelTrace:
		fallthrough
	case LevelInfo:
		o = s.withColor("green", "ⓘ")
	case LevelWarning:
		o = s.withColor("yellow", "⚠")
	case LevelError:
		o = s.withColor("red", "✗")
	case LevelPrompt:
		o = s.withColor("yellow", "?")
	default:
		o = s.withColor("yellow", "¿")
	}
	return o
}

func (s *MultiLineStatusHandler) Message(level Level, message string) {
	if level == LevelTrace && !s.trace {
		return
	}
	o := s.levelPrefix(level)
	s.printLine(message, o, true)
}

func (s *MultiLineStatusHandler) MessageFallback(level Level, message string) {
	// no fallback needed
}

func (sl *statusLine) lineFunc() string {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	if sl.barOverride != "" {
		return fmt.Sprintf("%s %s", sl.barOverride, sl.message)
	}

	s := multiline.Spinner()
	return fmt.Sprintf("%s %s", s, sl.message)
}

func (sl *statusLine) SetTotal(total int) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	sl.total = total
}

func (sl *statusLine) Increment() {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	if sl.current < sl.total {
		sl.current++
	}
}

func (sl *statusLine) Update(message string) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	sl.message = message
}

func (sl *statusLine) end(barOverride string) {
	sl.mutex.Lock()
	sl.barOverride = barOverride
	sl.current = sl.total
	sl.mutex.Unlock()

	sl.l.Remove(true)
}

func (sl *statusLine) End(result EndResult) {
	switch result {
	case EndSuccess:
		sl.end(sl.slh.withColor("green", "✓"))
	case EndWarning:
		sl.end(sl.slh.withColor("yellow", "⚠"))
	case EndError:
		sl.end(sl.slh.withColor("red", "✗"))
	}
}
