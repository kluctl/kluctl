package commands

import (
	"github.com/kluctl/kluctl/v2/pkg/version"
	"os"
)

type versionCmd struct {
}

func (cmd *versionCmd) Run() error {
	_, err := os.Stdout.WriteString(version.GetVersion() + "\n")
	return err
}
