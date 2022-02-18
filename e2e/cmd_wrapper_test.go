package e2e

import (
	"flag"
	"github.com/codablock/kluctl/cmd/kluctl/commands"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
	"testing"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, " ")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var cmdArgs arrayFlags

func init() {
	flag.Var(&cmdArgs, "karg", "")
}

func doRunCmd() error {
	log.Infof("Started wrapper: %s", cmdArgs.String())

	_, err := os.Stdout.WriteString(stdoutStartMarker + "\n")
	if err != nil {
		return err
	}
	_, err = os.Stderr.WriteString(stdoutStartMarker + "\n")
	if err != nil {
		return err
	}
	defer func() {
		_, _ = os.Stdout.WriteString(stdoutEndMarker + "\n")
		_, _ = os.Stderr.WriteString(stdoutEndMarker + "\n")
	}()

	cmd := commands.RootCmd()
	cmd.SetArgs(cmdArgs)
	err = cmd.Execute()
	return err
}

func TestKluctlWrapper(t *testing.T) {
	if len(cmdArgs) == 0 {
		return
	}

	err := doRunCmd()
	if err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}
