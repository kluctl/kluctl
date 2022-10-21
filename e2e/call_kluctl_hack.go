package e2e

import (
	"github.com/kluctl/kluctl/v2/cmd/kluctl/commands"
	"os"
)

func isCallKluctlHack() bool {
	return os.Getenv("CALL_KLUCTL") == "true"
}

func init() {
	// We use the Golang's initializing mechanism to run kluctl even though the test executable was invoked
	// This is clearly a hack, but it avoids the requirement to have a kluctl executable pre-built
	if isCallKluctlHack() {
		commands.Execute()
		os.Exit(0)
	}
}
