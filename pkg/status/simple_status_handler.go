package status

import (
	"fmt"
	"io"
)

type simpleStatusHandler struct {
	out    io.Writer
	trace  bool
	prefix string
}

type simpleStatusLine struct {
}

func NewSimpleStatusHandler(out io.Writer, trace bool, prefix string) StatusHandler {
	return &simpleStatusHandler{
		out:    out,
		trace:  trace,
		prefix: prefix,
	}
}

func (s *simpleStatusHandler) Stop() {
}

func (s *simpleStatusHandler) StartStatus(total int, message string) StatusLine {
	if message != "" {
		s.Info(message)
	}
	return &simpleStatusLine{}
}

func (s *simpleStatusHandler) Info(message string) {
	if s.prefix == "" {
		fmt.Fprintf(s.out, "%s\n", message)
	} else {
		fmt.Fprintf(s.out, "%s: %s\n", s.prefix, message)
	}
}

func (s *simpleStatusHandler) Warning(message string) {
	s.InfoFallback(message)
}

func (s *simpleStatusHandler) Error(message string) {
	s.InfoFallback(message)
}

func (s *simpleStatusHandler) Trace(message string) {
	if s.trace {
		s.Info(message)
	}
}

func (s *simpleStatusHandler) PlainText(text string) {
	_, _ = io.WriteString(s.out, text)
}

func (s *simpleStatusHandler) InfoFallback(message string) {
	s.Info(message)
}

func (sl *simpleStatusLine) SetTotal(total int) {
}

func (sl *simpleStatusLine) Increment() {
}

func (sl *simpleStatusLine) Update(message string) {
}

func (sl *simpleStatusLine) End(result EndResult) {
}
