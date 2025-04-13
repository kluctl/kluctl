package messages

import "fmt"

type MessageCallbacks struct {
	WarningFn            func(s string)
	TraceFn              func(s string)
	AskForPasswordFn     func(prompt string) (string, error)
	AskForConfirmationFn func(prompt string) bool
}

func (l MessageCallbacks) Warning(s string, args ...any) {
	if l.WarningFn != nil {
		l.WarningFn(fmt.Sprintf(s, args...))
	}
}

func (l MessageCallbacks) Trace(s string, args ...any) {
	if l.TraceFn != nil {
		l.TraceFn(fmt.Sprintf(s, args...))
	}
}

func (l MessageCallbacks) AskForPassword(prompt string) (string, error) {
	if l.AskForPasswordFn != nil {
		return l.AskForPasswordFn(prompt)
	}
	err := fmt.Errorf("AskForPasswordFn not provided, skipping prompt: %s", prompt)
	l.Warning("%s", err.Error())
	return "", err
}

func (l MessageCallbacks) AskForConfirmation(prompt string) bool {
	if l.AskForConfirmationFn != nil {
		return l.AskForConfirmationFn(prompt)
	}
	l.Warning("Not a terminal, suppressed prompt: %s", prompt)
	return false
}
