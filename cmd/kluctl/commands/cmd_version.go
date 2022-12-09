package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/version"
)

type versionCmd struct {
}

func (cmd *versionCmd) Run(ctx context.Context) error {
	_, err := getStdout(ctx).WriteString(version.GetVersion() + "\n")
	return err
}
