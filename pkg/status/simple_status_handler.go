package status

import (
	"fmt"
	"golang.org/x/term"
	"os"
	"syscall"
)

type simpleStatusHandler struct {
	cb         func(level Level, message string)
	isTerminal bool
	trace      bool
}

type simpleStatusLine struct {
}

func NewSimpleStatusHandler(cb func(level Level, message string), isTerminal bool, trace bool) StatusHandler {
	return &simpleStatusHandler{
		cb:         cb,
		isTerminal: isTerminal,
		trace:      trace,
	}
}

func (s *simpleStatusHandler) IsTerminal() bool {
	return s.isTerminal
}

func (s *simpleStatusHandler) IsTraceEnabled() bool {
	return s.trace
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
		s.Message(LevelInfo, message)
	}
	return &simpleStatusLine{}
}

func (s *simpleStatusHandler) Message(level Level, message string) {
	s.cb(level, message)
}

func (s *simpleStatusHandler) MessageFallback(level Level, message string) {
	s.Message(level, message)
}

func (s *simpleStatusHandler) Prompt(password bool, message string) (string, error) {
	s.cb(LevelPrompt, message)

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
