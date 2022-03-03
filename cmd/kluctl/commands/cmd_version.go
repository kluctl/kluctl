package commands

import (
	"github.com/codablock/kluctl/pkg/version"
	"os"
)

type versionCmd struct {
}

func (cmd *versionCmd) Run() error {
	_, err := os.Stdout.WriteString(version.Version + "\n")
	return err
}
