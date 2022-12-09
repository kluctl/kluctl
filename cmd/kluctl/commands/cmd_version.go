package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/version"
	"os"
)

type versionCmd struct {
}

func (cmd *versionCmd) Run(ctx context.Context) error {
	_, err := os.Stdout.WriteString(version.GetVersion() + "\n")
	return err
}
