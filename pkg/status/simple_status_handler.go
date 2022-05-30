package status

import (
	"fmt"
	"golang.org/x/term"
	"os"
	"syscall"
)

type simpleStatusHandler struct {
	cb    func(message string)
	trace bool
}

type simpleStatusLine struct {
}

func NewSimpleStatusHandler(cb func(message string), trace bool) StatusHandler {
	return &simpleStatusHandler{
		cb:    cb,
		trace: trace,
	}
}

func (s *simpleStatusHandler) SetTrace(trace bool) {
	s.trace = trace
}

func (s *simpleStatusHandler) Stop() {
}

func (s *simpleStatusHandler) Flush() {
}

func (s *simpleStatusHandler) StartStatus(total int, message string) StatusLine {
	if message != "" {
		s.Info(message)
	}
	return &simpleStatusLine{}
}

func (s *simpleStatusHandler) Info(message string) {
	s.cb(message)
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
	s.Info(text)
}

func (s *simpleStatusHandler) InfoFallback(message string) {
	s.Info(message)
}

func (s *simpleStatusHandler) Prompt(password bool, message string) (string, error) {
	s.cb(message)

	if password {
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		_, _ = fmt.Fprintf(os.Stderr, "\n")
		if err != nil {
			return "", err
		}
		return string(bytePassword), nil
	} else {
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil {
			return "", err
		}
		return response, nil
	}
}

func (sl *simpleStatusLine) SetTotal(total int) {
}

func (sl *simpleStatusLine) Increment() {
}

func (sl *simpleStatusLine) Update(message string) {
}

func (sl *simpleStatusLine) End(result EndResult) {
}
