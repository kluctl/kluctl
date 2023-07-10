package status

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/mattn/go-isatty"
	"os"
	"strings"
)

func isTerminal(ctx context.Context) bool {
	sh := FromContext(ctx)
	if sh != nil {
		return sh.IsTerminal()
	} else {
		return isatty.IsTerminal(os.Stderr.Fd())
	}
}

// AskForConfirmation uses Scanln to parse user input. A user must type in "yes" or "no" and
// then press enter. It has fuzzy matching, so "y", "Y", "yes", "YES", and "Yes" all count as
// confirmations. If the input is not recognized, it will ask again. The function does not return
// until it gets a valid response from the user. Typically, you should use fmt to print out a question
// before calling askForConfirmation. E.g. fmt.Println("WARNING: Are you sure? (yes/no)")
func AskForConfirmation(ctx context.Context, prompt string) bool {
	if !isTerminal(ctx) {
		Warningf(ctx, "Not a terminal, suppressed prompt: %s", prompt)
		return false
	}

	response, err := Prompt(ctx, false, prompt+" (y/N) ")
	if err != nil {
		return false
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if utils.FindStrInSlice(okayResponses, response) != -1 {
		return true
	} else if utils.FindStrInSlice(nokayResponses, response) != -1 || response == "" {
		return false
	} else {
		Warning(ctx, "Please type yes or no and then press enter!")
		return AskForConfirmation(ctx, prompt)
	}
}

func AskForPassword(ctx context.Context, prompt string) (string, error) {
	if !isTerminal(ctx) {
		err := fmt.Errorf("not a terminal, suppressed credentials prompt: %s", prompt)
		Warning(ctx, err.Error())
		return "", err
	}

	password, err := Prompt(ctx, true, fmt.Sprintf("%s: ", prompt))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(password), nil
}

func AskForCredentials(ctx context.Context, prompt string) (string, string, error) {
	if !isTerminal(ctx) {
		err := fmt.Errorf("not a terminal, suppressed credentials prompt: %s", prompt)
		Warning(ctx, err.Error())
		return "", "", err
	}

	Info(ctx, prompt)

	username, err := Prompt(ctx, false, "Enter Username: ")
	if err != nil {
		return "", "", err
	}

	password, err := Prompt(ctx, true, "Enter Password: ")
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(username), strings.TrimSpace(password), nil
}
