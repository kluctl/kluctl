package status

import (
	"fmt"
	"io"
)

type MultiLineStatusHandler struct {
	out   io.Writer
	trace bool
}

type statusLine struct {
	slh     *MultiLineStatusHandler
	message string
}

func NewMultiLineStatusHandler(out io.Writer, trace bool) *MultiLineStatusHandler {
	sh := &MultiLineStatusHandler{
		out:   out,
		trace: trace,
	}
	return sh
}

func (s *MultiLineStatusHandler) StartStatus(message string) StatusLine {
	sl := &statusLine{
		slh:     s,
		message: message,
	}
	s.writeOut(sl.message)
	return sl
}

func (s *MultiLineStatusHandler) writeOut(message string) {
	_, _ = fmt.Fprintf(s.out, "%s\n", message)
}

func (s *MultiLineStatusHandler) Info(message string) {
	s.writeOut(message)
}

func (s *MultiLineStatusHandler) Warning(message string) {
	s.writeOut(message)
}

func (s *MultiLineStatusHandler) Error(message string) {
	s.writeOut(message)
}

func (s *MultiLineStatusHandler) Trace(message string) {
	if s.trace {
		s.writeOut(message)
	}
}

func (s *MultiLineStatusHandler) PlainText(text string) {
	s.writeOut(text)
}

func (sl *statusLine) Update(message string) {
	sl.message = message
	sl.slh.writeOut(sl.message)
}

func (sl *statusLine) End(success bool) {
}
