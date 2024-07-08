package prompts

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/lib/status"
	"github.com/kluctl/kluctl/v2/pkg/utils/term"
	"os"
	"strings"
	"syscall"
)

type PromptProvider interface {
	Prompt(ctx context.Context, password bool, message string) (string, error)
}

type StatusAndStdinPromptProvider struct {
}

type SimplePromptProvider struct {
	Out *os.File
}

func (pp *StatusAndStdinPromptProvider) Prompt(ctx context.Context, password bool, message string) (string, error) {
	s := status.StartWithOptions(ctx,
		status.WithLevel(status.LevelPrompt),
		status.WithTotal(1),
		status.WithStatus(message),
	)
	defer s.Failed()

	doUpdate := func(ret []byte) {
		if password {
			s.Update(message + strings.Repeat("*", len(ret)))
		} else {
			s.Update(message + string(ret))
		}
	}

	ret, err := term.ReadLineNoEcho(int(syscall.Stdin), doUpdate)
	if err == nil {
		s.Success()
	}

	return string(ret), err
}

func (pp *SimplePromptProvider) Prompt(ctx context.Context, password bool, message string) (string, error) {
	_, err := fmt.Fprintf(pp.Out, "%s", message)
	if err != nil {
		return "", err
	}

	if password {
		ret, err := term.ReadLineNoEcho(int(syscall.Stdin), nil)
		return string(ret), err
	} else {
		var ret string
		_, err := fmt.Scanln(&ret)
		if err != nil {
			return "", err
		}
		return ret, nil
	}
}

type contextKey struct{}

func NewContext(ctx context.Context, provider PromptProvider) context.Context {
	return context.WithValue(ctx, contextKey{}, provider)
}

func FromContext(ctx context.Context) PromptProvider {
	v := ctx.Value(contextKey{})
	if v == nil {
		return nil
	}
	provider := v.(PromptProvider)
	return provider
}
