package commands

import (
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
)

type archiveCmd struct {
	args.ProjectFlags

	OutputArchive  string `group:"misc" help:"Path to .tgz to write project to." type:"path"`
	OutputMetadata string `group:"misc" help:"Path to .yml to write metadata to. If not specified, metadata is written into the archive."`
}

func (cmd *archiveCmd) Help() string {
	return `This archive can then be used with '--from-archive'`
}

func (cmd *archiveCmd) Run() error {
	return withKluctlProjectFromArgs(cmd.ProjectFlags, func(p *kluctl_project.KluctlProjectContext) error {
		return p.CreateTGZArchive(cmd.OutputArchive, cmd.OutputMetadata)
	})
}
