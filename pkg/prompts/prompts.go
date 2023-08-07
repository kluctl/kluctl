package prompts

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"strings"
)

// AskForConfirmation uses Scanln to parse user input. A user must type in "yes" or "no" and
// then press enter. It has fuzzy matching, so "y", "Y", "yes", "YES", and "Yes" all count as
// confirmations. If the input is not recognized, it will ask again. The function does not return
// until it gets a valid response from the user. Typically, you should use fmt to print out a question
// before calling askForConfirmation. E.g. fmt.Println("WARNING: Are you sure? (yes/no)")
func AskForConfirmation(ctx context.Context, prompt string) bool {
	response, err := Prompt(ctx, false, prompt+" (y/N) ")
	if err != nil {
		status.Warningf(ctx, "Failed prompting for \"%s\". Error: %v", prompt, err)
		return false
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if utils.FindStrInSlice(okayResponses, response) != -1 {
		return true
	} else if utils.FindStrInSlice(nokayResponses, response) != -1 || response == "" {
		return false
	} else {
		status.Warning(ctx, "Please type yes or no and then press enter!")
		return AskForConfirmation(ctx, prompt)
	}
}

func AskForPassword(ctx context.Context, prompt string) (string, error) {
	password, err := Prompt(ctx, true, fmt.Sprintf("%s: ", prompt))
	if err != nil {
		return "", fmt.Errorf("failed prompting for \"%s\". Error: %w", prompt, err)
	}

	return strings.TrimSpace(password), nil
}

func AskForCredentials(ctx context.Context, prompt string) (string, string, error) {
	status.Info(ctx, prompt)

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

func Prompt(ctx context.Context, password bool, message string) (string, error) {
	provider := FromContext(ctx)
	if provider == nil {
		return "", fmt.Errorf("no prompt provider specified")
	}
	return provider.Prompt(ctx, password, message)
}
