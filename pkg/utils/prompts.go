package utils

import (
	"fmt"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
	"os"
	"strings"
	"syscall"
)

// AskForConfirmation uses Scanln to parse user input. A user must type in "yes" or "no" and
// then press enter. It has fuzzy matching, so "y", "Y", "yes", "YES", and "Yes" all count as
// confirmations. If the input is not recognized, it will ask again. The function does not return
// until it gets a valid response from the user. Typically, you should use fmt to print out a question
// before calling askForConfirmation. E.g. fmt.Println("WARNING: Are you sure? (yes/no)")
func AskForConfirmation(prompt string) bool {
	if !isatty.IsTerminal(os.Stderr.Fd()) {
		log.Warningf("Not a terminal, suppressed prompt: %s", prompt)
		return false
	}

	_, err := os.Stderr.WriteString(prompt + " (y/N) ")
	if err != nil {
		log.Fatal(err)
	}

	var response string
	_, err = fmt.Scanln(&response)
	if err != nil {
		return false
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if FindStrInSlice(okayResponses, response) != -1 {
		return true
	} else if FindStrInSlice(nokayResponses, response) != -1 || response == "" {
		return false
	} else {
		fmt.Println("Please type yes or no and then press enter:")
		return AskForConfirmation(prompt)
	}
}

func AskForPassword(prompt string) (string, error) {
	if !isatty.IsTerminal(os.Stderr.Fd()) {
		err := fmt.Errorf("not a terminal, suppressed credentials prompt: %s", prompt)
		log.Warning(err)
		return "", err
	}

	_, err := fmt.Fprintf(os.Stderr, "%s: ", prompt)
	if err != nil {
		return "", err
	}

	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	_, _ = fmt.Fprintf(os.Stderr, "\n")
	if err != nil {
		return "", err
	}

	password := string(bytePassword)
	return strings.TrimSpace(password), nil
}
