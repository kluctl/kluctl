package status

import "fmt"

type NoopStatusHandler struct {
}

type NoopStatusLine struct {
}

func (n NoopStatusHandler) IsTerminal() bool {
	return false
}

func (n NoopStatusHandler) IsTraceEnabled() bool {
	return false
}

func (n NoopStatusHandler) SetTrace(trace bool) {
}

func (n NoopStatusHandler) Stop() {
}

func (n NoopStatusHandler) Flush() {
}

func (n NoopStatusHandler) StartStatus(total int, message string) StatusLine {
	return &NoopStatusLine{}
}

func (n NoopStatusHandler) Message(level Level, message string) {
}

func (n NoopStatusHandler) MessageFallback(level Level, message string) {
}

func (n NoopStatusHandler) Prompt(password bool, message string) (string, error) {
	return "", fmt.Errorf("Prompt not implemented in NoopStatusHandler")
}

var _ StatusHandler = &NoopStatusHandler{}

func (n NoopStatusLine) SetTotal(total int) {
}

func (n NoopStatusLine) Increment() {
}

func (n NoopStatusLine) Update(message string) {
}

func (n NoopStatusLine) End(result EndResult) {
}
