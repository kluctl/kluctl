package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
)

type archiveCmd struct {
	args.ProjectFlags

	OutputArchive string `group:"misc" help:"Path to .tgz to write project to." type:"path"`
}

func (cmd *archiveCmd) Help() string {
	return `This archive can then be used with '--from-archive'`
}

func (cmd *archiveCmd) Run() error {
	return withKluctlProjectFromArgs(cmd.ProjectFlags, true, false, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
		return p.WriteArchive(cmd.OutputArchive, cmd.ProjectFlags.OutputMetadata == "")
	})
}
